package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// repoRoot 是本仓库自己的配置树。Go 源码在 tool/ 子目录，故 tool/internal/config -> 上三级。
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// TestRepositoryConfigIsValid 是配置树本身的门禁：任何一次改 lines/ devices/
func TestRepositoryConfigIsValid(t *testing.T) {
	cfg, ps, err := Load(repoRoot(t))
	if err != nil {
		t.Fatalf("载入本仓库配置树失败: %v", err)
	}
	if ps.HasError() {
		t.Fatalf("本仓库的配置树有错：\n%s", ps)
	}
	if len(ps) > 0 {
		t.Logf("非阻断的提示：\n%s", ps)
	}
	if len(cfg.Devices) == 0 {
		t.Fatal("一台设备都没载入，配置树布局大概率不对")
	}
}

// intentionalAdditions 登记相对旧仓库**有意**新增的包。迁移的默认要求是逐包
var intentionalAdditions = map[string][]string{
	// zsh 两个插件旧仓库靠构建期钩子拉成 overlay 文件；现在改由自有 feed 的
	"mt3600be": {"zsh-autosuggestions", "zsh-syntax-highlighting"},
}

// TestMigrationPreservesPackageSets 是从 wrt-build 迁过来时唯一有意义的
func TestMigrationPreservesPackageSets(t *testing.T) {
	cfg, _, err := Load(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range cfg.SortedDeviceNames() {
		t.Run(name, func(t *testing.T) {
			baseline := append(readBaseline(t, name), intentionalAdditions[name]...)
			sort.Strings(baseline)
			device := cfg.Devices[name]

			layers := make([]Layer, 0, len(device.Packages.Include)+1)
			for _, setName := range device.Packages.Include {
				set, ok := cfg.Sets[setName]
				if !ok {
					t.Fatalf("include 的包集 %q 不存在", setName)
				}
				layers = append(layers, Layer{
					Name: "set:" + setName,
					Spec: PackageSpec{Add: set.Add, Remove: set.Remove},
				})
			}
			layers = append(layers, Layer{Name: "device:" + name, Spec: device.Packages})

			merged, ps := MergePackages(layers)
			if ps.HasError() {
				t.Fatalf("合并出错：\n%s", ps)
			}

			got := merged.List()
			sort.Strings(got)
			if diff := diffSets(baseline, got); diff != "" {
				t.Fatalf("与迁移前的包列表不一致：\n%s", diff)
			}
		})
	}
}

func readBaseline(t *testing.T, device string) []string {
	t.Helper()
	path := filepath.Join("testdata", "migration", device+".txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取迁移基线 %s: %v（新增设备时请一并补基线，或把这台设备从基线检查中排除）", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	sort.Strings(lines)
	return lines
}

func diffSets(want, got []string) string {
	inWant := map[string]bool{}
	for _, s := range want {
		inWant[s] = true
	}
	inGot := map[string]bool{}
	for _, s := range got {
		inGot[s] = true
	}

	var b strings.Builder
	for _, s := range want {
		if !inGot[s] {
			b.WriteString("  丢了: " + s + "\n")
		}
	}
	for _, s := range got {
		if !inWant[s] {
			b.WriteString("  多了: " + s + "\n")
		}
	}
	return b.String()
}
