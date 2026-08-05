// Package toolchain 从编译完的 OpenWrt 源码树里收集、拆分并签名产物
// （kmod / base 包、SDK / ImageBuilder 归档），产出可直接发布到 R2 的目录结构。
package toolchain

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/K-Lrize/openwrt-build/internal/artifacts"
	"github.com/K-Lrize/openwrt-build/internal/fsutil"
)

// Result 是 Collect 的产出摘要，供上层写进 meta/current 与 CI 输出。
type Result struct {
	Vermagic      string // 完整 ABI 段，如 6.12.94-1-<hash>
	KernelVersion string // 内核版本，如 6.12.94
	KmodCount     int
	SDKFile       string
	SDKSHA256     string
	IBFile        string
	IBSHA256      string
}

// Collect 抠 vermagic、把 kmod 与 base 包拆进各自目录、收集 SDK/IB 归档并生成
// sha256sums，最后（若能找到 apk）给 kmod/base 索引签名。warnw 收非致命提示。
func Collect(openwrtDir, outDir, target, subtarget string, warnw io.Writer) (Result, error) {
	vermagic, kernelVersion, err := readVermagic(openwrtDir, target, subtarget)
	if err != nil {
		return Result{}, fmt.Errorf("无法获取 vermagic: %w", err)
	}

	kmodOut := filepath.Join(outDir, "kmods", vermagic)
	baseOut := filepath.Join(outDir, "base")
	sdkIbOut := filepath.Join(outDir, "sdk-ib")
	for _, d := range []string{kmodOut, baseOut, sdkIbOut} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return Result{}, err
		}
	}

	tgtDir := filepath.Join(openwrtDir, "bin", "targets", target, subtarget)

	kmodCount, err := collectKmods(tgtDir, kmodOut)
	if err != nil {
		return Result{}, err
	}
	if err := collectBase(filepath.Join(tgtDir, "packages"), baseOut); err != nil {
		return Result{}, err
	}

	res := Result{Vermagic: vermagic, KernelVersion: kernelVersion, KmodCount: kmodCount}
	if err := collectArchives(tgtDir, sdkIbOut, &res); err != nil {
		return Result{}, err
	}

	if err := signIndexes(openwrtDir, warnw, kmodOut, baseOut); err != nil {
		return Result{}, err
	}
	return res, nil
}

// readVermagic 从 build_dir 的内核目录里读 .vermagic，拼成 <version>-1-<vermagic>。
func readVermagic(openwrtDir, target, subtarget string) (segment, kernelVersion string, err error) {
	glob := filepath.Join(openwrtDir, "build_dir", "target-*", "linux-"+target+"_"+subtarget, "linux-*")
	dirs, err := filepath.Glob(glob)
	if err != nil || len(dirs) == 0 {
		return "", "", fmt.Errorf("找不到 linux build 目录 (%s)", glob)
	}
	linuxDir := dirs[0]

	raw, err := os.ReadFile(filepath.Join(linuxDir, ".vermagic"))
	if err != nil {
		return "", "", fmt.Errorf("读取 .vermagic 失败: %w", err)
	}
	kernelVersion = strings.TrimPrefix(filepath.Base(linuxDir), "linux-")
	return fmt.Sprintf("%s-1-%s", kernelVersion, strings.TrimSpace(string(raw))), kernelVersion, nil
}

func collectKmods(tgtDir, kmodOut string) (int, error) {
	count := 0
	err := filepath.Walk(tgtDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		name := filepath.Base(p)
		if !info.IsDir() && strings.HasPrefix(name, "kmod-") && strings.HasSuffix(name, ".apk") {
			if err := fsutil.CopyFile(p, filepath.Join(kmodOut, name)); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("收集 kmods 失败: %w", err)
	}
	return count, nil
}

func collectBase(pkgDir, baseOut string) error {
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil // 没有 packages 目录不算错——某些 target 可能确实没有
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".apk") || strings.HasPrefix(name, "kmod-") {
			continue
		}
		if err := fsutil.CopyFile(filepath.Join(pkgDir, name), filepath.Join(baseOut, name)); err != nil {
			return fmt.Errorf("收集 base 包 %s 失败: %w", name, err)
		}
	}
	return nil
}

// collectArchives 把 SDK / IB 归档复制进 sdkIbOut，算好 sha256 写进 sha256sums，
// 并回填 res 的文件名与哈希。
func collectArchives(tgtDir, sdkIbOut string, res *Result) error {
	entries, err := os.ReadDir(tgtDir)
	if err != nil {
		return fmt.Errorf("读取 target 产物目录失败: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		isSDK := strings.HasPrefix(name, "openwrt-sdk-") && strings.Contains(name, ".tar.")
		isIB := strings.HasPrefix(name, "openwrt-imagebuilder-") && strings.Contains(name, ".tar.")
		if !isSDK && !isIB {
			continue
		}
		dst := filepath.Join(sdkIbOut, name)
		if err := fsutil.CopyFile(filepath.Join(tgtDir, name), dst); err != nil {
			return fmt.Errorf("收集 %s 失败: %w", name, err)
		}
		sha, err := fsutil.SHA256Hex(dst)
		if err != nil {
			return err
		}
		if isSDK {
			res.SDKFile, res.SDKSHA256 = name, sha
		} else {
			res.IBFile, res.IBSHA256 = name, sha
		}
	}

	if res.SDKFile == "" && res.IBFile == "" {
		return nil
	}
	var b strings.Builder
	if res.SDKFile != "" {
		fmt.Fprintf(&b, "%s *%s\n", res.SDKSHA256, res.SDKFile)
	}
	if res.IBFile != "" {
		fmt.Fprintf(&b, "%s *%s\n", res.IBSHA256, res.IBFile)
	}
	return os.WriteFile(filepath.Join(sdkIbOut, "sha256sums"), []byte(b.String()), 0o644)
}

// signIndexes 用源码树里的 host apk 给 kmod/base 目录生成并签名索引。
// apk 签名是必须的——设备运行期凭公钥验证索引，无法签名直接报错。
func signIndexes(openwrtDir string, warnw io.Writer, dirs ...string) error {
	apkBin, _ := filepath.Abs(filepath.Join(openwrtDir, "staging_dir", "host", "bin", "apk"))
	if _, err := os.Stat(apkBin); err != nil {
		return fmt.Errorf("找不到 host apk 工具 %s，make world 可能未完整编出 host 工具链: %w", apkBin, err)
	}
	root, _ := filepath.Abs(openwrtDir)
	key, _ := filepath.Abs(filepath.Join(openwrtDir, "private-key.pem"))
	for _, dir := range dirs {
		if err := regenIndex(apkBin, key, root, dir); err != nil {
			return fmt.Errorf("%s 索引生成失败: %w", filepath.Base(dir), err)
		}
	}
	return nil
}

func regenIndex(apkBin, key, root, dir string) error {
	apks, _ := filepath.Glob(filepath.Join(dir, "*.apk"))
	if len(apks) == 0 {
		return nil
	}
	mkndx := exec.Command(apkBin, "mkndx", "--root", root, "--keys-dir", root,
		"--allow-untrusted", "--sign", key, "--output", artifacts.IndexFile)
	for _, apk := range apks {
		mkndx.Args = append(mkndx.Args, filepath.Base(apk))
	}
	mkndx.Dir = dir
	var stderr bytes.Buffer
	mkndx.Stderr = &stderr
	if err := mkndx.Run(); err != nil {
		return fmt.Errorf("apk mkndx: %v, stderr: %s", err, stderr.String())
	}

	verify := exec.Command(apkBin, "verify", "-v", "--keys-dir", root, artifacts.IndexFile)
	verify.Dir = dir
	verify.Stderr = &stderr
	if err := verify.Run(); err != nil {
		return fmt.Errorf("apk verify: %v, stderr: %s", err, stderr.String())
	}
	return nil
}
