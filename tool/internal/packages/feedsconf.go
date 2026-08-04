package packages

import "strings"

// CustomFeedName 是注入 SDK 的自有 feed 名。它同时决定产物路径
const CustomFeedName = "custom_feed"

// MergeFeedsConf 把仓库的 pin 列表覆盖进 SDK 自带的 feeds.conf.default，并追加
func MergeFeedsConf(sdkDefault, pins, customPath string) string {
	lines := splitLines(sdkDefault)

	// index 把 feed 名映射到它在 lines 里的下标，供整行覆盖。
	index := make(map[string]int, len(lines))
	for i, ln := range lines {
		if name, ok := feedName(ln); ok {
			index[name] = i
		}
	}

	apply := func(name, newLine string) {
		if i, ok := index[name]; ok {
			lines[i] = newLine
			return
		}
		index[name] = len(lines)
		lines = append(lines, newLine)
	}

	for _, ln := range splitLines(pins) {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		name, ok := feedName(ln)
		if !ok {
			continue
		}
		apply(name, trimmed)
	}

	apply(CustomFeedName, "src-link "+CustomFeedName+" "+customPath)

	return strings.Join(lines, "\n") + "\n"
}

// feedName 取一行的 feed 名（第二个字段），要求第一个字段形如 src-<kind>。
func feedName(line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 || !strings.HasPrefix(fields[0], "src-") {
		return "", false
	}
	return fields[1], true
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
