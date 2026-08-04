package rootfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/K-Lrize/openwrt-build/internal/config"
	"github.com/K-Lrize/openwrt-build/internal/resolve"
)

func testVariant() resolve.Variant {
	return resolve.Variant{
		ID:     "mt3600be@25.12",
		Device: "mt3600be",
		Line:   resolve.LineFacts{ID: "25.12", OpenWrtVersion: "25.12.5", Artifacts: config.ArtifactsOfficial},
		Hardware: config.Hardware{
			Target: "mediatek", Subtarget: "filogic",
			Profile: "glinet_gl-mt3600be", Arch: "aarch64_cortex-a53",
		},
		Packages: []string{"zsh", "luci", "-dnsmasq"},
	}
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func atRepo(t *testing.T, root string, elem ...string) {
	t.Helper()
	path := filepath.Join(append([]string{root}, elem...)...)
	writeFile(t, path, "", 0o644)
}

func mustChmod(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func TestDeviceLayerOverridesCommonLayer(t *testing.T) {
	root := t.TempDir()
	atRepo(t, root, "rootfs", "etc", "banner", "common banner")
	atRepo(t, root, "devices", "router", "rootfs", "etc", "banner", "router banner")
	writeFile(t, filepath.Join(root, "rootfs/root/.zshrc"), "通用 zshrc\n", 0o644)
	writeFile(t, filepath.Join(root, "devices/mt3600be/rootfs/root/.zshrc"), "设备 zshrc\n", 0o644)
	writeFile(t, filepath.Join(root, "devices/mt3600be/rootfs/etc/sysctl.d/99-bbr.conf"), "bbr\n", 0o644)

	dest := filepath.Join(t.TempDir(), "overlay")
	if err := Assemble(root, testVariant(), dest); err != nil {
		t.Fatal(err)
	}

	if got := read(t, filepath.Join(dest, "etc/banner")); got != "router banner" {
		t.Errorf("设备层应覆盖通用层: %q", got)
	}
	if got := read(t, filepath.Join(dest, "root/.zshrc")); got != "设备 zshrc\n" {
		t.Errorf("同路径应由设备层覆盖: %q", got)
	}
	if got := read(t, filepath.Join(dest, "etc/sysctl.d/99-bbr.conf")); got != "bbr\n" {
		t.Errorf("设备层独有文件应存在: %q", got)
	}
}

func TestExecutableBitIsPreserved(t *testing.T) {
	root := t.TempDir()
	atRepo(t, root, "rootfs", "etc", "uci-defaults", "00-base", "#!/bin/sh\nexit 0\n")
	mustChmod(t, filepath.Join(root, "rootfs", "etc", "uci-defaults", "00-base"), 0o755)
	writeFile(t, filepath.Join(root, "rootfs/etc/banner"), "hi\n", 0o644)

	dest := filepath.Join(t.TempDir(), "overlay")
	if err := Assemble(root, testVariant(), dest); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(dest, "etc/uci-defaults/00-base"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("可执行位丢失: %v", info.Mode())
	}
	plain, err := os.Stat(filepath.Join(dest, "etc/banner"))
	if err != nil {
		t.Fatal(err)
	}
	if plain.Mode().Perm()&0o111 != 0 {
		t.Errorf("普通文件不该变成可执行: %v", plain.Mode())
	}
}

func TestSymlinksArePreservedNotDereferenced(t *testing.T) {
	root := t.TempDir()
	atRepo(t, root, "rootfs", "etc", "real.conf", "real content")
	if err := os.Symlink("real.conf", filepath.Join(root, "rootfs", "etc", "link.conf")); err != nil {
		t.Skipf("这个文件系统不支持符号链接: %v", err)
	}

	dest := filepath.Join(t.TempDir(), "overlay")
	if err := Assemble(root, testVariant(), dest); err != nil {
		t.Fatal(err)
	}

	target, err := os.Readlink(filepath.Join(dest, "etc/link.conf"))
	if err != nil {
		t.Fatalf("符号链接应当原样保留而不是被解引用: %v", err)
	}
	if target != "real.conf" {
		t.Errorf("链接目标 = %q", target)
	}
}

func TestMissingLayersAreFine(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "overlay")
	if err := Assemble(t.TempDir(), testVariant(), dest); err != nil {
		t.Fatalf("层目录不存在不该报错: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("dest 仍应被创建出来: %v", err)
	}
}

func TestNonEmptyDestIsRejected(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "rootfs/etc/banner"), "hi\n", 0o644)

	dest := t.TempDir()
	writeFile(t, filepath.Join(dest, "leftover"), "旧的\n", 0o644)

	if err := Assemble(root, testVariant(), dest); err == nil {
		t.Fatal("dest 非空时应当报错")
	}
}

func TestGenScriptsRunInOrderAfterFilesAreInPlace(t *testing.T) {
	root := t.TempDir()
	atRepo(t, root, "rootfs", "a", "1")
	atRepo(t, root, "scripts/build", "01-first.sh", "#!/bin/sh\necho \"first: $(cat $WRT_OVERLAY_DIR/a)\" >> $WRT_OVERLAY_DIR/order\n")
	atRepo(t, root, "scripts/build", "02-second.sh", "#!/bin/sh\necho \"second\" >> $WRT_OVERLAY_DIR/order\n")
	mustChmod(t, filepath.Join(root, "scripts/build", "01-first.sh"), 0o755)
	mustChmod(t, filepath.Join(root, "scripts/build", "02-second.sh"), 0o755)

	dest := filepath.Join(t.TempDir(), "overlay")
	if err := Assemble(root, testVariant(), dest); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(dest, "order")); got != "first: 1\nsecond\n" {
		t.Errorf("执行顺序 = %q", got)
	}
}

func TestDeviceGenScriptsRunAfterCommonOnes(t *testing.T) {
	root := t.TempDir()
	atRepo(t, root, "scripts/build", "10-common.sh", "#!/bin/sh\necho \"common\" >> $WRT_OVERLAY_DIR/order\n")
	mustChmod(t, filepath.Join(root, "scripts/build", "10-common.sh"), 0o755)

	atRepo(t, root, "devices", "router", "scripts/build", "01-device.sh", "#!/bin/sh\necho \"device\" >> $WRT_OVERLAY_DIR/order\n")
	mustChmod(t, filepath.Join(root, "devices", "router", "scripts/build", "01-device.sh"), 0o755)

	dest := filepath.Join(t.TempDir(), "overlay")
	if err := Assemble(root, testVariant(), dest); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(dest, "order")); got != "common\ndevice\n" {
		t.Errorf("执行顺序 = %q", got)
	}
}

func TestGenScriptsOnlyGetOverlayDir(t *testing.T) {
	root := t.TempDir()
	atRepo(t, root, "scripts/build", "fail.sh", `#!/bin/sh
env > $WRT_OVERLAY_DIR/env
`)
	mustChmod(t, filepath.Join(root, "scripts/build", "dump.sh"), 0o755)

	dest := filepath.Join(t.TempDir(), "overlay")
	if err := Assemble(root, testVariant(), dest); err != nil {
		t.Fatal(err)
	}

	got := read(t, filepath.Join(dest, "env"))
	if !strings.Contains(got, "files_dir="+dest) {
		t.Errorf("WRT_FILES_DIR 应指向 overlay 目录:\n%s", got)
	}
	// variant / device / 包列表不该被注入——注入它们就是在邀请脚本做决定。
	for _, want := range []string{"variant=<unset>", "device=<unset>", "packages=<unset>"} {
		if !strings.Contains(got, want) {
			t.Errorf("variant 上下文不该出现在 files-gen 环境里，缺少 %q:\n%s", want, got)
		}
	}
}

func TestFailingGenScriptAbortsAssembly(t *testing.T) {
	// 脚本失败被吞掉，会产出一份少了内容却看着正常的 overlay。
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "files-gen/boom.sh"),
		"#!/usr/bin/env bash\necho 出错了 >&2\nexit 3\n", 0o755)

	dest := filepath.Join(t.TempDir(), "overlay")
	err := Assemble(root, testVariant(), dest)
	if err == nil {
		t.Fatal("脚本失败必须中断组装")
	}
	if !strings.Contains(err.Error(), "boom.sh") {
		t.Errorf("错误信息要点名是哪个脚本: %v", err)
	}
}

func TestNonShellFilesInGenDirAreIgnored(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "files-gen/README.md"), "不是脚本\n", 0o644)
	dest := filepath.Join(t.TempDir(), "overlay")
	if err := Assemble(root, testVariant(), dest); err != nil {
		t.Fatalf("files-gen 目录里的非 .sh 文件应当被忽略: %v", err)
	}
}
