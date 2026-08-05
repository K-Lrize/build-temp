package config

import (
	"reflect"
	"testing"
)

func layer(name string, add, remove []string) Layer {
	return Layer{Name: name, Spec: PackageSpec{Add: add, Remove: remove}}
}

func TestMergePackages(t *testing.T) {
	tests := []struct {
		name   string
		layers []Layer
		add    []string
		remove []string
		rules  []string
	}{
		{
			name:   "空层列表",
			layers: nil,
		},
		{
			name:   "单层直通",
			layers: []Layer{layer("device:vm", []string{"luci", "qemu-ga"}, nil)},
			add:    []string{"luci", "qemu-ga"},
		},
		{
			name: "层序决定最终顺序：先 set 后 device",
			layers: []Layer{
				layer("set:common", []string{"curl", "jq"}, nil),
				layer("set:base-router", []string{"ip-full"}, nil),
				layer("device:mt3600be", []string{"sing-box"}, nil),
			},
			add: []string{"curl", "jq", "ip-full", "sing-box"},
		},
		{
			// 去重保留首次出现的位置：包列表顺序会进指纹，靠后去重会让
			name: "跨层重复的包只保留首次出现的位置",
			layers: []Layer{
				layer("set:a", []string{"curl", "jq"}, nil),
				layer("set:b", []string{"htop", "curl"}, nil),
			},
			add: []string{"curl", "jq", "htop"},
		},
		{
			name: "同层内部重复也去重",
			layers: []Layer{
				layer("set:a", []string{"curl", "curl", "jq"}, nil),
			},
			add: []string{"curl", "jq"},
		},
		{
			name: "remove 取并集并去重",
			layers: []Layer{
				layer("set:base-router", nil, []string{"dnsmasq", "wpad-basic"}),
				layer("device:mt3600be", nil, []string{"wpad-basic", "wpad-mbedtls"}),
			},
			remove: []string{"dnsmasq", "wpad-basic", "wpad-mbedtls"},
		},
		{
			name: "add 与 remove 分别成列，互不干扰",
			layers: []Layer{
				layer("set:base-router", []string{"dnsmasq-full"}, []string{"dnsmasq"}),
				layer("device:mt3600be", []string{"sing-box"}, nil),
			},
			add:    []string{"dnsmasq-full", "sing-box"},
			remove: []string{"dnsmasq"},
		},

		// 跨层冲突：后层覆盖前层。
		{
			name: "set 装的包被 device 卸掉 —— 局部覆盖整体",
			layers: []Layer{
				layer("set:base-router", []string{"dnsmasq-full"}, nil),
				layer("device:mt3600be", nil, []string{"dnsmasq-full"}),
			},
			add:    nil,
			remove: []string{"dnsmasq-full"},
		},
		{
			name: "set 卸的包被 device 装回来 —— 局部覆盖整体",
			layers: []Layer{
				layer("set:base-router", nil, []string{"dnsmasq"}),
				layer("device:mt3600be", []string{"dnsmasq"}, nil),
			},
			add:    []string{"dnsmasq"},
			remove: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ps := MergePackages(tc.layers)

			if !equalStrings(got.Add, tc.add) {
				t.Errorf("add 不符\n want: %v\n got:  %v", tc.add, got.Add)
			}
			if !equalStrings(got.Remove, tc.remove) {
				t.Errorf("remove 不符\n want: %v\n got:  %v", tc.remove, got.Remove)
			}
			if rules := ps.Rules(); !reflect.DeepEqual(rules, tc.rules) {
				t.Errorf("规则不符\n want: %v\n got:  %v\n%s", tc.rules, rules, ps)
			}
		})
	}
}

func TestPackagesList(t *testing.T) {
	// ImageBuilder 的 `make image PACKAGES=` 语法：装的包直接写，卸的包带 -
	p := Packages{
		Add:    []string{"luci", "sing-box"},
		Remove: []string{"dnsmasq", "wpad-basic"},
	}
	want := []string{"luci", "sing-box", "-dnsmasq", "-wpad-basic"}
	if got := p.List(); !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestPackagesListEmpty(t *testing.T) {
	if got := (Packages{}).List(); len(got) != 0 {
		t.Fatalf("空包集应产出空列表，得到 %v", got)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}
