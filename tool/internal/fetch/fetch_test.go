package fetch

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/K-Lrize/openwrt-build/internal/artifacts"
	"github.com/K-Lrize/openwrt-build/internal/config"
)

const sampleSums = `abc123  openwrt-imagebuilder-25.12.5-mediatek-filogic.Linux-x86_64.tar.zst
def456 *openwrt-sdk-25.12.5-mediatek-filogic_gcc.Linux-x86_64.tar.zst
000000  sha256sums
111111  openwrt-mediatek-filogic-glinet.img.gz
`

func TestFilenameFromSums(t *testing.T) {
	if got := FilenameFromSums(sampleSums, "openwrt-sdk-"); got != "openwrt-sdk-25.12.5-mediatek-filogic_gcc.Linux-x86_64.tar.zst" {
		t.Errorf("SDK 文件名解析错: %q", got)
	}
	if got := FilenameFromSums(sampleSums, "openwrt-imagebuilder-"); got != "openwrt-imagebuilder-25.12.5-mediatek-filogic.Linux-x86_64.tar.zst" {
		t.Errorf("IB 文件名解析错: %q", got)
	}
	if got := FilenameFromSums(sampleSums, "openwrt-nonesuch-"); got != "" {
		t.Errorf("找不到时应返回空，得到 %q", got)
	}
}

func TestHashFromSums(t *testing.T) {
	// 带 '*'（二进制模式）与不带 '*' 都要认。
	if got := HashFromSums(sampleSums, "openwrt-sdk-25.12.5-mediatek-filogic_gcc.Linux-x86_64.tar.zst"); got != "def456" {
		t.Errorf("带星号文件名的哈希解析错: %q", got)
	}
	if got := HashFromSums(sampleSums, "openwrt-imagebuilder-25.12.5-mediatek-filogic.Linux-x86_64.tar.zst"); got != "abc123" {
		t.Errorf("普通文件名的哈希解析错: %q", got)
	}
	if got := HashFromSums(sampleSums, "not-there"); got != "" {
		t.Errorf("找不到时应返回空，得到 %q", got)
	}
}

func TestVermagicFromRepos(t *testing.T) {
	repos := "src c... https://downloads.openwrt.org/releases/25.12.5/targets/mediatek/filogic/kmods/6.12.94-1-abc/\n"
	if got := VermagicFromRepos(repos); got != "6.12.94-1-abc" {
		t.Errorf("vermagic 解析错: %q", got)
	}
	if got := VermagicFromRepos("没有 kmods 段的内容\n"); got != "" {
		t.Errorf("没有 /kmods/ 时应返回空，得到 %q", got)
	}
}

// fakeGetter 按 URL 返回预置内容，URL 未登记则报错——顺便验证 Locate 拼出的
// 是我们期望的那个 URL。
type fakeGetter map[string][]byte

func (f fakeGetter) GetBytes(url string) ([]byte, error) {
	if b, ok := f[url]; ok {
		return b, nil
	}
	return nil, fmt.Errorf("未登记的 URL: %s", url)
}

func TestLocateOfficialOnlyDownloadsSumsOnce(t *testing.T) {
	base := artifacts.DefaultUpstreamRoot + "/releases/25.12.5/targets/mediatek/filogic"
	calls := 0
	g := countingGetter{fakeGetter{base + "/sha256sums": []byte(sampleSums)}, &calls}

	loc, err := Locate(g, IB, Source{
		Artifacts: config.ArtifactsOfficial, Version: "25.12.5",
		Target: "mediatek", Subtarget: "filogic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("official 应只下载一次 sha256sums，实际 %d 次", calls)
	}
	wantURL := base + "/openwrt-imagebuilder-25.12.5-mediatek-filogic.Linux-x86_64.tar.zst"
	if loc.ArchiveURL != wantURL {
		t.Errorf("归档 URL 不符\n want %s\n got  %s", wantURL, loc.ArchiveURL)
	}
	if loc.ExpectedHash != "abc123" {
		t.Errorf("期望哈希 = %q", loc.ExpectedHash)
	}
}

func TestLocateSelfUsesArtifactsPaths(t *testing.T) {
	const repoBase = "https://repo.example.com"
	curURL := repoBase + "/" + artifacts.CurrentPath("25.12-mtk", "mediatek", "filogic")
	// SDK/IB 现平铺在 TargetDir，不再按 builds/<build_id> 归档。
	targetBase := repoBase + "/" + artifacts.TargetDir("25.12-mtk", "mediatek", "filogic")

	cur, _ := json.Marshal(artifacts.Current{
		BuildID:             "b1", // 纯溯源字段，不再用于路径
		Vermagic:            "6.12.94-1-xyz",
		SDKArchive:          "openwrt-sdk-self.tar.zst",
		ImageBuilderArchive: "openwrt-imagebuilder-self.tar.zst",
	})
	g := fakeGetter{
		curURL:                      cur,
		targetBase + "/sha256sums":  []byte("hhh  openwrt-imagebuilder-self.tar.zst\n"),
	}

	loc, err := Locate(g, IB, Source{
		Artifacts: config.ArtifactsSelf, RepoBase: repoBase, Line: "25.12-mtk",
		Target: "mediatek", Subtarget: "filogic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if loc.ArchiveURL != targetBase+"/openwrt-imagebuilder-self.tar.zst" {
		t.Errorf("归档 URL 不符: %s", loc.ArchiveURL)
	}
	if loc.ExpectedHash != "hhh" {
		t.Errorf("期望哈希 = %q", loc.ExpectedHash)
	}
	if loc.Vermagic != "6.12.94-1-xyz" {
		t.Errorf("self 线 vermagic 应直接来自 current.json，得到 %q", loc.Vermagic)
	}
}

type countingGetter struct {
	inner Getter
	calls *int
}

func (c countingGetter) GetBytes(url string) ([]byte, error) {
	*c.calls++
	return c.inner.GetBytes(url)
}
