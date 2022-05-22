package build

import (
	"fmt"
	"os"

	"github.com/zengchen1024/obs-worker/sdk/buildinfo"
)

const (
	BuildStagePrepare   = "prepare"
	BuildStageBuilding  = "building"
	BuildStagePostBuild = "postbuild"
)

type Build interface {
	DoBuild(string) (int, error)

	GetBuildInfo() *buildinfo.BuildInfo
	Kill() error
	SetSysrq()
	AppenBuildLog(string)
	GetBuildLogFile() string
	CanDo() error
	GetBuildStage() string
}

func NewBuild(cfg *Config, info *buildinfo.BuildInfo) (Build, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	kiwiMode := getkiwimode(info)

	if kiwiMode == "" && !isFollowupMode(info) && !isDeltaMode(info) {
		return newNonModeBuild(dir, cfg, info)
	}

	return nil, fmt.Errorf("unsupported build")
}
