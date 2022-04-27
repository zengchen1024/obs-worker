package build

import (
	"fmt"

	"github.com/zengchen1024/obs-worker/sdk/buildinfo"
)

type Build interface {
	Do() error
}

func NewBuild(cfg *Config, info *buildinfo.BuildInfo) (Build, error) {
	kiwiMode := getkiwimode(info)

	if kiwiMode == "" && !isFollowupMode(info) && !isDeltaMode(info) {
		return newNonModeBuild(cfg, info)
	}

	return nil, fmt.Errorf("unsupported build")
}
