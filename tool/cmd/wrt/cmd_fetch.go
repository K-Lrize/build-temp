package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type FetchOptions struct {
	Artifacts      string
	OpenwrtVersion string
	Target         string
	Subtarget      string
	RepoBase       string
	Line           string
	OutDir         string
}

func fetchCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "拉取上游或自建的 SDK / ImageBuilder 归档，并校验哈希解压",
	}

	cmd.AddCommand(fetchSDKCmd(app))
	cmd.AddCommand(fetchIBCmd(app))
	return cmd
}

func fetchSDKCmd(app *App) *cobra.Command {
	var opts FetchOptions
	cmd := &cobra.Command{
		Use:   "sdk",
		Short: "拉取 SDK 并解压",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFetch(app, "sdk", opts)
		},
	}
	addFetchFlags(cmd, &opts)
	return cmd
}

func fetchIBCmd(app *App) *cobra.Command {
	var opts FetchOptions
	cmd := &cobra.Command{
		Use:   "ib",
		Short: "拉取 ImageBuilder 并解压，同时输出 vermagic 到 GITHUB_OUTPUT",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFetch(app, "ib", opts)
		},
	}
	addFetchFlags(cmd, &opts)
	return cmd
}

func addFetchFlags(cmd *cobra.Command, opts *FetchOptions) {
	cmd.Flags().StringVar(&opts.Artifacts, "artifacts", "", "official | self")
	cmd.Flags().StringVar(&opts.OpenwrtVersion, "openwrt-version", "", "OpenWrt 版本号 (e.g. 23.05.3)")
	cmd.Flags().StringVar(&opts.Target, "target", "", "目标架构")
	cmd.Flags().StringVar(&opts.Subtarget, "subtarget", "", "子目标架构")
	cmd.Flags().StringVar(&opts.RepoBase, "repo-base", "", "R2 仓库基地址 (对于 self)")
	cmd.Flags().StringVar(&opts.Line, "line", "", "Line ID (对于 self)")
	cmd.Flags().StringVar(&opts.OutDir, "out", "", "解压目标目录")
}

func runFetch(app *App, typ string, opts FetchOptions) error {
	if opts.Artifacts == "" || opts.Target == "" || opts.Subtarget == "" || opts.OutDir == "" {
		return errors.New("缺少必需参数 (--artifacts, --target, --subtarget, --out)")
	}
	if opts.Artifacts == "official" && opts.OpenwrtVersion == "" {
		return errors.New("official 模式需指定 --openwrt-version")
	}
	if opts.Artifacts == "self" && (opts.RepoBase == "" || opts.Line == "") {
		return errors.New("self 模式需指定 --repo-base 和 --line")
	}

	if err := os.MkdirAll(opts.OutDir, 0755); err != nil {
		return err
	}

	var baseUrl string
	var archiveName string
	var vermagic string

	client := &http.Client{Timeout: 15 * time.Minute}

	if opts.Artifacts == "official" {
		baseUrl = fmt.Sprintf("https://downloads.openwrt.org/releases/%s/targets/%s/%s", opts.OpenwrtVersion, opts.Target, opts.Subtarget)
		
		// 下载 sha256sums 来推断文件名
		sumsData, err := httpGetBytes(client, baseUrl+"/sha256sums")
		if err != nil {
			return fmt.Errorf("下载 sha256sums 失败: %w", err)
		}
		
		prefix := "openwrt-sdk-"
		if typ == "ib" {
			prefix = "openwrt-imagebuilder-"
		}
		
		archiveName = extractFilenameFromSums(string(sumsData), prefix)
		if archiveName == "" {
			return fmt.Errorf("sha256sums 中找不到 %s 前缀的归档", prefix)
		}
	} else if opts.Artifacts == "self" {
		metaUrl := fmt.Sprintf("%s/%s/targets/%s/%s/current.json", strings.TrimRight(opts.RepoBase, "/"), opts.Line, opts.Target, opts.Subtarget)
		metaData, err := httpGetBytes(client, metaUrl)
		if err != nil {
			return fmt.Errorf("下载 current.json 失败 (%s): %w", metaUrl, err)
		}
		
		var cur struct {
			BuildID             string `json:"build_id"`
			SDKArchive          string `json:"sdk_archive"`
			ImageBuilderArchive string `json:"imagebuilder_archive"`
			Vermagic            string `json:"vermagic"`
		}
		if err := json.Unmarshal(metaData, &cur); err != nil {
			return fmt.Errorf("解析 current.json 失败: %w", err)
		}
		
		if cur.BuildID == "" {
			return errors.New("current.json 中缺少 build_id，请先运行 toolchain 构建")
		}
		
		baseUrl = fmt.Sprintf("%s/%s/targets/%s/%s/builds/%s", strings.TrimRight(opts.RepoBase, "/"), opts.Line, opts.Target, opts.Subtarget, cur.BuildID)
		
		if typ == "sdk" {
			archiveName = cur.SDKArchive
		} else {
			archiveName = cur.ImageBuilderArchive
		}
		if archiveName == "" {
			return fmt.Errorf("current.json 中缺少对应的归档字段记录")
		}
		vermagic = cur.Vermagic
	} else {
		return fmt.Errorf("不支持的 artifacts 模式: %s", opts.Artifacts)
	}

	fmt.Fprintf(app.Stdout, "基地址: %s\n", baseUrl)
	fmt.Fprintf(app.Stdout, "目标文件: %s\n", archiveName)

	// 下载 sha256sums
	sumsData, err := httpGetBytes(client, baseUrl+"/sha256sums")
	if err != nil {
		return fmt.Errorf("下载 sha256sums 失败: %w", err)
	}
	expectedHash := extractHashFromSums(string(sumsData), archiveName)
	if expectedHash == "" {
		return fmt.Errorf("sha256sums 中找不到文件 %s 的哈希记录", archiveName)
	}

	// 下载归档
	archivePath := filepath.Join(opts.OutDir, archiveName)
	fmt.Fprintf(app.Stdout, "正在下载 %s...\n", archiveName)
	if err := downloadFile(client, baseUrl+"/"+archiveName, archivePath); err != nil {
		return fmt.Errorf("下载归档失败: %w", err)
	}
	
	// 校验哈希
	fmt.Fprintf(app.Stdout, "校验哈希 (期望: %s)...\n", expectedHash)
	if err := verifyHash(archivePath, expectedHash); err != nil {
		return fmt.Errorf("哈希校验失败: %w", err)
	}
	
	// 解压
	fmt.Fprintf(app.Stdout, "解压到 %s...\n", opts.OutDir)
	cmd := exec.Command("tar", "-xf", archivePath, "--strip-components=1", "-C", opts.OutDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar 解压失败: %v, 输出: %s", err, string(out))
	}

	// 清理压缩包
	os.Remove(archivePath)

	// 提取或输出 vermagic
	if typ == "ib" {
		if opts.Artifacts == "official" {
			// official：vermagic 抠自官方 IB。
			repoFile := filepath.Join(opts.OutDir, "repositories.conf")
			if _, err := os.Stat(repoFile); os.IsNotExist(err) {
				repoFile = filepath.Join(opts.OutDir, "repositories")
			}
			data, err := os.ReadFile(repoFile)
			if err == nil {
				vm := extractVermagicFromRepos(string(data))
				if vm != "" {
					vermagic = vm
				}
			}
		}
		
		if vermagic == "" {
			return errors.New("无法确定 vermagic")
		}
		
		fmt.Fprintf(app.Stdout, "vermagic: %s\n", vermagic)
		outF := os.Getenv("GITHUB_OUTPUT")
		if outF != "" {
			f, err := os.OpenFile(outF, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				defer f.Close()
				fmt.Fprintf(f, "vermagic=%s\n", vermagic)
			}
		}
	}

	fmt.Fprintln(app.Stdout, "完成！")
	return nil
}

func httpGetBytes(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func downloadFile(client *http.Client, url, dest string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	_, err = io.Copy(out, resp.Body)
	return err
}

func verifyHash(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	
	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expected {
		return fmt.Errorf("哈希不匹配，实际: %s", actual)
	}
	return nil
}

func extractFilenameFromSums(sumsData, prefix string) string {
	scanner := bufio.NewScanner(strings.NewReader(sumsData))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := parts[1]
			if strings.HasPrefix(name, "*") {
				name = name[1:]
			}
			if strings.HasPrefix(name, prefix) && (strings.HasSuffix(name, ".tar.xz") || strings.HasSuffix(name, ".tar.zst") || strings.HasSuffix(name, ".tar.gz")) {
				return name
			}
		}
	}
	return ""
}

func extractHashFromSums(sumsData, filename string) string {
	scanner := bufio.NewScanner(strings.NewReader(sumsData))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := parts[1]
			if strings.HasPrefix(name, "*") {
				name = name[1:]
			}
			if name == filename {
				return parts[0]
			}
		}
	}
	return ""
}

func extractVermagicFromRepos(reposData string) string {
	// 查找 /kmods/<vermagic>/ 结构
	scanner := bufio.NewScanner(strings.NewReader(reposData))
	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.Index(line, "/kmods/"); idx != -1 {
			rem := line[idx+7:]
			if endIdx := strings.Index(rem, "/"); endIdx != -1 {
				return rem[:endIdx]
			}
		}
	}
	return ""
}
