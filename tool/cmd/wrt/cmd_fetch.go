package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/K-Lrize/openwrt-build/internal/config"
	"github.com/K-Lrize/openwrt-build/internal/fetch"
)

type fetchOptions struct {
	artifacts      string
	openwrtVersion string
	target         string
	subtarget      string
	repoBase       string
	line           string
	outDir         string
}

func fetchCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "拉取上游或自建的 SDK / ImageBuilder 归档，并校验哈希解压",
	}
	cmd.AddCommand(fetchArchiveCmd(app, fetch.SDK, "sdk", "拉取 SDK 并解压"))
	cmd.AddCommand(fetchArchiveCmd(app, fetch.IB, "ib", "拉取 ImageBuilder 并解压，同时输出 vermagic 到 GITHUB_OUTPUT"))
	return mustSubcommand(cmd)
}

func fetchArchiveCmd(app *App, kind fetch.Kind, use, short string) *cobra.Command {
	var opts fetchOptions
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFetch(app, kind, opts)
		},
	}
	cmd.Flags().StringVar(&opts.artifacts, "artifacts", "", "official | self")
	cmd.Flags().StringVar(&opts.openwrtVersion, "openwrt-version", "", "OpenWrt 版本号 (e.g. 23.05.3)")
	cmd.Flags().StringVar(&opts.target, "target", "", "目标架构")
	cmd.Flags().StringVar(&opts.subtarget, "subtarget", "", "子目标架构")
	cmd.Flags().StringVar(&opts.repoBase, "repo-base", "", "R2 仓库基地址 (对于 self)")
	cmd.Flags().StringVar(&opts.line, "line", "", "Line ID (对于 self)")
	cmd.Flags().StringVar(&opts.outDir, "out", "", "解压目标目录")
	return cmd
}

func runFetch(app *App, kind fetch.Kind, opts fetchOptions) error {
	if opts.artifacts == "" || opts.target == "" || opts.subtarget == "" || opts.outDir == "" {
		return errors.New("缺少必需参数 (--artifacts, --target, --subtarget, --out)")
	}

	src := fetch.Source{
		Artifacts: config.Artifacts(opts.artifacts),
		Version:   opts.openwrtVersion,
		Target:    opts.target,
		Subtarget: opts.subtarget,
		RepoBase:  opts.repoBase,
		Line:      opts.line,
	}

	client := &http.Client{Timeout: 15 * time.Minute}
	loc, err := fetch.Locate(fetch.HTTPGetter{Client: client}, kind, src)
	if err != nil {
		return err
	}

	fmt.Fprintf(app.Stdout, "目标文件: %s\n", loc.ArchiveName)
	fmt.Fprintf(app.Stdout, "校验哈希: %s\n", loc.ExpectedHash)
	if err := fetch.DownloadVerifyExtract(client, loc, opts.outDir); err != nil {
		return err
	}

	if kind == fetch.IB {
		vermagic := loc.Vermagic
		if vermagic == "" {
			// official 线：vermagic 抠自解压后的 IB repositories 文件。
			vermagic = fetch.VermagicFromExtractedIB(opts.outDir)
		}
		if vermagic == "" {
			return errors.New("无法确定 vermagic")
		}
		fmt.Fprintf(app.Stdout, "vermagic: %s\n", vermagic)
		writeGitHubOutput(app, "vermagic", vermagic)
	}

	fmt.Fprintln(app.Stdout, "完成！")
	return nil
}

// writeGitHubOutput 把一个 key=value 追加进 $GITHUB_OUTPUT；不在 CI 里（变量为空）
// 时静默跳过。这是 wrt 与 workflow 之间唯一的输出契约——不再走 stdout 抠日志。
func writeGitHubOutput(app *App, key, value string) {
	outF := os.Getenv("GITHUB_OUTPUT")
	if outF == "" {
		return
	}
	f, err := os.OpenFile(outF, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(app.Stderr, "警告: 写 GITHUB_OUTPUT 失败: %v\n", err)
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s=%s\n", key, value)
}
