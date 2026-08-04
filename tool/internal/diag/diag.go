// Package diag 是全仓库共用的诊断类型。
package diag

import (
	"fmt"
	"sort"
	"strings"
)

// Severity 区分「必须修」与「值得看一眼」。
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarn
)

func (s Severity) String() string {
	if s == SeverityWarn {
		return "warn"
	}
	return "error"
}

// Problem 是一条校验发现。Rule 是稳定的规则代号，用于在文档与排障时互相指认；
type Problem struct {
	Source   string
	Rule     string
	Message  string
	Severity Severity
}

func (p Problem) String() string {
	src := p.Source
	if src == "" {
		src = "<unknown>"
	}
	return fmt.Sprintf("%s: [%s] %s (%s)", src, p.Severity, p.Message, p.Rule)
}

// Problems 是校验结果的累积容器。
type Problems []Problem

func (ps Problems) Errorf(rule, format string, args ...any) Problems {
	return append(ps, Problem{Rule: rule, Message: fmt.Sprintf(format, args...), Severity: SeverityError})
}

func (ps Problems) Warnf(rule, format string, args ...any) Problems {
	return append(ps, Problem{Rule: rule, Message: fmt.Sprintf(format, args...), Severity: SeverityWarn})
}

// WithSource 给一批还没有出处的问题回填来源文件。
func (ps Problems) WithSource(source string) Problems {
	out := make(Problems, len(ps))
	for i, p := range ps {
		if p.Source == "" {
			p.Source = source
		}
		out[i] = p
	}
	return out
}

// HasError 报告是否存在 Error 级问题。只有 Warn 时应当放行。
func (ps Problems) HasError() bool {
	for _, p := range ps {
		if p.Severity == SeverityError {
			return true
		}
	}
	return false
}

func (ps Problems) Count(s Severity) int {
	n := 0
	for _, p := range ps {
		if p.Severity == s {
			n++
		}
	}
	return n
}

// Rules 列出结果里出现过的规则代号，去重且有序——测试断言「触发了哪几条规则」
func (ps Problems) Rules() []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range ps {
		if !seen[p.Rule] {
			seen[p.Rule] = true
			out = append(out, p.Rule)
		}
	}
	sort.Strings(out)
	return out
}

func (ps Problems) String() string {
	var b strings.Builder
	for _, p := range ps {
		b.WriteString(p.String())
		b.WriteByte('\n')
	}
	return b.String()
}
