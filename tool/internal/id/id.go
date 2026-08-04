// Package id 生成 build_id 与 release_id。
package id

import (
	"fmt"
	"strings"
	"time"
)

// timeLayout 是 id 第一段的时间格式：UTC、紧凑、可排序。
const timeLayout = "20060102-150405"

// Build 拼一个 build_id。run 与 sha 都不能为空——空了就等于少一段防撞。
func Build(now time.Time, run, sha string) (string, error) {
	r, s := strings.TrimSpace(run), strings.TrimSpace(sha)
	if r == "" || s == "" {
		return "", fmt.Errorf("id: run 与 sha 都不能为空（run=%q sha=%q）", run, sha)
	}
	return now.UTC().Format(timeLayout) + "-" + r + "-" + s, nil
}

// Release 拼一个 release_id：build_id 格式加 "r" 前缀，用来一眼区分是发布还是
func Release(now time.Time, run, sha string) (string, error) {
	b, err := Build(now, run, sha)
	if err != nil {
		return "", err
	}
	return "r" + b, nil
}

// Short 取一个 commit 哈希的前 7 位——id 第三段的常规取值。短于 7 位原样返回
func Short(sha string) string {
	s := strings.TrimSpace(sha)
	if len(s) <= 7 {
		return s
	}
	return s[:7]
}
