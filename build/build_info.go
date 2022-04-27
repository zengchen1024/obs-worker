package build

import (
	"regexp"
	"strings"

	"github.com/zengchen1024/obs-worker/sdk/buildinfo"
)

type BDep = buildinfo.BDep
type RepoPath = buildinfo.Path

type BuildInfo struct {
	buildinfo.BuildInfo
	BuildTime int
	BuildHost string
}

func (b *BuildInfo) getkiwimode() string {
	return getkiwimode(&b.BuildInfo)
}

func (b *BuildInfo) isDeltaMode() bool {
	return b.File == "_delta"
}

func (b *BuildInfo) isPTFMode() bool {
	return b.File == "_ptf"
}

func (b *BuildInfo) isPreInstallImage() bool {
	return b.File == "_preinstallimage"
}

func (b *BuildInfo) isFollowupMode() bool {
	return b.FollowupFile != ""
}

func (b *BuildInfo) getSrcmd5() string {
	if b.VerifyMd5 != "" {
		return b.VerifyMd5
	}

	return b.SrcMd5
}

func (b *BuildInfo) getNotSrcBDep() []*BDep {
	return b.getBdep(func(item *BDep) bool {
		return item.RepoArch != "src"
	})
}

func (b *BuildInfo) getSrcBDep() []*BDep {
	return b.getBdep(func(item *BDep) bool {
		return item.RepoArch == "src"
	})
}

func (b *BuildInfo) getNotMetaBDep() []*BDep {
	return b.getBdep(func(item *BDep) bool {
		return item.RepoArch != "src" && buildinfo.IsTrue(item.NotMeta)
	})
}

func (b *BuildInfo) getMetaBDep() []*BDep {
	return b.getBdep(func(item *BDep) bool {
		return !buildinfo.IsTrue(item.NotMeta)
	})
}

func (b *BuildInfo) getNoInstallButSrcBDep() []*BDep {
	return b.getBdep(func(item *BDep) bool {
		return item.RepoArch != "src" && buildinfo.IsTrue(item.NoInstall)
	})
}

func (b *BuildInfo) getNoInstallBDep() []*BDep {
	return b.getBdep(func(item *BDep) bool {
		return buildinfo.IsTrue(item.NoInstall)
	})
}

func (b *BuildInfo) getBdep(ok func(*BDep) bool) []*BDep {
	items := b.BDeps
	r := make([]*BDep, 0, len(items))
	for i := range items {
		if item := &items[i]; ok(item) {
			r = append(r, item)
		}
	}

	return r
}

func (b *BuildInfo) isRepoNoMeta(repo *RepoPath) bool {
	return repo.Project != b.Project ||
		repo.Repository != b.Repository ||
		b.isPreInstallImage()
}

func (b *BuildInfo) getRepoServer(repo *RepoPath) string {
	if repo.Server != "" {
		return repo.Server
	}

	return b.RepoServer
}

func (b *BuildInfo) getPrpaOfRepo(repo *RepoPath) string {
	return genPrpa(repo.Project, repo.Repository, b.Arch)
}

func (b *BuildInfo) getPrpa() string {
	return genPrpa(b.Project, b.Repository, b.Arch)
}

func getkiwimode(info *buildinfo.BuildInfo) string {
	re := regexp.MustCompile("^_service:.*:")
	v := re.ReplaceAllString(info.File, "")

	if v == "fissile.yml" {
		return "fissile"
	}

	if v == "Dockerfile" {
		return "docker"
	}

	if strings.HasSuffix(v, ".kiwi") {
		if len(info.ImageType) > 0 && info.ImageType[0] == "product" {
			return "product"
		}

		return "image"
	}

	return ""
}

func isDeltaMode(info *buildinfo.BuildInfo) bool {
	return info.File == "_delta"
}

func isPTFMode(info *buildinfo.BuildInfo) bool {
	return info.File == "_ptf"
}

func isFollowupMode(info *buildinfo.BuildInfo) bool {
	return info.FollowupFile != ""
}
