package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/K-Lrize/openwrt-build/internal/publish"
)

// R2 凭据统一从环境读——绝不作为命令行参数（免得进 shell 历史 / CI 日志）。
func envS3Endpoint() string   { return os.Getenv("WRT_S3_ENDPOINT") }
func envS3Bucket() string     { return os.Getenv("WRT_S3_BUCKET") }
func envAWSAccessKey() string { return os.Getenv("AWS_ACCESS_KEY_ID") }
func envAWSSecretKey() string { return os.Getenv("AWS_SECRET_ACCESS_KEY") }

func publishCmd(app *App) *cobra.Command {
	var verify bool

	cmd := &cobra.Command{
		Use:   "publish <本地目录> <R2 前缀>",
		Short: "把本地产物发布到 R2（内置内容→索引→指针顺序）",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]

			cl, err := publish.NewClient(envS3Endpoint(), envS3Bucket(), envAWSAccessKey(), envAWSSecretKey())
			if err != nil {
				return err
			}

			ctx := context.Background()
			keys, err := cl.PutDir(ctx, dst, src)
			if err != nil {
				return err
			}
			for _, k := range keys {
				fmt.Fprintln(app.Stdout, "  ↑", k)
			}
			fmt.Fprintf(app.Stdout, "已发布 %d 个对象（顺序：内容→索引→指针）\n", len(keys))

			if verify {
				remote, err := cl.List(ctx, dst)
				if err != nil {
					return fmt.Errorf("发布后自证列举失败: %w", err)
				}
				fmt.Fprintf(app.Stdout, "自证：%s 下现在有 %d 个对象\n", dst, len(remote))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&verify, "verify", true, "发布后重新列举一遍前缀，自证对象确实落地")
	return cmd
}
