package worker

import (
	"github.com/zengchen1024/obs-worker/sdk/buildinfo"
)

type Job struct {
	Id        string
	NoBadHost int

	buildinfo.BuildInfo
}

func (j *Job) Validate() error {
	return instance.canBuild(&j.BuildInfo)
}

func (j *Job) Create(registerServer string) error {
	// init buildinfo

	return nil
	// return instance.createJob(registerServer, &j.BuildInfo)
}
