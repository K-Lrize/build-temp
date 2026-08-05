package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/K-Lrize/openwrt-build/internal/config"
	"github.com/K-Lrize/openwrt-build/internal/resolve"
	"github.com/K-Lrize/openwrt-build/internal/rootfs"
)

func filesCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "files <device>@<line> <dest>",
		Short: "组装设备的 rootfs overlay（合并文件层 + 跑 files-gen 脚本）",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, problems, err := config.Load(app.Root)
			if err != nil {
				return err
			}
			if problems.HasError() {
				return fmt.Errorf("配置有错，先跑 wrt lint：\n%s", problems)
			}

			variant, err := resolve.One(cfg, args[0])
			if err != nil {
				return err
			}
			if err := rootfs.Assemble(app.Root, variant, args[1]); err != nil {
				return err
			}
			fmt.Fprintf(app.Stdout, "%s 的 overlay 已组装到 %s\n", variant.ID, args[1])
			return nil
		},
	}
}
