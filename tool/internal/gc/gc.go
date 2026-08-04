// Package gc 是引用计数回收的纯判定逻辑：给定"现存哪些对象"与"哪些还活着"，
package gc

import "sort"

// TopN 返回按字典序从新到旧的前 n 个 id。release_id / build_id 形如
func TopN(ids []string, n int) []string {
	s := append([]string(nil), ids...)
	sort.Sort(sort.Reverse(sort.StringSlice(s)))
	if n >= 0 && n < len(s) {
		s = s[:n]
	}
	return s
}

// LiveReleaseIDs 是单个 (device, line) 的存活 release 集合 = 最新 keepN 个
func LiveReleaseIDs(all []string, keepN int, pinned []string) []string {
	allSet := make(map[string]bool, len(all))
	for _, id := range all {
		allSet[id] = true
	}
	live := make(map[string]bool)
	for _, id := range TopN(all, keepN) {
		live[id] = true
	}
	for _, p := range pinned {
		if allSet[p] {
			live[p] = true
		}
	}
	return sortedKeys(live)
}

// Entry 是一个待判定存活性的对象。Key 用于和存活集合比对，Path 是它在 R2 上
type Entry struct {
	Key  string
	Path string
}

// Classify 把 existing 按 liveKeys 分成保留 / 删除两组（各返回 Path）。
func Classify(existing []Entry, liveKeys []string) (keep, del []string) {
	live := make(map[string]bool, len(liveKeys))
	for _, k := range liveKeys {
		live[k] = true
	}
	for _, e := range existing {
		if live[e.Key] {
			keep = append(keep, e.Path)
		} else {
			del = append(del, e.Path)
		}
	}
	return keep, del
}

// OverThreshold 报告"计划删除的比例是否超过阈值百分比"。total==0 视为安全
func OverThreshold(total, deleteCount, thresholdPct int) bool {
	if total == 0 {
		return false
	}
	return deleteCount*100/total > thresholdPct
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
