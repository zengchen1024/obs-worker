package worker

import (
	"fmt"
	"strings"
	"sync"

	"github.com/zengchen1024/obs-worker/build"
	"github.com/zengchen1024/obs-worker/sdk/buildinfo"
	"github.com/zengchen1024/obs-worker/sdk/worker"
	"github.com/zengchen1024/obs-worker/sdk/workerstate"
	"github.com/zengchen1024/obs-worker/utils"
)

var instance *BuildManager

func GetBuildManager() *BuildManager {
	return instance
}

type BuildManager struct {
	cfg  *build.Config
	hc   utils.HttpClient
	w    worker.Worker
	port int

	state workerstate.WorkerState
	job   build.Build
	lock  sync.RWMutex
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

func (b *BuildManager) createJob(registerServer string, info *buildinfo.BuildInfo) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.state.State != "idle" {
		return fmt.Errorf("I am not idle!\n")
	}

	return nil
}

func (b *BuildManager) GetJob(jobid string) (buildinfo.BuildInfo, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()

	info := buildinfo.BuildInfo{}
	state := b.state.State

	if err := b.checkWorkerState(jobid, false); err != nil {
		return info, err
	}

	if state == "building" {
		return *b.job.GetBuildInfo(), nil
	}

	info.Error = state

	return info, nil
}

func (b *BuildManager) GetWorkerInfo(jobid string) (worker.Worker, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()

	w := worker.Worker{}

	if err := b.checkWorkerState(jobid, false); err != nil {
		return w, err
	}

	w.Hostarch = b.cfg.HostArch
	w.Workerid = b.cfg.Id
	w.Port = b.port

	return w, nil
}

func (b *BuildManager) SetSysrq(jobid string) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	if err := b.checkWorkerState(jobid, true); err != nil {
		return err
	}

	b.job.SetSysrq()

	b.job.AppenBuildLog("\n\nSent sysrq $cgi->{'sysrq'} to job\n")

	return nil
}
func (b *BuildManager) KillJob(jobid string) error {
	return b.updateJob(jobid, "killed", "\n\nKilled job\n")
}

func (b *BuildManager) DiscardJob(jobid string) error {
	return b.updateJob(jobid, "discarded", "\n\nDiscarded job\n")
}

func (b *BuildManager) SetBadHostJob(jobid string) error {
	return b.updateJob(jobid, "badhost", "\n\nTriggered badhost state for job\n")
}

func (b *BuildManager) updateJob(jobid, state, log string) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	if err := b.checkWorkerState(jobid, true); err != nil {
		return err
	}

	b.job.Kill()

	b.job.AppenBuildLog(log)

	b.state.State = state

	return nil
}

func (b *BuildManager) checkWorkerState(jobid string, needBuilding bool) error {
	if (jobid != "" || needBuilding) && b.state.State != "building" {
		return fmt.Errorf("not building a job")
	}

	if jobid != "" && jobid != b.state.Jobid {
		return fmt.Errorf("building a different job")
	}

	return nil
}

func Init(cfg *build.Config, port int) error {
	b := BuildManager{
		cfg:  cfg,
		port: port,
	}

	if err := b.getWorkerInfo(); err != nil {
		return err
	}

	b.sendIdleState()

	b.state.State = "idle"

	instance = &b

	return nil
}

func Exit() {
	if instance != nil {
		instance.sendExitState()
	}
}
