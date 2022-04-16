package obsbuild

import (
	"regexp"
	"strings"
)

type BDep struct {
	Name        string
	RepoArch    string
	Project     string
	Package     string
	Srcmd5      string
	NotMeta     bool
	NoInstall   bool
	PreInstall  bool
	VMInstall   bool
	RunScripts  bool
	InstallOnly bool
}

type RepoPath struct {
	Server     string
	Project    string
	Repository string
}

type BuildInfo struct {
	Project      string
	Package      string
	Repository   string
	Arch         string
	HostArch     string
	File         string
	ImageType    []string
	FollowupFile string

	VerifyMd5 string
	Srcmd5    string

	SrcServer  string
	RepoServer string

	BDep    []BDep
	SubPack []string

	Path   []RepoPath
	Module []string

	Release   string
	DebugInfo string

	BuildTime int
	BuildHost string
	DistURL   string
}

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

	return b.Srcmd5
}

func (b *BuildInfo) getNotSrcBDep() (r []BDep) {
	for _, item := range b.BDep {
		if item.RepoArch != "src" {
			r = append(r, item)
		}
	}

	return
}

func (b *BuildInfo) getSrcBDep() (r []BDep) {
	for _, item := range b.BDep {
		if item.RepoArch == "src" {
			r = append(r, item)
		}
	}

	return
}

func (b *BuildInfo) getNotMetaBDep() (r []BDep) {
	for _, item := range b.BDep {
		if item.NotMeta && item.RepoArch != "src" {
			r = append(r, item)
		}
	}

	return
}

func (b *BuildInfo) getMetaBDep() (r []BDep) {
	for _, item := range b.BDep {
		if !item.NotMeta {
			r = append(r, item)
		}
	}

	return
}

func (b *BuildInfo) getNotInstallBDep() (r []BDep) {
	for _, item := range b.BDep {
		if item.NoInstall && item.RepoArch != "src" {
			r = append(r, item)
		}
	}

	return
}

func (b *BuildInfo) getAllNotInstallBDep() (r []BDep) {
	for _, item := range b.BDep {
		if item.NoInstall {
			r = append(r, item)
		}
	}

	return
}
