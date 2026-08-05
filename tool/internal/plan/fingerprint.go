// Package plan 回答「这次改动到底要不要重新构建」。
package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/K-Lrize/openwrt-build/internal/config"
	"github.com/K-Lrize/openwrt-build/internal/resolve"
)

// Fingerprints 是一个 variant 的三层指纹。
type Fingerprints struct {
	// LineTree 是 line 目录树的哈希，**不含** upstream commit。
	LineTree string `json:"line_tree"`
	Line     string `json:"line"`
	Feed     string `json:"feed"`
	Variant  string `json:"variant"`
}

// Computer 按需计算指纹，并缓存已经算过的目录树哈希。
type Computer struct {
	root  string
	cache map[string]string
}

func NewComputer(root string) *Computer {
	return &Computer{root: root, cache: map[string]string{}}
}

// For 算出一个 variant 的三层指纹。
func (c *Computer) For(cfg *config.Config, v resolve.Variant) (Fingerprints, error) {
	lineDir := path.Join("lines", v.Line.ID)
	lineTree, err := c.hashPaths(
		path.Join(lineDir, "line.yaml"),
		path.Join(lineDir, "overlay"),
		path.Join(lineDir, "patches"),
		path.Join(lineDir, "config"),
	)
	if err != nil {
		return Fingerprints{}, err
	}

	// upstream commit 是源码基线的另一半，但它不在磁盘上——它是 line.yaml
	upstreamCommit := ""
	if v.Line.Source != nil {
		upstreamCommit = v.Line.Source.Commit
	}
	lineFP := combine(lineTree, upstreamCommit)

	feedTree, err := c.hashPaths("packages")
	if err != nil {
		return Fingerprints{}, err
	}
	feedFP := combine(feedTree, lineFP)

	// 只把这台设备**实际 include 的**包集计入。
	device, ok := cfg.Devices[v.Device]
	if !ok {
		return Fingerprints{}, fmt.Errorf("设备 %q 不存在", v.Device)
	}
	deviceDir := path.Join("devices", v.Device)
	paths := []string{
		path.Join(deviceDir, "device.yaml"),
		path.Join(deviceDir, "rootfs"),
		path.Join(deviceDir, "scripts/build"),
		path.Join(lineDir, "rootfs"),
		"rootfs",
		"scripts/build",
	}
	for _, setName := range device.Packages.Include {
		paths = append(paths, path.Join("sets", setName+".yaml"))
	}
	deviceTree, err := c.hashPaths(paths...)
	if err != nil {
		return Fingerprints{}, err
	}

	// 最终包列表单独参与：它是合并后的结果，不等于任何一份输入文件的内容。
	variantFP := combine(
		deviceTree,
		hashString(strings.Join(v.Packages, " ")),
		lineFP,
		feedFP,
	)

	return Fingerprints{LineTree: lineTree, Line: lineFP, Feed: feedFP, Variant: variantFP}, nil
}

// hashPaths 对一组相对仓库根的路径求内容哈希。
func (c *Computer) hashPaths(paths ...string) (string, error) {
	var lines []string
	for _, p := range paths {
		if cached, ok := c.cache[p]; ok {
			lines = append(lines, cached)
			continue
		}
		one, err := c.hashOne(p)
		if err != nil {
			return "", err
		}
		c.cache[p] = one
		lines = append(lines, one)
	}
	// 先各自算好再排序：调用方传入的顺序不该影响结果。
	sort.Strings(lines)
	return hashString(strings.Join(lines, "\n")), nil
}

func (c *Computer) hashOne(rel string) (string, error) {
	full := filepath.Join(c.root, rel)
	info, err := os.Stat(full)
	if errors.Is(err, fs.ErrNotExist) {
		return rel + " -", nil
	}
	if err != nil {
		return "", err
	}

	if !info.IsDir() {
		sum, err := hashFile(full, info)
		if err != nil {
			return "", err
		}
		return rel + " " + sum, nil
	}

	var entries []string
	err = filepath.WalkDir(full, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		sub, err := filepath.Rel(full, p)
		if err != nil {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		sum, err := hashFile(p, fi)
		if err != nil {
			return err
		}
		entries = append(entries, filepath.ToSlash(sub)+" "+sum)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("哈希目录 %s: %w", rel, err)
	}
	sort.Strings(entries)
	return rel + " " + hashString(strings.Join(entries, "\n")), nil
}

// hashFile 把内容与可执行位一起算进去。
func hashFile(path string, info fs.FileInfo) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	exec := "-"
	if info.Mode().Perm()&0o111 != 0 {
		exec = "x"
	}
	return exec + hex.EncodeToString(h.Sum(nil)), nil
}

// combine 把若干段拼成一个指纹。用 ":" 分隔而不是直接连接，避免
func combine(parts ...string) string {
	return hashString(strings.Join(parts, ":"))
}

func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
