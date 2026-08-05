package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/K-Lrize/openwrt-build/internal/config"
	"github.com/K-Lrize/openwrt-build/internal/diag"
	"github.com/K-Lrize/openwrt-build/internal/pkglint"
	"github.com/K-Lrize/openwrt-build/internal/resolve"
)

func lintCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "lint",
		Short: "校验全部配置与自有软件包 Makefile",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, problems, err := config.Load(app.Root)
			if err != nil {
				return err
			}
			// 跨层的包冲突要展开 variant 才看得见（单个文件内部自洽不代表合并后自洽），
			if !problems.HasError() {
				if _, more := resolve.All(cfg); len(more) > 0 {
					problems = append(problems, more...)
				}
			}

			_, feedProblems, err := pkglint.Load(app.Root)
			if err != nil {
				return err
			}
			problems = append(problems, feedProblems...)

			for _, p := range problems {
				fmt.Fprintln(app.Stdout, p)
			}

			errCount := problems.Count(diag.SeverityError)
			warnCount := problems.Count(diag.SeverityWarn)
			fmt.Fprintf(app.Stdout, "\n%d 个错误，%d 个提示\n", errCount, warnCount)
			if errCount > 0 {
				return fmt.Errorf("配置校验未通过：%d 个错误", errCount)
			}
			return nil
		},
	}
}
