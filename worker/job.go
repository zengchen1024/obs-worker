package worker

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/zengchen1024/obs-worker/build"
	"github.com/zengchen1024/obs-worker/sdk/buildinfo"
	"github.com/zengchen1024/obs-worker/sdk/workerstate"
	"github.com/zengchen1024/obs-worker/utils"
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

func (b *BuildManager) canBuild(info *buildinfo.BuildInfo) error {
	if b.cfg.LocalKiwi != "" {
		name := fmt.Sprintf("%s/%s", info.Arch, info.Job)

		if !strings.HasSuffix(info.File, ".kiwi") {
			return fmt.Errorf("bad job: %s: not a kiwi job\n", name)
		}

		if !(len(info.ImageType) > 0 && info.ImageType[0] == "product") {
			return fmt.Errorf("bad job: %s: not a kiwi product job\n", name)
		}
	}

	return nil
}

func (b *BuildManager) createJob(registerServer string, j *Job) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.state.State != workerstate.WorkerStateIdle {
		return fmt.Errorf("I am not idle!\n")
	}

	v, _ := j.Marshal()
	err := utils.WriteFile(filepath.Join(b.cfg.StateDir, "job"), v)
	if err != nil {
		return err
	}

	job, err := build.NewBuild(b.cfg, &j.BuildInfo)
	if err != nil {
		return err
	}

	if err := job.CanDo(); err != nil {
		return err
	}

	go func() {
		b.wg.Add(1)
		defer b.wg.Done()

		b.runJob(job)
	}()

	b.job = job
	b.nobadhost = j.NoBadHost

	state := &b.state
	state.State = workerstate.WorkerStateBuilding
	state.JobId = j.Id
	/*
		if j.Logidlelimit != 0 {
			state.LogIdleLimit = j.Logidlelimit
		}
		if j.LogSizeLimit != 0 {

		}
	*/

	if registerServer == "" {
		registerServer = j.RepoServer
	}
	b.sendBuildingState(registerServer)

	return nil
}

func (b *BuildManager) runJob(job build.Build) {
	err := job.Do()
	utils.LogErr("run job, err:%v", err)
	// TODO post action
}
