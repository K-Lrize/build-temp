package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/K-Lrize/openwrt-build/internal/config"
	"github.com/K-Lrize/openwrt-build/internal/resolve"
)

func resolveCmd(app *App) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "resolve <device>@<line> | wrt resolve --all",
		Short: "把 device × line 展开成 variant 并打印 JSON",
		Args: func(cmd *cobra.Command, args []string) error {
			if all && len(args) > 0 {
				return fmt.Errorf("--all 与具体 variant 互斥，收到 %v", args)
			}
			if !all && len(args) == 0 {
				return fmt.Errorf("用法: wrt resolve <device>@<line> | wrt resolve --all")
			}
			if !all && len(args) > 1 {
				return fmt.Errorf("一次只能解析一个 variant，收到 %v", args)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, problems, err := config.Load(app.Root)
			if err != nil {
				return err
			}
			if problems.HasError() {
				return fmt.Errorf("配置有错，先跑 wrt lint：\n%s", problems)
			}

			if all {
				variants, more := resolve.All(cfg)
				if more.HasError() {
					return fmt.Errorf("展开 variant 失败：\n%s", more)
				}
				return emitJSON(app.Stdout, variants)
			}

			variant, err := resolve.One(cfg, args[0])
			if err != nil {
				return err
			}
			return emitJSON(app.Stdout, variant)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "输出全部 variant（JSON 数组）")
	return cmd
}
