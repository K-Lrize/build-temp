package main

import (
	"encoding/json"
	"strings"
	"testing"
)



func TestMetaLatestEmitsJSON(t *testing.T) {
	// meta latest 现在产固件 current.json：需要 variant 来算 variant 指纹。
	stdout, _, err := runCLI(t, atRepo(t, "meta", "latest", "vm-armsr@25.12", "--release-id", "r-abc")...)
	if err != nil {
		t.Fatal(err)
	}
	var l struct {
		Fingerprint string `json:"fingerprint"`
		ReleaseID   string `json:"release_id"`
		UpdatedAt   string `json:"updated_at"`
	}
	if err := json.Unmarshal([]byte(stdout), &l); err != nil {
		t.Fatalf("输出不是 JSON: %v\n%s", err, stdout)
	}
	if l.ReleaseID != "r-abc" {
		t.Errorf("release_id = %q", l.ReleaseID)
	}
	if l.Fingerprint == "" {
		t.Error("固件 current.json 应带 variant 指纹（plan 一跳判定的依据）")
	}
	if !strings.HasSuffix(l.UpdatedAt, "Z") {
		t.Errorf("updated_at 应是 UTC: %q", l.UpdatedAt)
	}
}

func TestMetaManifestNeedsRealVariantAndFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"缺 release-id", []string{"meta", "manifest", "vm-armsr@25.12", "--vermagic", "vm"}},
		{"缺 vermagic", []string{"meta", "manifest", "vm-armsr@25.12", "--release-id", "r-1"}},
		{"variant 不存在", []string{"meta", "manifest", "nope@25.12", "--release-id", "r-1", "--vermagic", "vm"}},
		{"未知子命令", []string{"meta", "nope"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := runCLI(t, atRepo(t, tc.args...)...); err == nil {
				t.Error("应当报错")
			}
		})
	}
}
