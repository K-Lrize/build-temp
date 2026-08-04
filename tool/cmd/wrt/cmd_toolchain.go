package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func toolchainCmd(app *App) *cobra.Command {
	var openwrtDir, outDir, target, subtarget string

	cmd := &cobra.Command{
		Use:   "toolchain",
		Short: "工具链相关操作（抠 vermagic、签名索引、拆分 kmod 等）",
	}

	collectCmd := &cobra.Command{
		Use:   "collect",
		Short: "从编译完的 openwrt 树中收集、拆分包并签名",
		RunE: func(cmd *cobra.Command, args []string) error {
			if openwrtDir == "" || outDir == "" || target == "" || subtarget == "" {
				return errors.New("缺少必需参数")
			}
			return runToolchainCollect(app, openwrtDir, outDir, target, subtarget)
		},
	}
	collectCmd.Flags().StringVar(&openwrtDir, "openwrt-dir", "", "openwrt 源码目录")
	collectCmd.Flags().StringVar(&outDir, "out-dir", "", "输出目录")
	collectCmd.Flags().StringVar(&target, "target", "", "target")
	collectCmd.Flags().StringVar(&subtarget, "subtarget", "", "subtarget")

	cmd.AddCommand(collectCmd)
	return cmd
}

func getVermagic(openwrtDir, outDir, target, subtarget string) (string, error) {
	// 兜底逻辑：读取 .vermagic 和内核目录名
	buildDir := filepath.Join(openwrtDir, "build_dir")
	targetDirs, err := filepath.Glob(filepath.Join(buildDir, "target-*", "linux-"+target+"_"+subtarget, "linux-*"))
	if err != nil || len(targetDirs) == 0 {
		return "", errors.New("找不到 linux build 目录")
	}
	linuxDir := targetDirs[0]
	vmFile := filepath.Join(linuxDir, ".vermagic")
	
	vmBytes, err := os.ReadFile(vmFile)
	if err != nil {
		return "", fmt.Errorf("读取 .vermagic 失败: %w", err)
	}
	
	// 提取 linux-6.1.94 中的 6.1.94
	linuxName := filepath.Base(linuxDir)
	version := strings.TrimPrefix(linuxName, "linux-")
	
	vmSeg := fmt.Sprintf("%s-1-%s", version, strings.TrimSpace(string(vmBytes)))
	return vmSeg, nil
}

func runToolchainCollect(app *App, openwrtDir, outDir, target, subtarget string) error {
	vmSeg, err := getVermagic(openwrtDir, outDir, target, subtarget)
	if err != nil {
		return fmt.Errorf("无法获取 vermagic: %w", err)
	}
	fmt.Fprintf(app.Stdout, "vermagic: %s\n", vmSeg)

	kmodOut := filepath.Join(outDir, "kmods", vmSeg)
	baseOut := filepath.Join(outDir, "base")

	if err := os.MkdirAll(kmodOut, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(baseOut, 0755); err != nil {
		return err
	}

	tgtDir := filepath.Join(openwrtDir, "bin", "targets", target, subtarget)
	pkgDir := filepath.Join(tgtDir, "packages")

	// 收集 kmods
	err = filepath.Walk(tgtDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasPrefix(filepath.Base(path), "kmod-") && strings.HasSuffix(path, ".apk") {
			dest := filepath.Join(kmodOut, filepath.Base(path))
			return copyFile(path, dest)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("收集 kmods 失败: %w", err)
	}

	// 收集 base
	entries, err := os.ReadDir(pkgDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".apk") && !strings.HasPrefix(e.Name(), "kmod-") {
				dest := filepath.Join(baseOut, e.Name())
				if err := copyFile(filepath.Join(pkgDir, e.Name()), dest); err != nil {
					return fmt.Errorf("收集 base 包 %s 失败: %w", e.Name(), err)
				}
			}
		}
	}

	// 收集 SDK 和 IB
	sdkIbOut := filepath.Join(outDir, "sdk-ib")
	if err := os.MkdirAll(sdkIbOut, 0755); err != nil {
		return err
	}
	var sdkFile, ibFile string
	entries, err = os.ReadDir(tgtDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			isSDK := strings.HasPrefix(name, "openwrt-sdk-") && strings.Contains(name, ".tar.")
			isIB := strings.HasPrefix(name, "openwrt-imagebuilder-") && strings.Contains(name, ".tar.")
			
			if isSDK || isIB {
				dest := filepath.Join(sdkIbOut, name)
				if err := copyFile(filepath.Join(tgtDir, name), dest); err != nil {
					return fmt.Errorf("收集 %s 失败: %w", name, err)
				}
				if isSDK {
					sdkFile = name
				} else {
					ibFile = name
				}
			}
		}
	}
	
	// 生成 sha256sums
	var sdkSha, ibSha string
	if sdkFile != "" || ibFile != "" {
		sumFile, err := os.Create(filepath.Join(sdkIbOut, "sha256sums"))
		if err == nil {
			if sdkFile != "" {
				if h, err := calcSha256(filepath.Join(sdkIbOut, sdkFile)); err == nil {
					sdkSha = h
					fmt.Fprintf(sumFile, "%s *%s\n", h, sdkFile)
				}
			}
			if ibFile != "" {
				if h, err := calcSha256(filepath.Join(sdkIbOut, ibFile)); err == nil {
					ibSha = h
					fmt.Fprintf(sumFile, "%s *%s\n", h, ibFile)
				}
			}
			sumFile.Close()
		}
	}

	// 签名索引
	apkBin := filepath.Join(openwrtDir, "staging_dir", "host", "bin", "apk")
	apkKey := filepath.Join(openwrtDir, "private-key.pem")

	if _, err := os.Stat(apkBin); err == nil {
		if err := regenIndex(apkBin, apkKey, openwrtDir, kmodOut); err != nil {
			return fmt.Errorf("kmod 索引生成失败: %w", err)
		}
		if err := regenIndex(apkBin, apkKey, openwrtDir, baseOut); err != nil {
			return fmt.Errorf("base 索引生成失败: %w", err)
		}
	} else {
		fmt.Fprintf(app.Stderr, "警告: 找不到 apk %s，跳过索引生成\n", apkBin)
	}
	
	// 输出便于 CI 读取的变量 (兼容现代 GitHub Actions)
	outF := os.Getenv("GITHUB_OUTPUT")
	if outF != "" {
		f, err := os.OpenFile(outF, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			fmt.Fprintf(f, "vm_seg=%s\n", vmSeg)
			
			// 统计 kmod 数量
			kmodFiles, _ := filepath.Glob(filepath.Join(kmodOut, "kmod-*.apk"))
			fmt.Fprintf(f, "kmod_count=%d\n", len(kmodFiles))
			if sdkFile != "" {
				fmt.Fprintf(f, "sdk_file=%s\nsdk_sha256=%s\n", sdkFile, sdkSha)
			}
			if ibFile != "" {
				fmt.Fprintf(f, "ib_file=%s\nib_sha256=%s\n", ibFile, ibSha)
			}
		}
	} else {
		// 回退逻辑，方便本地调试查看
		fmt.Fprintf(app.Stdout, "vm_seg=%s\n", vmSeg)
		kmodFiles, _ := filepath.Glob(filepath.Join(kmodOut, "kmod-*.apk"))
		fmt.Fprintf(app.Stdout, "kmod_count=%d\n", len(kmodFiles))
		fmt.Fprintf(app.Stdout, "sdk_file=%s\nsdk_sha256=%s\nib_file=%s\nib_sha256=%s\n", sdkFile, sdkSha, ibFile, ibSha)
	}
	
	return nil
}

func calcSha256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func regenIndex(apkBin, apkKey, root, dir string) error {
	apks, _ := filepath.Glob(filepath.Join(dir, "*.apk"))
	if len(apks) == 0 {
		return nil
	}
	
	cmd := exec.Command(apkBin, "mkndx", "--root", root, "--keys-dir", root, "--allow-untrusted", "--sign", apkKey, "--output", "packages.adb")
	for _, apk := range apks {
		cmd.Args = append(cmd.Args, filepath.Base(apk))
	}
	cmd.Dir = dir
	
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apk mkndx error: %v, stderr: %s", err, stderr.String())
	}
	
	verify := exec.Command(apkBin, "verify", "-v", "--keys-dir", root, "packages.adb")
	verify.Dir = dir
	verify.Stderr = &stderr
	if err := verify.Run(); err != nil {
		return fmt.Errorf("apk verify error: %v, stderr: %s", err, stderr.String())
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
