// Package repos 装配三层 apk 软件源地址。
package repos

import (
	"cmp"
	"errors"
	"strings"

	"github.com/K-Lrize/openwrt-build/internal/config"
	"github.com/K-Lrize/openwrt-build/internal/resolve"
)

// DefaultUpstreamRoot 是 OpenWrt 官方发布站。
const DefaultUpstreamRoot = "https://downloads.openwrt.org"

// communityFeeds 是 L3 借用的官方社区 feed，按 arch 键控。
var communityFeeds = []string{"base", "luci", "packages", "routing", "telephony"}

// indexFile 是 apk 索引文件名。软件源地址一律指到索引本身而不是目录——
const indexFile = "packages.adb"

// Options 是装配时需要注入的外部事实。
type Options struct {
	// RepoBase 是自有产物的公网访问根（R2 的自定义域名）。必填。
	RepoBase string
// Vermagic 是本次固件对应的内核 ABI 标识。必填——L2 驱动层按它键控，
	Vermagic string
// LocalL1 / LocalKmod 是构建机上已预同步的索引文件路径，非空时构建期
	LocalL1   string
	LocalKmod string
	// UpstreamRoot 覆盖官方发布站（内网镜像或测试替身），空则用官方。
	UpstreamRoot string
}

// Repos 是两份软件源列表。
type Repos struct {
	Build   []string `json:"build"`
	Runtime []string `json:"runtime"`
}

// Assemble 为一个 variant 装配三层软件源。
func Assemble(v resolve.Variant, opt Options) (Repos, error) {
	if opt.RepoBase == "" {
		return Repos{}, errors.New("repos: RepoBase 必填——自有业务包层没有它就没有地址可拼")
	}
	if opt.Vermagic == "" {
		return Repos{}, errors.New("repos: Vermagic 必填——L2 驱动层按它键控，缺了会产出一份看着正常、实际装不上任何 kmod 的固件")
	}

	var (
		repoBase     = strings.TrimRight(opt.RepoBase, "/")
		upstreamRoot = strings.TrimRight(cmp.Or(opt.UpstreamRoot, DefaultUpstreamRoot), "/")
		upstreamBase = upstreamRoot + "/releases/" + v.Line.OpenWrtVersion
		lineBase     = repoBase + "/" + v.Line.ID
		targetPath   = "/targets/" + v.Hardware.TargetKey()
		r            Repos
	)

// L2 两段（内核驱动 + target 基础包）必须整体同源：自编内核的 vermagic
	kernelBase := upstreamBase
	if v.Line.Artifacts == config.ArtifactsSelf {
		kernelBase = lineBase
	}

	add := func(build, runtime string) {
		r.Build = append(r.Build, build)
		r.Runtime = append(r.Runtime, runtime)
	}

	// L1 自有业务包：无论哪条线都来自我们自己的 R2。
	l1 := lineBase + "/packages/" + v.Hardware.Arch + "/" + indexFile
	add(localOr(opt.LocalL1, l1), l1)

	// L2 内核驱动，按 vermagic 键控。
	kmod := kernelBase + targetPath + "/kmods/" + opt.Vermagic + "/" + indexFile
	add(localOr(opt.LocalKmod, kmod), kmod)

	// L2 target 基础包（libc/libgcc/fstools/kernel...）。
	base := kernelBase + targetPath + "/packages/" + indexFile
	add(base, base)

	// L3 官方社区 feed。
	for _, feed := range communityFeeds {
		url := upstreamBase + "/packages/" + v.Hardware.Arch + "/" + feed + "/" + indexFile
		add(url, url)
	}

	// 额外的第三方源，原样追加在最后。
	for _, extra := range v.ExtraRepos {
		add(extra, extra)
	}

	return r, nil
}

// localOr 在给了本地预同步路径时返回 file:// 形式，否则返回在线地址。
func localOr(localPath, online string) string {
	if localPath == "" {
		return online
	}
	return "file://" + localPath
}
