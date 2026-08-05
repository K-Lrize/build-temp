// Package files 组装设备的 rootfs overlay。
package rootfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/K-Lrize/openwrt-build/internal/fsutil"
	"github.com/K-Lrize/openwrt-build/internal/resolve"
)

const (
	dirFiles = "rootfs"
	dirGen   = "scripts/build"
	genExt   = ".sh"
)

// Assemble 把有序的 overlay 层合并进 dest，然后按同样的顺序执行 files-gen 脚本。
func Assemble(root string, v resolve.Variant, dest string) error {
	if err := prepareDest(dest); err != nil {
		return err
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return err
	}

	deviceDir := filepath.Join(root, "devices", v.Device)
	for _, layer := range []string{
		filepath.Join(root, dirFiles),
		filepath.Join(deviceDir, dirFiles),
	} {
		if err := copyTree(layer, absDest); err != nil {
			return fmt.Errorf("合并 overlay 层 %s: %w", layer, err)
		}
	}

	env := genEnv(absDest)
	for _, dir := range []string{
		filepath.Join(root, dirGen),
		filepath.Join(deviceDir, dirGen),
	} {
		if err := runGen(root, dir, env); err != nil {
			return err
		}
	}
	return nil
}

func prepareDest(dest string) error {
	entries, err := os.ReadDir(dest)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return os.MkdirAll(dest, 0o755)
	case err != nil:
		return fmt.Errorf("检查目标目录 %s: %w", dest, err)
	case len(entries) > 0:
		return fmt.Errorf("目标目录 %s 非空：就地叠加会把上一次构建的残留打进这次的固件，请换一个空目录或先清空", dest)
	}
	return nil
}

// genEnv 是注入给 files-gen 脚本的唯一事实：overlay 目录在哪。
func genEnv(dest string) []string {
	return append(os.Environ(), "WRT_FILES_DIR="+dest)
}

func runGen(root, dir string, env []string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取 files-gen 目录 %s: %w", dir, err)
	}

	var scripts []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), genExt) {
			scripts = append(scripts, e.Name())
		}
	}
	sort.Strings(scripts)

	for _, name := range scripts {
		path := filepath.Join(dir, name)
		// 显式用 bash 跑而不依赖 shebang 与可执行位：脚本从 git 检出后权限位
		cmd := exec.Command("bash", path)
		cmd.Dir = root
		cmd.Env = env
		cmd.Stdout = os.Stderr // 脚本的输出是构建日志，不能污染 stdout 上的 JSON
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("files-gen 脚本 %s 失败: %w", name, err)
		}
	}
	return nil
}

// copyTree 把 src 整棵树复制进 dst，保留权限位与符号链接。
func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s 不是目录", src)
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type()&fs.ModeSymlink != 0:
			// 保留链接而不是解引用：rootfs 里的符号链接是有意义的内容，
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_ = os.Remove(target) // 后层覆盖前层
			return os.Symlink(link, target)
		case d.Type().IsRegular():
			return fsutil.CopyFile(path, target)
		default:
			// 设备节点、FIFO 之类不该出现在 git 里。静默跳过会让固件少东西
			return fmt.Errorf("%s 是不支持的文件类型 %v", path, d.Type())
		}
	})
}
