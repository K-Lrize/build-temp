package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

type App struct {
	Root   string
	Stdout io.Writer
	Stderr io.Writer
}

func main() {
	if err := NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func NewRootCmd() *cobra.Command {
	var rootDir string
	app := &App{}

	rootCmd := &cobra.Command{
		Use:   "wrt",
		Short: "wrt —— openwrt-build 的构建编排入口",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := repositoryRoot(rootDir)
			if err != nil {
				return err
			}
			app.Root = resolved
			app.Stdout = cmd.OutOrStdout()
			app.Stderr = cmd.ErrOrStderr()
			return nil
		},
	}
	rootCmd.PersistentFlags().StringVarP(&rootDir, "C", "C", "", "仓库根目录（默认从当前目录向上找）")

	rootCmd.AddCommand(lintCmd(app))
	rootCmd.AddCommand(resolveCmd(app))
	rootCmd.AddCommand(reposCmd(app))
	rootCmd.AddCommand(planCmd(app))
	rootCmd.AddCommand(filesCmd(app))
	rootCmd.AddCommand(feedsCmd(app))
	rootCmd.AddCommand(metaCmd(app))
	rootCmd.AddCommand(publishCmd(app))
	rootCmd.AddCommand(toolchainCmd(app))
	rootCmd.AddCommand(gcCmd(app))
	rootCmd.AddCommand(idCmd(app))
	rootCmd.AddCommand(fetchCmd(app))

	return rootCmd
}

// repositoryRoot 定位仓库根。显式 -C 优先，其次 WRT_ROOT，最后从当前目录
func repositoryRoot(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	if env := os.Getenv("WRT_ROOT"); env != "" {
		return filepath.Abs(env)
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if isRepositoryRoot(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("没找到仓库根（当前目录及其各级父目录都没有 lines/ 与 sets/）；用 -C 显式指定")
		}
		dir = parent
	}
}

func isRepositoryRoot(dir string) bool {
	for _, marker := range [...]string{"lines", "sets"} {
		info, err := os.Stat(filepath.Join(dir, marker))
		if err != nil || !info.IsDir() {
			return false
		}
	}
	return true
}

func emitJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
