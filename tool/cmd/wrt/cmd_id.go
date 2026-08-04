package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/K-Lrize/openwrt-build/internal/id"
)

// idCmd 从 CI 环境生成 build_id 或 release_id，供 workflow 一处取值、下游共享。
func idCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "id <build|release>",
		Short: "从 CI 环境生成 build_id / release_id（供 workflow 一处取值）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			run := os.Getenv("GITHUB_RUN_NUMBER")
			if run == "" {
				run = "0"
			}
			sha := id.Short(os.Getenv("GITHUB_SHA"))
			if sha == "" {
				sha = "0000000"
			}

			now := time.Now()
			var (
				out string
				err error
			)
			switch args[0] {
			case "build":
				out, err = id.Build(now, run, sha)
			case "release":
				out, err = id.Release(now, run, sha)
			default:
				return fmt.Errorf("未知的 id 类型 %q，应为 build 或 release", args[0])
			}
			if err != nil {
				return err
			}
			fmt.Fprintln(app.Stdout, out)
			return nil
		},
	}
}
