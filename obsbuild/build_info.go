package obsbuild

import (
	"fmt"
	"regexp"
	"strings"
)

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

func (b *BuildInfo) isFollowupMode() bool {
	return b.FollowupFile != ""
}

func (b *BuildInfo) getSrcmd5() string {
	if b.VerifyMd5 != "" {
		return b.VerifyMd5
	}

	return b.Srcmd5
}
