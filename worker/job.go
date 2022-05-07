package worker

import (
	"github.com/zengchen1024/obs-worker/sdk/buildinfo"
)

type Job struct {
	Id        string
	NoBadHost string

	buildinfo.BuildInfo
}

func (j *Job) Validate() error {
	return instance.canBuild(&j.BuildInfo)
}

func (j *Job) Create(registerServer string) error {
	if len(j.Paths) > 0 {
		p := j.Paths[0]

		if j.Project == "" {
			j.Project = p.Project
		}

		if j.Repository == "" {
			j.Repository = p.Repository
		}

		if j.RepoServer == "" {
			j.RepoServer = p.Server
		}
	}

	return instance.createJob(registerServer, j)
}
