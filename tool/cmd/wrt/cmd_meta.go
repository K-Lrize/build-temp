package main

import (
	"errors"
	"time"

	"github.com/spf13/cobra"

	"github.com/K-Lrize/openwrt-build/internal/artifacts"
	"github.com/K-Lrize/openwrt-build/internal/resolve"
)

// meta 子命令是纯序列化器：把 CI 传入的事实拼成一份发布小票 JSON。
//
// 指纹是设计上的单一事实源——一律由 `wrt plan` 在 plan 阶段算一次，经 CI 传下来
// （--fingerprint / --line-tree），meta 绝不自己重算。因此这里不 load 配置、
// 不展开 variant，每个子命令入参形态唯一，没有任何「要么给 flag 要么给 variant」
// 的分支。

// stamp 是所有小票统一的时间戳格式：UTC、RFC3339、可排序。
func stamp(now time.Time) string { return now.UTC().Format(time.RFC3339) }

func metaCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meta",
		Short: "生成发布用的元数据 JSON（纯序列化，指纹由 plan 传入）",
	}
	cmd.AddCommand(metaReleaseCmd(app))
	cmd.AddCommand(metaPointerCmd(app))
	cmd.AddCommand(metaBuildCmd(app))
	cmd.AddCommand(metaPointerBuildCmd(app))
	cmd.AddCommand(metaPointerPackagesCmd(app))
	return mustSubcommand(cmd)
}

// metaReleaseCmd 产固件的不可变档案 meta.json。device/line 由 variant id 拆出，
// 其余全部来自 flag。
func metaReleaseCmd(app *App) *cobra.Command {
	var releaseID, buildID, vermagic, upstreamCommit, fingerprint, ciURL string
	cmd := &cobra.Command{
		Use:   "release <device>@<line>",
		Short: "产固件的不可变档案 meta.json",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if releaseID == "" || vermagic == "" || fingerprint == "" {
				return errors.New("--release-id、--vermagic、--fingerprint 必填")
			}
			device, line, err := resolve.ParseID(args[0])
			if err != nil {
				return err
			}
			return emitJSON(app.Stdout, artifacts.ReleaseMeta{
				ReleaseID:      releaseID,
				Variant:        args[0],
				Device:         device,
				Line:           line,
				BuildID:        buildID,
				Vermagic:       vermagic,
				UpstreamCommit: upstreamCommit,
				Fingerprint:    fingerprint,
				CIRunURL:       ciURL,
				CreatedAt:      stamp(time.Now()),
			})
		},
	}
	cmd.Flags().StringVar(&releaseID, "release-id", "", "本次发布编号")
	cmd.Flags().StringVar(&buildID, "build-id", "", "对应的工具链构建编号（official 线留空）")
	cmd.Flags().StringVar(&vermagic, "vermagic", "", "内核 ABI 标识")
	cmd.Flags().StringVar(&upstreamCommit, "upstream-commit", "", "上游源码 commit（official 线留空）")
	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", "plan 算好的 variant 指纹")
	cmd.Flags().StringVar(&ciURL, "ci-run-url", "", "本次 CI run 链接")
	return cmd
}

// metaPointerCmd 产固件的「当前状态」current.json。它只需指纹与指向的发布号，
// device/line 已编码在对象路径里，不进 JSON。
func metaPointerCmd(app *App) *cobra.Command {
	var releaseID, fingerprint string
	cmd := &cobra.Command{
		Use:   "pointer",
		Short: "产固件的「当前状态」current.json",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if releaseID == "" || fingerprint == "" {
				return errors.New("--release-id 与 --fingerprint 必填")
			}
			return emitJSON(app.Stdout, artifacts.FirmwareCurrent{
				Fingerprint: fingerprint,
				ReleaseID:   releaseID,
				UpdatedAt:   stamp(time.Now()),
			})
		},
	}
	cmd.Flags().StringVar(&releaseID, "release-id", "", "指向的发布编号")
	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", "plan 算好的 variant 指纹")
	return cmd
}

// metaBuildCmd 产工具链的不可变溯源档案 meta.json。
func metaBuildCmd(app *App) *cobra.Command {
	var buildID, vermagic, kernelVersion, sdkSHA, ibSHA, ciURL string
	var line, target, subtarget, lineTree, upstreamCommit string
	cmd := &cobra.Command{
		Use:   "build",
		Short: "产工具链的 meta.json",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if buildID == "" || vermagic == "" || line == "" {
				return errors.New("--build-id、--vermagic、--line 必填")
			}
			return emitJSON(app.Stdout, artifacts.BuildMeta{
				BuildID:        buildID,
				Line:           line,
				Target:         target,
				Subtarget:      subtarget,
				UpstreamCommit: upstreamCommit,
				LineTree:       lineTree,
				Vermagic:       vermagic,
				KernelVersion:  kernelVersion,
				SDKSHA256:      sdkSHA,
				IBSHA256:       ibSHA,
				CIRunURL:       ciURL,
				CreatedAt:      stamp(time.Now()),
			})
		},
	}
	cmd.Flags().StringVar(&buildID, "build-id", "", "本次工具链构建编号")
	cmd.Flags().StringVar(&vermagic, "vermagic", "", "内核 ABI 标识")
	cmd.Flags().StringVar(&kernelVersion, "kernel-version", "", "内核版本")
	cmd.Flags().StringVar(&sdkSHA, "sdk-sha256", "", "SDK 归档 sha256")
	cmd.Flags().StringVar(&ibSHA, "ib-sha256", "", "ImageBuilder 归档 sha256")
	cmd.Flags().StringVar(&ciURL, "ci-run-url", "", "本次 CI run 链接")
	cmd.Flags().StringVar(&line, "line", "", "line id")
	cmd.Flags().StringVar(&target, "target", "", "目标 target")
	cmd.Flags().StringVar(&subtarget, "subtarget", "", "目标 subtarget")
	cmd.Flags().StringVar(&lineTree, "line-tree", "", "plan 算好的 line 目录树指纹")
	cmd.Flags().StringVar(&upstreamCommit, "upstream-commit", "", "上游源码 commit")
	return cmd
}

// metaPointerBuildCmd 产工具链的「当前状态」current.json。
func metaPointerBuildCmd(app *App) *cobra.Command {
	var fingerprint, buildID, vermagic, sdk, ib string
	var kmodCount int
	cmd := &cobra.Command{
		Use:   "pointer-build",
		Short: "产工具链的「当前状态」current.json",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if buildID == "" || vermagic == "" {
				return errors.New("--build-id 与 --vermagic 必填")
			}
			return emitJSON(app.Stdout, artifacts.Current{
				Fingerprint:         fingerprint,
				BuildID:             buildID,
				Vermagic:            vermagic,
				SDKArchive:          sdk,
				ImageBuilderArchive: ib,
				KmodCount:           kmodCount,
				UpdatedAt:           stamp(time.Now()),
			})
		},
	}
	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", "plan 已算好的 line 指纹")
	cmd.Flags().StringVar(&buildID, "build-id", "", "指向的构建编号")
	cmd.Flags().StringVar(&vermagic, "vermagic", "", "该构建的内核 ABI 标识")
	cmd.Flags().StringVar(&sdk, "sdk", "", "SDK 归档文件名")
	cmd.Flags().StringVar(&ib, "ib", "", "ImageBuilder 归档文件名")
	cmd.Flags().IntVar(&kmodCount, "kmod-count", 0, "本次产出的 kmod 数")
	return cmd
}

// metaPointerPackagesCmd 产自有包层的「当前状态」current.json。
func metaPointerPackagesCmd(app *App) *cobra.Command {
	var fingerprint string
	cmd := &cobra.Command{
		Use:   "pointer-packages",
		Short: "产自有包层的「当前状态」current.json",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if fingerprint == "" {
				return errors.New("--fingerprint 必填")
			}
			return emitJSON(app.Stdout, artifacts.PackagesCurrent{
				Fingerprint: fingerprint,
				UpdatedAt:   stamp(time.Now()),
			})
		},
	}
	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", "plan 已算好的 packages 层指纹")
	return cmd
}
