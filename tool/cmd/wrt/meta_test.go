package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// meta 现在是纯序列化器：不 load 配置、不重算指纹，指纹一律由 --fingerprint 传入。

func TestMetaReleaseSerializesFlags(t *testing.T) {
	stdout, _, err := runCLI(t, "meta", "release", "vm-armsr@25.12",
		"--release-id", "r-abc", "--vermagic", "vm", "--fingerprint", "FP123",
		"--upstream-commit", "c0ffee")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		ReleaseID      string `json:"release_id"`
		Variant        string `json:"variant"`
		Device         string `json:"device"`
		Line           string `json:"line"`
		Fingerprint    string `json:"fingerprint"`
		UpstreamCommit string `json:"upstream_commit"`
		CreatedAt      string `json:"created_at"`
	}
	if err := json.Unmarshal([]byte(stdout), &m); err != nil {
		t.Fatalf("输出不是 JSON: %v\n%s", err, stdout)
	}
	if m.ReleaseID != "r-abc" || m.Fingerprint != "FP123" {
		t.Errorf("字段没透传: %+v", m)
	}
	if m.Device != "vm-armsr" || m.Line != "25.12" {
		t.Errorf("device/line 应由 variant id 拆出: %+v", m)
	}
	if !strings.HasSuffix(m.CreatedAt, "Z") {
		t.Errorf("created_at 应是 UTC: %q", m.CreatedAt)
	}
}

func TestMetaPointerNeedsFingerprintAndReleaseID(t *testing.T) {
	// 固件 current.json 只吃 --release-id 与 --fingerprint，不再接受 variant 位置参数。
	stdout, _, err := runCLI(t, "meta", "pointer", "--release-id", "r-1", "--fingerprint", "FP")
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		Fingerprint string `json:"fingerprint"`
		ReleaseID   string `json:"release_id"`
	}
	if err := json.Unmarshal([]byte(stdout), &c); err != nil {
		t.Fatalf("输出不是 JSON: %v\n%s", err, stdout)
	}
	if c.Fingerprint != "FP" || c.ReleaseID != "r-1" {
		t.Errorf("字段没透传: %+v", c)
	}
}

func TestMetaRejectsMissingFlagsAndBadSubcommand(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"release 缺 fingerprint", []string{"meta", "release", "vm-armsr@25.12", "--release-id", "r-1", "--vermagic", "vm"}},
		{"release variant 非法", []string{"meta", "release", "nope", "--release-id", "r-1", "--vermagic", "vm", "--fingerprint", "f"}},
		{"pointer 缺 fingerprint", []string{"meta", "pointer", "--release-id", "r-1"}},
		{"pointer-packages 缺 fingerprint", []string{"meta", "pointer-packages"}},
		{"未知子命令", []string{"meta", "nope"}},
		{"pointer 不再吃位置参数", []string{"meta", "pointer", "vm-armsr@25.12", "--release-id", "r-1", "--fingerprint", "f"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := runCLI(t, tc.args...); err == nil {
				t.Error("应当报错")
			}
		})
	}
}

func TestMetaPointerPackagesSerializes(t *testing.T) {
	stdout, _, err := runCLI(t, "meta", "pointer-packages", "--fingerprint", "FEEDFP")
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.Unmarshal([]byte(stdout), &c); err != nil {
		t.Fatalf("输出不是 JSON: %v\n%s", err, stdout)
	}
	if c.Fingerprint != "FEEDFP" {
		t.Errorf("fingerprint = %q", c.Fingerprint)
	}
}
