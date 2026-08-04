package config

import "github.com/K-Lrize/openwrt-build/internal/diag"

// Layer 是参与合并的一层包清单。Name 只用于报错定位（"set:base-router"、
type Layer struct {
	Name string
	Spec PackageSpec
}

// Packages 是合并后的最终包清单。
type Packages struct {
	Add    []string
	Remove []string
}

// List 产出 ImageBuilder `make image PACKAGES=` 需要的形式：装的包直接写，
func (p Packages) List() []string {
	out := make([]string, 0, len(p.Add)+len(p.Remove))
	out = append(out, p.Add...)
	for _, name := range p.Remove {
		out = append(out, "-"+name)
	}
	return out
}

// MergePackages 把有序的层列表合并成最终包清单。
func MergePackages(layers []Layer) (Packages, diag.Problems) {
	var result Packages

	// 第一遍：计算每个包的最终命运（后浪推前浪）
	finalState := make(map[string]bool) // true: add, false: remove
	for _, l := range layers {
		for _, name := range l.Spec.Add {
			finalState[name] = true
		}
		for _, name := range l.Spec.Remove {
			finalState[name] = false
		}
	}

	// 第二遍：按最终命运生成列表，去重并保留首次出现位置
	addedSeen := make(map[string]bool)
	removedSeen := make(map[string]bool)

	for _, l := range layers {
		for _, name := range l.Spec.Add {
			if finalState[name] {
				if !addedSeen[name] {
					addedSeen[name] = true
					result.Add = append(result.Add, name)
				}
			}
		}
		for _, name := range l.Spec.Remove {
			if !finalState[name] {
				if !removedSeen[name] {
					removedSeen[name] = true
					result.Remove = append(result.Remove, name)
				}
			}
		}
	}

	return result, nil
}
