package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/K-Lrize/openwrt-build/internal/config"
	"github.com/K-Lrize/openwrt-build/internal/repos"
	"github.com/K-Lrize/openwrt-build/internal/resolve"
)

func reposCmd(app *App) *cobra.Command {
	var opt repos.Options

	cmd := &cobra.Command{
		Use:   "repos <device>@<line>",
		Short: "装配三层 apk 软件源地址（构建期 / 运行期两份）",
		Args:  cobra.ExactArgs(1),
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
			assembled, err := repos.Assemble(variant, opt)
			if err != nil {
				return err
			}
			return emitJSON(app.Stdout, assembled)
		},
	}

	cmd.Flags().StringVar(&opt.RepoBase, "repo-base", os.Getenv("WRT_REPO_BASE"), "自有产物的公网访问根")
	cmd.Flags().StringVar(&opt.Vermagic, "vermagic", "", "本次固件对应的内核 ABI 标识")
	cmd.Flags().StringVar(&opt.LocalL1, "local-l1", "", "构建机上已预同步的自有包索引（只影响构建期列表）")
	cmd.Flags().StringVar(&opt.LocalKmod, "local-kmod", "", "构建机上已预同步的 kmod 索引（只影响构建期列表）")
	cmd.Flags().StringVar(&opt.UpstreamRoot, "upstream-root", "", "覆盖官方发布站（内网镜像）")
	return cmd
}
