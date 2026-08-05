// Package fetch 定位并取回上游或自建的 SDK / ImageBuilder 归档。
//
// 所有 R2 / 官方路径都来自 internal/artifacts；本包只负责「从哪个站点、按什么
// 规则找到归档，并校验哈希后解压」。URL 定位（Locate）与纯解析函数都可单测，
// 只有真正的下载/解压落在带 IO 的 DownloadVerifyExtract 里。
package fetch

import (
	"bufio"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/K-Lrize/openwrt-build/internal/artifacts"
	"github.com/K-Lrize/openwrt-build/internal/config"
)

// Kind 区分要取的是 SDK 还是 ImageBuilder。
type Kind int

const (
	SDK Kind = iota
	IB
)

// archivePrefix 是该类归档在 sha256sums / 目录里的文件名前缀。
func (k Kind) archivePrefix() string {
	if k == IB {
		return "openwrt-imagebuilder-"
	}
	return "openwrt-sdk-"
}

func (k Kind) String() string {
	if k == IB {
		return "ib"
	}
	return "sdk"
}

// Source 描述一次取回的来源坐标。
type Source struct {
	Artifacts config.Artifacts
	Version   string // official：OpenWrt 版本号
	Target    string
	Subtarget string
	RepoBase  string // self：R2 公网根
	Line      string // self：line id
	// UpstreamRoot 覆盖官方发布站，空则用官方（主要给测试替身用）。
	UpstreamRoot string
}

// Getter 抽象一次 HTTP GET，便于把 Locate 与真实网络解耦做单测。
type Getter interface {
	GetBytes(url string) ([]byte, error)
}

// Located 是一次定位的结果：archive 的确切下载地址、期望哈希，以及（IB self
// 情况下）顺带取到的 vermagic。
type Located struct {
	ArchiveURL   string
	ArchiveName  string
	ExpectedHash string
	// Vermagic 仅在 self 线能从 current.json 直接取到；official 线要等解压后
	// 从 IB 的 repositories 文件里抠，见 VermagicFromRepos。
	Vermagic string
}

// Locate 定位归档：official 从官方 sha256sums 推断文件名与哈希（只下载一次
// sha256sums）；self 读 R2 的 current.json 拿到文件名/哈希所在目录再取哈希。
func Locate(g Getter, kind Kind, src Source) (Located, error) {
	switch src.Artifacts {
	case config.ArtifactsOfficial:
		return locateOfficial(g, kind, src)
	case config.ArtifactsSelf:
		return locateSelf(g, kind, src)
	default:
		return Located{}, fmt.Errorf("不支持的 artifacts 模式: %q", src.Artifacts)
	}
}

func locateOfficial(g Getter, kind Kind, src Source) (Located, error) {
	if src.Version == "" {
		return Located{}, fmt.Errorf("official 模式需指定 version")
	}
	upstream := artifacts.NewSite(cmp.Or(src.UpstreamRoot, artifacts.DefaultUpstreamRoot))
	baseURL := upstream.URL(artifacts.UpstreamTargetDir(src.Version, src.Target, src.Subtarget))

	sums, err := g.GetBytes(baseURL + "/sha256sums")
	if err != nil {
		return Located{}, fmt.Errorf("下载 sha256sums 失败: %w", err)
	}
	name := FilenameFromSums(string(sums), kind.archivePrefix())
	if name == "" {
		return Located{}, fmt.Errorf("sha256sums 中找不到 %s 前缀的归档", kind.archivePrefix())
	}
	hash := HashFromSums(string(sums), name)
	if hash == "" {
		return Located{}, fmt.Errorf("sha256sums 中找不到文件 %s 的哈希记录", name)
	}
	return Located{ArchiveURL: baseURL + "/" + name, ArchiveName: name, ExpectedHash: hash}, nil
}

func locateSelf(g Getter, kind Kind, src Source) (Located, error) {
	if src.RepoBase == "" || src.Line == "" {
		return Located{}, fmt.Errorf("self 模式需指定 repo-base 和 line")
	}
	r2 := artifacts.NewSite(src.RepoBase)

	curURL := r2.URL(artifacts.CurrentPath(src.Line, src.Target, src.Subtarget))
	raw, err := g.GetBytes(curURL)
	if err != nil {
		return Located{}, fmt.Errorf("下载 current.json 失败 (%s): %w", curURL, err)
	}
	var cur artifacts.Current
	if err := json.Unmarshal(raw, &cur); err != nil {
		return Located{}, fmt.Errorf("解析 current.json 失败: %w", err)
	}
	// build_id 仅为源源字段，不再承载路径；空字符串时只警告，不硬报错。

	name := cur.SDKArchive
	if kind == IB {
		name = cur.ImageBuilderArchive
	}
	if name == "" {
		return Located{}, fmt.Errorf("current.json 中缺少 %s 归档字段记录", kind)
	}

	// SDK/IB/sha256sums 直接平铺在 TargetDir，不再按 builds/<build_id> 归档。
	baseURL := r2.URL(artifacts.TargetDir(src.Line, src.Target, src.Subtarget))
	sums, err := g.GetBytes(baseURL + "/sha256sums")
	if err != nil {
		return Located{}, fmt.Errorf("下载 sha256sums 失败: %w", err)
	}
	hash := HashFromSums(string(sums), name)
	if hash == "" {
		return Located{}, fmt.Errorf("sha256sums 中找不到文件 %s 的哈希记录", name)
	}
	return Located{ArchiveURL: baseURL + "/" + name, ArchiveName: name, ExpectedHash: hash, Vermagic: cur.Vermagic}, nil
}

// DownloadVerifyExtract 把归档下载到 outDir、校验 sha256、解压（strip 一层）后
// 删掉压缩包。校验失败时不会留下解压产物。
func DownloadVerifyExtract(client *http.Client, loc Located, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	archivePath := filepath.Join(outDir, loc.ArchiveName)

	if err := downloadTo(client, loc.ArchiveURL, archivePath); err != nil {
		return fmt.Errorf("下载归档失败: %w", err)
	}
	defer os.Remove(archivePath)

	if err := verifyHash(archivePath, loc.ExpectedHash); err != nil {
		return fmt.Errorf("哈希校验失败: %w", err)
	}

	cmd := exec.Command("tar", "-xf", archivePath, "--strip-components=1", "-C", outDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar 解压失败: %v, 输出: %s", err, string(out))
	}
	return nil
}

// VermagicFromExtractedIB 从解压后的官方 IB 目录里抠 vermagic：读它的
// repositories(.conf) 文件，找 /kmods/<vermagic>/ 结构。self 线不需要它——
// vermagic 已经在 Locate 时从 current.json 拿到了。
func VermagicFromExtractedIB(ibDir string) string {
	for _, name := range []string{"repositories.conf", "repositories"} {
		data, err := os.ReadFile(filepath.Join(ibDir, name))
		if err != nil {
			continue
		}
		if vm := VermagicFromRepos(string(data)); vm != "" {
			return vm
		}
	}
	return ""
}

// ── 纯解析函数（全部可单测）──

// FilenameFromSums 从 sha256sums 里挑出指定前缀、且是 tar 归档的那一行文件名。
func FilenameFromSums(sums, prefix string) string {
	for _, name := range sumsNames(sums) {
		if strings.HasPrefix(name, prefix) &&
			(strings.HasSuffix(name, ".tar.xz") || strings.HasSuffix(name, ".tar.zst") || strings.HasSuffix(name, ".tar.gz")) {
			return name
		}
	}
	return ""
}

// HashFromSums 取指定文件名在 sha256sums 里的哈希。
func HashFromSums(sums, filename string) string {
	scanner := bufio.NewScanner(strings.NewReader(sums))
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) >= 2 && trimStar(parts[1]) == filename {
			return parts[0]
		}
	}
	return ""
}

// VermagicFromRepos 从 apk repositories 文件内容里抠 /kmods/<vermagic>/ 段。
func VermagicFromRepos(reposData string) string {
	scanner := bufio.NewScanner(strings.NewReader(reposData))
	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.Index(line, "/kmods/"); idx != -1 {
			rem := line[idx+len("/kmods/"):]
			if end := strings.IndexByte(rem, '/'); end != -1 {
				return rem[:end]
			}
		}
	}
	return ""
}

func sumsNames(sums string) []string {
	var names []string
	scanner := bufio.NewScanner(strings.NewReader(sums))
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) >= 2 {
			names = append(names, trimStar(parts[1]))
		}
	}
	return names
}

// trimStar 去掉 coreutils sha256sum 二进制模式在文件名前打的 '*'。
func trimStar(name string) string { return strings.TrimPrefix(name, "*") }

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
	if actual := hex.EncodeToString(h.Sum(nil)); actual != expected {
		return fmt.Errorf("期望 %s，实际 %s", expected, actual)
	}
	return nil
}

func downloadTo(client *http.Client, url, dest string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	return out.Close()
}

// HTTPGetter 是把 *http.Client 适配成 Getter 的生产实现。
type HTTPGetter struct{ Client *http.Client }

func (h HTTPGetter) GetBytes(url string) ([]byte, error) {
	resp, err := h.Client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
