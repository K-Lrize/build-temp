// Package repos 装配一个 variant 的三层 apk 软件源地址。
// 所有 URL 的路径部分都来自 internal/artifacts——本包只决定「哪一层来自哪个
// 站点」，不自己拼路径。
package repos

import (
	"cmp"
	"errors"

	"github.com/K-Lrize/openwrt-build/internal/artifacts"
	"github.com/K-Lrize/openwrt-build/internal/config"
	"github.com/K-Lrize/openwrt-build/internal/resolve"
)

// communityFeeds 是 L3 借用的官方社区 feed，按 arch 键控。
var communityFeeds = []string{"base", "luci", "packages", "routing", "telephony"}

// Options 是装配时需要注入的外部事实。
type Options struct {
	// RepoBase 是自有产物的公网访问根（R2 的自定义域名）。必填。
	RepoBase string
	// Vermagic 是本次固件对应的内核 ABI 标识。必填——L2 驱动层按它键控，
	// 缺了会产出一份看着正常、实际装不上任何 kmod 的固件。
	Vermagic string
	// LocalL1 / LocalKmod 是构建机上已预同步的索引文件路径，非空时构建期
	// 列表改指 file://（省一圈公网），运行期列表不受影响。
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
		r2       = artifacts.NewSite(opt.RepoBase)
		upstream = artifacts.NewSite(cmp.Or(opt.UpstreamRoot, artifacts.DefaultUpstreamRoot))
		line     = v.Line.ID
		version  = v.Line.OpenWrtVersion
		target   = v.Hardware.Target
		sub      = v.Hardware.Subtarget
		arch     = v.Hardware.Arch
		self     = v.Line.Artifacts == config.ArtifactsSelf
		r        Repos
	)

	add := func(build, runtime string) {
		r.Build = append(r.Build, build)
		r.Runtime = append(r.Runtime, runtime)
	}

	// L1 自有业务包：无论哪条线都来自我们自己的 R2。
	l1 := r2.URL(artifacts.Index(artifacts.PackagesDir(line, arch)))
	add(localOr(opt.LocalL1, l1), l1)

	// L2 内核驱动 + target 基础包：必须整体同源。自编内核的 vermagic 含
	// 配置哈希，官方那边不可能有——所以 self 线两段一起改指自有 R2。
	var kmod, base string
	if self {
		kmod = r2.URL(artifacts.Index(artifacts.KmodsDir(line, target, sub, opt.Vermagic)))
		base = r2.URL(artifacts.Index(artifacts.TargetPackagesDir(line, target, sub)))
	} else {
		kmod = upstream.URL(artifacts.Index(artifacts.UpstreamKmodsDir(version, target, sub, opt.Vermagic)))
		base = upstream.URL(artifacts.Index(artifacts.UpstreamTargetPackagesDir(version, target, sub)))
	}
	add(localOr(opt.LocalKmod, kmod), kmod)
	add(base, base)

	// L3 官方社区 feed：无论哪条线都借官方同版本线。
	for _, feed := range communityFeeds {
		url := upstream.URL(artifacts.Index(artifacts.UpstreamFeedDir(version, arch, feed)))
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
