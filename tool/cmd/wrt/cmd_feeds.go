package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/K-Lrize/openwrt-build/internal/packages"
)

// runFeeds 就地改写 SDK 的 feeds.conf.default：外部 feed 按 feed/feeds.conf 的
func feedsCmd(app *App) *cobra.Command {
	var sdkDir string

	cmd := &cobra.Command{
		Use:   "feeds",
		Short: "把外部 feed pin 覆盖进 SDK 的 feeds.conf.default 并注入自有 feed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sdkDir == "" {
				return errors.New("必须提供 --sdk 参数")
			}

			defaultPath := filepath.Join(sdkDir, "feeds.conf.default")
			sdkDefault, err := os.ReadFile(defaultPath)
			if err != nil {
				return fmt.Errorf("读取 %s：%w", defaultPath, err)
			}

			pins, err := os.ReadFile(filepath.Join(app.Root, "config", "feeds.conf"))
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("读取 feed/feeds.conf：%w", err)
			}

			feedPath, err := filepath.Abs(filepath.Join(app.Root, "packages"))
			if err != nil {
				return err
			}

			merged := packages.MergeFeedsConf(string(sdkDefault), string(pins), feedPath)
			if err := os.WriteFile(defaultPath, []byte(merged), 0o644); err != nil {
				return fmt.Errorf("写回 %s：%w", defaultPath, err)
			}

			fmt.Fprintf(app.Stdout, "%s 已 pin 外部 feed 并注入自有 feed（%s）\n", defaultPath, packages.CustomFeedName)
			return nil
		},
	}

	cmd.Flags().StringVar(&sdkDir, "sdk", "", "OpenWrt SDK 目录路径")
	return cmd
}
