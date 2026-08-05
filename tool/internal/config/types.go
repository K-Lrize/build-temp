// Package config 是配置的唯一真相：类型定义即 schema，校验只在这里发生一次。
package config

// Artifacts 决定 L2（SDK/IB + kmod + target base 包）整层的来源。
type Artifacts string

const (
	// ArtifactsOfficial：SDK/IB、kmod、target base 包全部借 OpenWrt 官方发布产物。
	ArtifactsOfficial Artifacts = "official"
	// ArtifactsSelf：以上三者全部由本仓库的工具链流水线自行编译产出。
	ArtifactsSelf Artifacts = "self"
)

// Source 指向一棵 OpenWrt 源码树。
type Source struct {
	Repo string `yaml:"repo" json:"repo"`
	// Commit 是唯一权威。tag 理论上不可变，但只信 commit 才能保证
	Commit string `yaml:"commit" json:"commit"`
	// Ref 供人读，CI 不用它检出。唯一的机器用途是核对 openwrt_version 与源码
	Ref string `yaml:"ref,omitempty" json:"ref,omitempty"`
}

// Line 是一条源码基线 + 由它产出的产物身份。
type Line struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description,omitempty"`
	// OpenWrtVersion 是完整 patch 号（如 25.12.5），不允许只写 25.12 让系统猜。
	OpenWrtVersion string    `yaml:"openwrt_version"`
	Artifacts      Artifacts `yaml:"artifacts"`
	Source         *Source   `yaml:"source,omitempty"`

	// RequiresBuild 是派生结论：lines/<id>/ 下 overlay、patches、config 任一非空，
	RequiresBuild bool `yaml:"-"`
}

// Hardware 是一台设备的构建坐标。
type Hardware struct {
	Target    string `yaml:"target" json:"target"`
	Subtarget string `yaml:"subtarget" json:"subtarget"`
	Profile   string `yaml:"profile" json:"profile"`
	Arch      string `yaml:"arch" json:"arch"`
}

// TargetKey 是 target/subtarget 组合，同时也是 R2 上 targets/ 下的路径片段。
func (h Hardware) TargetKey() string { return h.Target + "/" + h.Subtarget }

// PackageSpec 是一层包清单。设备与包集共用同一种形状，因为合并算法把它们
type PackageSpec struct {
	// Include 引用 sets/<name>.yaml，顺序有意义（决定最终包列表的顺序）。
	Include []string `yaml:"include,omitempty"`
	Add     []string `yaml:"add,omitempty"`
	Remove  []string `yaml:"remove,omitempty"`
}

type Image struct {
	RootfsPartsize int `yaml:"rootfs_partsize,omitempty" json:"rootfs_partsize"`
}

// Device 只描述硬件事实与装什么包，一个源码字段都没有。
type Device struct {
	Name     string   `yaml:"name"`
	Hardware Hardware `yaml:"hardware"`
	// Metadata 是事实性硬件资料（soc/wifi/owner/location），不参与构建逻辑。
	Metadata map[string]string `yaml:"metadata,omitempty"`
	// Lines 是这台设备的出货矩阵：每一项与设备展开成一个 variant。
	Lines    []string    `yaml:"lines"`
	Packages PackageSpec `yaml:"packages"`
	Image    Image       `yaml:"image,omitempty"`
	// Repos 是额外的第三方 apk 源，原样追加到三层 repositories 之后。
	Repos []string `yaml:"repos,omitempty"`
}

// Set 是可复用的包清单。不支持嵌套 include —— 合并函数接受有序层列表，
type Set struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description,omitempty"`
	Add         []string `yaml:"add,omitempty"`
	Remove      []string `yaml:"remove,omitempty"`
}
