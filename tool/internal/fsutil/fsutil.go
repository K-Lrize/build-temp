// Package fsutil 收拢仓库里到处重复的文件操作，避免每个包各写一份 copyFile。
package fsutil

import (
	"io"
	"os"
)

// CopyFile 把 src 复制到 dst，保留权限位。先删再建：dst 若是只读文件直接写会失败
// （overlay 后层覆盖前层、uci-defaults 的可执行位都依赖这一点）。
func CopyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	_ = os.Remove(dst)
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// SHA256Hex 返回文件内容的十六进制 sha256。
func SHA256Hex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return hashReader(f)
}
