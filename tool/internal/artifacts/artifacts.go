// Package artifacts 是 R2 布局的唯一事实源：所有对象路径、软件源 URL 的
// 拼法都只在这里定义一次。plan/gc/publish/repos/fetch/meta 一律复用本包，
// 任何人都不该再用字符串拼接或 fmt.Sprintf 手搓一遍 R2 路径。
package artifacts

import (
	"path"
	"strings"
)

// 两种角色的文件名。
const (
	FileCurrent = "current.json" // 每个被查询路径上「当前状态 + 指纹」的可变文件
	FileMeta    = "meta.json"    // 不可变目录里的档案（GC 引用 + 溯源）

	// IndexFile 是 apk 二进制索引文件名。软件源地址一律指到索引本身而非
	// 目录——apk 不会自己去猜索引叫什么。
	IndexFile = "packages.adb"
)

// DefaultUpstreamRoot 是 OpenWrt 官方发布站。self 线的 L2 改指自有 R2，
// 但 L3 社区 feed 与 official 线的 L2 仍然来自这里。
const DefaultUpstreamRoot = "https://downloads.openwrt.org"

// ── 站点：把「相对布局路径」拼成「可访问的绝对 URL」──
//
// R2 公网根与官方镜像根都是 Site；两者的区别只在 base，路径尾巴共用同一套
// 规则（见下方 targetTree / UpstreamTargetDir）。

// Site 是一个以 base 为根的 HTTP(S) 站点。
type Site struct{ base string }

// NewSite 归一化 base（去掉尾斜杠），避免拼出双斜杠。
func NewSite(base string) Site { return Site{base: strings.TrimRight(base, "/")} }

// Base 返回归一化后的根。
func (s Site) Base() string { return s.base }

// URL 把一段布局相对路径拼成绝对 URL。
func (s Site) URL(rel string) string { return s.base + "/" + rel }

// Index 把一个目录指向它里面的 apk 索引文件。
func Index(dir string) string { return path.Join(dir, IndexFile) }

// ── R2 自有产物（根前缀是 <line>/）──

// LineRoot 是一条 line 的命名空间前缀。
func LineRoot(line string) string { return line }

// PackagesDir 是自有业务包的稳定路径。设备的 apk 直接命中这里，只增不删。
func PackagesDir(line, arch string) string {
	return path.Join(LineRoot(line), "packages", arch)
}

// PackagesCurrentPath 与包索引同目录，记录这批包的 feed 指纹，供 plan 一跳比对。
func PackagesCurrentPath(line, arch string) string {
	return path.Join(PackagesDir(line, arch), FileCurrent)
}

// targetTree 是 targets/<target>/<subtarget> 子树：R2 自有产物与官方镜像
// 共用这套尾结构，仅根前缀不同。
func targetTree(prefix, target, subtarget string) string {
	return path.Join(prefix, "targets", target, subtarget)
}

// TargetDir 是一条 line 下某个 target/subtarget 的根。
func TargetDir(line, target, subtarget string) string {
	return targetTree(LineRoot(line), target, subtarget)
}

// CurrentPath 是工具链「当前状态」文件：本目录下唯一可变的文件。
func CurrentPath(line, target, subtarget string) string {
	return path.Join(TargetDir(line, target, subtarget), FileCurrent)
}

// BuildMetaPath 是工具链构建溯源档案的落点：平铺在 TargetDir，覆盖写。
// build_id 继续盖章进 meta.json 内容，但不再作为路径分量。
func BuildMetaPath(line, target, subtarget string) string {
	return path.Join(TargetDir(line, target, subtarget), FileMeta)
}

// KmodsDir 按内核 ABI 键控。已刷机设备固化的地址就在这里，路径必须永久稳定。
func KmodsDir(line, target, subtarget, vermagic string) string {
	return path.Join(TargetDir(line, target, subtarget), "kmods", vermagic)
}

// TargetPackagesDir 是 target 基础包（libc/libgcc/fstools/kernel...）的稳定路径。
func TargetPackagesDir(line, target, subtarget string) string {
	return path.Join(TargetDir(line, target, subtarget), "packages")
}

// ── OpenWrt 官方发布站（根前缀是 releases/<version>/）──
//
// 官方与自有 R2 的 targets/<t>/<s>/ 尾巴完全相同——这正是 official 线能借官方
// L2、self 线改指自有 R2 却共用同一套 repos 装配逻辑的原因。

func upstreamRelease(version string) string { return path.Join("releases", version) }

// UpstreamTargetDir 是官方某版本某 target 的目录。
func UpstreamTargetDir(version, target, subtarget string) string {
	return targetTree(upstreamRelease(version), target, subtarget)
}

// UpstreamKmodsDir 是官方内核驱动仓，按 vermagic 键控。
func UpstreamKmodsDir(version, target, subtarget, vermagic string) string {
	return path.Join(UpstreamTargetDir(version, target, subtarget), "kmods", vermagic)
}

// UpstreamTargetPackagesDir 是官方 target 基础包目录。
func UpstreamTargetPackagesDir(version, target, subtarget string) string {
	return path.Join(UpstreamTargetDir(version, target, subtarget), "packages")
}

// UpstreamFeedDir 是官方社区 feed（base/luci/packages/routing/telephony），按 arch 键控。
func UpstreamFeedDir(version, arch, feed string) string {
	return path.Join(upstreamRelease(version), "packages", arch, feed)
}

// ── 固件（根前缀是 devices/<device>/<line>/）──

// DeviceLineDir 是一个 variant 的固件根。
func DeviceLineDir(device, line string) string {
	return path.Join("devices", device, line)
}

// FirmwareCurrentPath 是固件「当前状态」文件：本目录下唯一可变的文件。
func FirmwareCurrentPath(device, line string) string {
	return path.Join(DeviceLineDir(device, line), FileCurrent)
}

// ReleaseDir 是一次固件发布的不可变目录。
func ReleaseDir(device, line, releaseID string) string {
	return path.Join(DeviceLineDir(device, line), "releases", releaseID)
}

// ReleaseMetaPath 是不可变发布目录里的档案：GC 引用计数 + 溯源。
func ReleaseMetaPath(device, line, releaseID string) string {
	return path.Join(ReleaseDir(device, line, releaseID), FileMeta)
}

// Current 是工具链「当前状态」。
type Current struct {
	Fingerprint         string `json:"fingerprint"`
	BuildID             string `json:"build_id"`
	Vermagic            string `json:"vermagic"`
	SDKArchive          string `json:"sdk_archive"`
	ImageBuilderArchive string `json:"imagebuilder_archive"`
	KmodCount           int    `json:"kmod_count"`
	UpdatedAt           string `json:"updated_at"`
}

// PackagesCurrent 是自有包层「当前状态」：只有一个 feed 指纹供 plan 比对。
type PackagesCurrent struct {
	Fingerprint string `json:"fingerprint"`
	UpdatedAt   string `json:"updated_at"`
}

// FirmwareCurrent 是固件「当前状态」：variant 指纹（plan 一跳）+ 指向的 release。
type FirmwareCurrent struct {
	Fingerprint string `json:"fingerprint"`
	ReleaseID   string `json:"release_id"`
	UpdatedAt   string `json:"updated_at"`
}

// BuildMeta 是一次工具链构建的不可变溯源档案。
type BuildMeta struct {
	BuildID        string `json:"build_id"`
	Line           string `json:"line"`
	Target         string `json:"target"`
	Subtarget      string `json:"subtarget"`
	UpstreamCommit string `json:"upstream_commit"`
	LineTree       string `json:"line_tree"`
	Vermagic       string `json:"vermagic"`
	KernelVersion  string `json:"kernel_version"`
	SDKSHA256      string `json:"sdk_sha256"`
	IBSHA256       string `json:"ib_sha256"`
	CIRunURL       string `json:"ci_run_url"`
	CreatedAt      string `json:"created_at"`
}

// ReleaseMeta 是一次固件发布的不可变档案。
type ReleaseMeta struct {
	ReleaseID      string `json:"release_id"`
	Variant        string `json:"variant"`
	Device         string `json:"device"`
	Line           string `json:"line"`
	BuildID        string `json:"build_id"`
	Vermagic       string `json:"vermagic"`
	UpstreamCommit string `json:"upstream_commit"`
	Fingerprint    string `json:"fingerprint"`
	CIRunURL       string `json:"ci_run_url"`
	CreatedAt      string `json:"created_at"`
}
