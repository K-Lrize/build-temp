package main

import (
	"errors"
	"time"

	"github.com/spf13/cobra"

	"github.com/K-Lrize/openwrt-build/internal/artifacts"
	"github.com/K-Lrize/openwrt-build/internal/config"
	"github.com/K-Lrize/openwrt-build/internal/plan"
	"github.com/K-Lrize/openwrt-build/internal/resolve"
)

// stamp 是所有小票统一的时间戳格式：UTC、RFC3339、可排序。
func stamp(now time.Time) string { return now.UTC().Format(time.RFC3339) }

func variantFingerprints(app *App, variantID string) (resolve.Variant, plan.Fingerprints, error) {
	cfg, problems, err := config.Load(app.Root)
	if err != nil {
		return resolve.Variant{}, plan.Fingerprints{}, err
	}
	if problems.HasError() {
		return resolve.Variant{}, plan.Fingerprints{}, errors.New("配置有错，先跑 wrt lint：\n" + problems.String())
	}
	v, err := resolve.One(cfg, variantID)
	if err != nil {
		return resolve.Variant{}, plan.Fingerprints{}, err
	}
	fp, err := plan.NewComputer(app.Root).For(cfg, v)
	if err != nil {
		return resolve.Variant{}, plan.Fingerprints{}, err
	}
	return v, fp, nil
}

func sourceCommit(v resolve.Variant) string {
	if v.Line.Source != nil {
		return v.Line.Source.Commit
	}
	return ""
}

func metaCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meta",
		Short: "生成发布用的元数据 JSON（build/current/manifest/latest/packages）",
	}

	cmd.AddCommand(metaReleaseCmd(app))
	cmd.AddCommand(metaPointerCmd(app))
	cmd.AddCommand(metaBuildCmd(app))
	cmd.AddCommand(metaPointerBuildCmd(app))
	cmd.AddCommand(metaPointerPackagesCmd(app))

	return cmd
}

func metaReleaseCmd(app *App) *cobra.Command {
	var releaseID, buildID, vermagic, ciURL string

	cmd := &cobra.Command{
		Use:   "release <device>@<line>",
		Short: "产固件的不可变档案 meta.json",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if releaseID == "" || vermagic == "" {
				return errors.New("--release-id 与 --vermagic 必填")
			}
			v, fp, err := variantFingerprints(app, args[0])
			if err != nil {
				return err
			}
			meta := artifacts.ReleaseMeta{
				ReleaseID:      releaseID,
				Variant:        v.ID,
				Device:         v.Device,
				Line:           v.Line.ID,
				BuildID:        buildID,
				Vermagic:       vermagic,
				UpstreamCommit: sourceCommit(v),
				Fingerprint:    fp.Variant,
				CIRunURL:       ciURL,
				CreatedAt:      stamp(time.Now()),
			}
			return emitJSON(app.Stdout, meta)
		},
	}
	cmd.Flags().StringVar(&releaseID, "release-id", "", "本次发布编号")
	cmd.Flags().StringVar(&buildID, "build-id", "", "对应的工具链构建编号（official 线留空）")
	cmd.Flags().StringVar(&vermagic, "vermagic", "", "内核 ABI 标识")
	cmd.Flags().StringVar(&ciURL, "ci-run-url", "", "本次 CI run 链接")
	return cmd
}

func metaPointerCmd(app *App) *cobra.Command {
	var releaseID string

	cmd := &cobra.Command{
		Use:   "pointer <device>@<line>",
		Short: "产固件的「当前状态」current.json",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if releaseID == "" {
				return errors.New("--release-id 必填")
			}
			_, fp, err := variantFingerprints(app, args[0])
			if err != nil {
				return err
			}
			return emitJSON(app.Stdout, artifacts.FirmwareCurrent{
				Fingerprint: fp.Variant,
				ReleaseID:   releaseID,
				UpdatedAt:   stamp(time.Now()),
			})
		},
	}
	cmd.Flags().StringVar(&releaseID, "release-id", "", "指向的发布编号")
	return cmd
}

func metaBuildCmd(app *App) *cobra.Command {
	var buildID, vermagic, kernelVersion, sdkSHA, ibSHA, ciURL string
	var line, target, subtarget, lineTree, upstreamCommit string

	cmd := &cobra.Command{
		Use:   "build [<device>@<line>]",
		Short: "产工具链的 meta.json",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if buildID == "" || vermagic == "" {
				return errors.New("--build-id 与 --vermagic 必填")
			}

			var b artifacts.BuildMeta
			now := time.Now()
			
			if line != "" && len(args) == 0 {
				b = artifacts.BuildMeta{
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
					CreatedAt:      stamp(now),
				}
			} else if line == "" && len(args) == 1 {
				v, fp, err := variantFingerprints(app, args[0])
				if err != nil {
					return err
				}
				b = artifacts.BuildMeta{
					BuildID:        buildID,
					Line:           v.Line.ID,
					Target:         v.Hardware.Target,
					Subtarget:      v.Hardware.Subtarget,
					UpstreamCommit: sourceCommit(v),
					LineTree:       fp.LineTree,
					Vermagic:       vermagic,
					KernelVersion:  kernelVersion,
					SDKSHA256:      sdkSHA,
					IBSHA256:       ibSHA,
					CIRunURL:       ciURL,
					CreatedAt:      stamp(now),
				}
			} else {
				return errors.New("用法: wrt meta build --line L ... 或 wrt meta build <device>@<line> ...")
			}
			return emitJSON(app.Stdout, b)
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

func metaPointerPackagesCmd(app *App) *cobra.Command {
	var fingerprint string

	cmd := &cobra.Command{
		Use:   "pointer-packages [<device>@<line>]",
		Short: "产自有包层的「当前状态」current.json",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			feedFP := fingerprint
			if feedFP == "" && len(args) == 1 {
				_, computed, err := variantFingerprints(app, args[0])
				if err != nil {
					return err
				}
				feedFP = computed.Feed
			} else if feedFP == "" || len(args) > 0 {
				return errors.New("用法: wrt meta pointer-packages --fingerprint <fp> 或 wrt meta pointer-packages <device>@<line>")
			}
			return emitJSON(app.Stdout, artifacts.PackagesCurrent{Fingerprint: feedFP, UpdatedAt: stamp(time.Now())})
		},
	}
	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", "plan 已算好的 packages 层指纹")
	return cmd
}
