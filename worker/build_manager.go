package worker

import (
	"fmt"
	"os"
	"sync"

	"github.com/zengchen1024/obs-worker/build"
	"github.com/zengchen1024/obs-worker/sdk/buildinfo"
	"github.com/zengchen1024/obs-worker/sdk/worker"
	"github.com/zengchen1024/obs-worker/sdk/workerstate"
)

var instance *BuildManager

func GetBuildManager() *BuildManager {
	return instance
}

type BuildManager struct {
	cfg     *build.Config
	w       worker.Worker
	port    int
	workDir string

	lock  sync.RWMutex
	state workerstate.WorkerState
	opLog string

	job       build.Build
	nobadhost string

	wg sync.WaitGroup
}

func (b *BuildManager) GetJob(jobid string) (buildinfo.BuildInfo, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()

	info := buildinfo.BuildInfo{}
	state := b.state.State

	if err := b.checkWorkerState(jobid, false); err != nil {
		return info, err
	}

	if state == workerstate.WorkerStateBuilding {
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

func (b *BuildManager) SetSysrqJob(jobid string) error {
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
	return b.updateJob(jobid, workerstate.WorkerStateKilled, "\n\nKilled job\n")
}

func (b *BuildManager) DiscardJob(jobid string) error {
	return b.updateJob(jobid, workerstate.WorkerStateDiscarded, "\n\nDiscarded job\n")
}

func (b *BuildManager) SetBadHostJob(jobid string) error {
	return b.updateJob(jobid, workerstate.WorkerStateBadHost, "\n\nTriggered badhost state for job\n")
}

func (b *BuildManager) GetBuildLog(jobid string, callback func(string) error) error {
	b.lock.RLock()
	defer b.lock.RUnlock()

	if err := b.checkWorkerState(jobid, true); err != nil {
		return err
	}

	return callback(b.job.GetBuildLogFile())
}

func (b *BuildManager) updateJob(jobid, state, log string) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	if err := b.checkWorkerState(jobid, true); err != nil {
		return err
	}

	if err := b.job.Kill(); err != nil {
		return fmt.Errorf("could not kill job, err: %s", err.Error())
	}

	b.opLog = log
	//b.job.AppenBuildLog(log)

	b.state.State = state

	return nil
}

func (b *BuildManager) checkWorkerState(jobid string, needBuilding bool) error {
	if (jobid != "" || needBuilding) && b.state.State != workerstate.WorkerStateBuilding {
		return fmt.Errorf("not building a job")
	}

	if jobid != "" && jobid != b.state.JobId {
		return fmt.Errorf("building a different job")
	}

	return nil
}

func (b *BuildManager) wait() {
	b.wg.Wait()
}

func Init(cfg *build.Config, port int) error {
	b := BuildManager{
		cfg:  cfg,
		port: port,
	}

	if err := b.getWorkerInfo(); err != nil {
		return err
	}

	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	b.workDir = dir

	b.sendIdleState()

	b.state.State = workerstate.WorkerStateIdle

	instance = &b

	return nil
}

func Exit() {
	if instance != nil {
		instance.sendExitState()
		instance.wait()
	}
}
