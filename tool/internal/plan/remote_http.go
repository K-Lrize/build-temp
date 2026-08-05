package plan

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/K-Lrize/openwrt-build/internal/artifacts"
)

// httpRemote 从 R2 的公网访问根读当前已发布的状态。
type httpRemote struct {
	base   string
	client *http.Client
}

// NewHTTPRemote 按 R2 公网根构造一个远端查询器。base 为空时返回 NoRemote：
func NewHTTPRemote(base string, client *http.Client) Remote {
	if base == "" {
		return NoRemote{}
	}
	if client == nil {
		// 变更检测不该因为一个卡住的连接把整条流水线拖到超时。
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &httpRemote{base: strings.TrimRight(base, "/"), client: client}
}

// 三层判定同形：读该层的 current.json，取 Fingerprint。指纹就存在指针里，

func (r *httpRemote) ToolchainFingerprint(line, target, subtarget string) string {
	var cur artifacts.Current
	if !r.fetch(artifacts.CurrentPath(line, target, subtarget), &cur) {
		return ""
	}
	return cur.Fingerprint
}

func (r *httpRemote) PackagesFingerprint(line, arch string) string {
	var cur artifacts.PackagesCurrent
	if !r.fetch(artifacts.PackagesCurrentPath(line, arch), &cur) {
		return ""
	}
	return cur.Fingerprint
}

func (r *httpRemote) FirmwareFingerprint(device, line string) string {
	var cur artifacts.FirmwareCurrent
	if !r.fetch(artifacts.FirmwareCurrentPath(device, line), &cur) {
		return ""
	}
	return cur.Fingerprint
}

// fetch 取一份 JSON。任何失败都返回 false —— 调用方据此当作「不知道」。
func (r *httpRemote) fetch(objectPath string, out any) bool {
	resp, err := r.client.Get(r.base + "/" + objectPath)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	// 限长读取：远端返回一个巨大的对象不该把 plan 撑爆。这几份元数据都是
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false
	}
	return json.Unmarshal(body, out) == nil
}
