package worker

import (
	"github.com/zengchen1024/obs-worker/obsbuild"
	"github.com/zengchen1024/obs-worker/sdk/statistic"
	"github.com/zengchen1024/obs-worker/sdk/worker"
	"github.com/zengchen1024/obs-worker/utils"
)

var instance *buildManager

type buildManager struct {
	hc utils.HttpClient

	w worker.Worker

	stats statistic.BuildStatistics

	cfg *obsbuild.Config

	port int
}

func Init(cfg *obsbuild.Config, port int) error {
	b := buildManager{
		cfg:  cfg,
		port: port,
	}

	if err := b.getWorkerInfo(); err != nil {
		return err
	}

	b.sendIdleState()

	instance = &b

	return nil
}

func Exit() {
	if instance != nil {
		instance.sendExitState()
	}
}
