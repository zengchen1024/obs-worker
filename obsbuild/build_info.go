package obsbuild

import (
	"regexp"
	"strings"

	"github.com/zengchen1024/obs-worker/sdk/buildinfo"
)

type BDep = buildinfo.BDep
type BuildInfo buildinfo.BuildInfo

func (b *BuildInfo) getkiwimode() string {
	re := regexp.MustCompile("^_service:.*:")
	v := re.ReplaceAllString(b.File, "")

	if v == "fissile.yml" {
		return "fissile"
	}

	if v == "Dockerfile" {
		return "docker"
	}

	if strings.HasSuffix(v, ".kiwi") {
		if len(b.ImageType) > 0 && b.ImageType[0] == "product" {
			return "product"
		}

		return "image"
	}

	return ""
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

func (b *BuildInfo) getNotSrcBDep() (r []BDep) {
	for _, item := range b.BDeps {
		if item.RepoArch != "src" {
			r = append(r, item)
		}
	}

	return
}

func (b *BuildInfo) getSrcBDep() (r []BDep) {
	for _, item := range b.BDeps {
		if item.RepoArch == "src" {
			r = append(r, item)
		}
	}

	return
}

func (b *BuildInfo) getNotMetaBDep() (r []BDep) {
	for _, item := range b.BDeps {
		if buildinfo.IsTrue(item.NotMeta) && item.RepoArch != "src" {
			r = append(r, item)
		}
	}

	return
}

func (b *BuildInfo) getMetaBDep() (r []BDep) {
	for _, item := range b.BDeps {
		if !buildinfo.IsTrue(item.NotMeta) {
			r = append(r, item)
		}
	}

	return
}

func (b *BuildInfo) getNotInstallBDep() (r []BDep) {
	for _, item := range b.BDeps {
		if buildinfo.IsTrue(item.NoInstall) && item.RepoArch != "src" {
			r = append(r, item)
		}
	}

	return
}

func (b *BuildInfo) getAllNotInstallBDep() (r []BDep) {
	for _, item := range b.BDeps {
		if buildinfo.IsTrue(item.NoInstall) {
			r = append(r, item)
		}
	}

	return
}
