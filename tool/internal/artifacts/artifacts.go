// Package artifacts 定义 R2 上的路径规则与元数据 JSON 的形状。
package artifacts

import "path"

// 两种角色的文件名。
const (
	FileCurrent = "current.json" // 每个被查询路径上「当前状态 + 指纹」的可变文件
	FileMeta    = "meta.json"    // 不可变目录里的档案（GC 引用 + 溯源）
)

// LineRoot 是一条 line 的命名空间前缀。
func LineRoot(line string) string { return line }

// PackagesDir 是自有业务包的稳定路径。设备的 apk 直接命中这里，只增不删。
func PackagesDir(line, arch string) string {
	return path.Join(line, "packages", arch)
}

// PackagesCurrentPath 与包索引同目录，记录这批包的 feed 指纹，供 plan 一跳比对。
func PackagesCurrentPath(line, arch string) string {
	return path.Join(PackagesDir(line, arch), FileCurrent)
}

// TargetDir 是一条 line 下某个 target/subtarget 的根。
func TargetDir(line, target, subtarget string) string {
	return path.Join(line, "targets", target, subtarget)
}

// CurrentPath 是工具链「当前状态」文件：本目录下唯一可变的文件。
func CurrentPath(line, target, subtarget string) string {
	return path.Join(TargetDir(line, target, subtarget), FileCurrent)
}



// KmodsDir 按内核 ABI 键控。已刷机设备固化的地址就在这里，路径必须永久稳定。
func KmodsDir(line, target, subtarget, vermagic string) string {
	return path.Join(TargetDir(line, target, subtarget), "kmods", vermagic)
}

// TargetPackagesDir 是 target 基础包（libc/libgcc/fstools/kernel...）的稳定路径。
func TargetPackagesDir(line, target, subtarget string) string {
	return path.Join(TargetDir(line, target, subtarget), "packages")
}

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
