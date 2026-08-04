package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/K-Lrize/openwrt-build/internal/config"
	"github.com/K-Lrize/openwrt-build/internal/plan"
	"github.com/K-Lrize/openwrt-build/internal/resolve"
)

func planCmd(app *App) *cobra.Command {
	var repoBase string
	var all bool

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "内容寻址变更检测：这次改动到底要不要重新构建",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, problems, err := config.Load(app.Root)
			if err != nil {
				return err
			}
			if problems.HasError() {
				return fmt.Errorf("配置有错，先跑 wrt lint：\n%s", problems)
			}
			variants, more := resolve.All(cfg)
			if more.HasError() {
				return fmt.Errorf("展开 variant 失败：\n%s", more)
			}

			result, err := plan.Build(app.Root, cfg, variants, plan.NewHTTPRemote(repoBase, nil))
			if err != nil {
				return err
			}
			if !all {
				result = result.Pending()
			}
			return emitJSON(app.Stdout, result)
		},
	}

	cmd.Flags().StringVar(&repoBase, "repo-base", os.Getenv("WRT_REPO_BASE"), "R2 公网访问根；不给就无从判定远端状态，一切都算作需要构建")
	cmd.Flags().BoolVar(&all, "all", false, "连已确认无需构建的条目一起输出，便于核对「为什么这条被跳过了」")
	return cmd
}
