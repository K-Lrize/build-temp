package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/K-Lrize/openwrt-build/internal/toolchain"
)

func toolchainCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "toolchain",
		Short: "工具链相关操作（抠 vermagic、拆分 kmod/base、签名索引）",
	}
	cmd.AddCommand(toolchainCollectCmd(app))
	return mustSubcommand(cmd)
}

func toolchainCollectCmd(app *App) *cobra.Command {
	var openwrtDir, outDir, target, subtarget string
	cmd := &cobra.Command{
		Use:   "collect",
		Short: "从编译完的 openwrt 树中收集、拆分包并签名",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if openwrtDir == "" || outDir == "" || target == "" || subtarget == "" {
				return errors.New("缺少必需参数 (--openwrt-dir, --out-dir, --target, --subtarget)")
			}
			res, err := toolchain.Collect(openwrtDir, outDir, target, subtarget, app.Stderr)
			if err != nil {
				return err
			}

			fmt.Fprintf(app.Stdout, "vermagic: %s（内核 %s），kmod %d 个\n", res.Vermagic, res.KernelVersion, res.KmodCount)

			// 唯一输出契约：结构化写 GITHUB_OUTPUT，供 workflow 直接以
			// steps.<id>.outputs.<key> 读取。不再让 workflow 去 grep stdout。
			for _, kv := range [][2]string{
				{"vermagic", res.Vermagic},
				{"kernel_version", res.KernelVersion},
				{"kmod_count", fmt.Sprint(res.KmodCount)},
				{"sdk_file", res.SDKFile},
				{"sdk_sha256", res.SDKSHA256},
				{"ib_file", res.IBFile},
				{"ib_sha256", res.IBSHA256},
			} {
				writeGitHubOutput(app, kv[0], kv[1])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&openwrtDir, "openwrt-dir", "", "openwrt 源码目录")
	cmd.Flags().StringVar(&outDir, "out-dir", "", "输出目录")
	cmd.Flags().StringVar(&target, "target", "", "target")
	cmd.Flags().StringVar(&subtarget, "subtarget", "", "subtarget")
	return cmd
}
