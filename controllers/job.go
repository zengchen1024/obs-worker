package controllers

import (
	"net/http"

	"github.com/zengchen1024/obs-worker/worker"
)

func (b BuildController) jobid(r *http.Request) string {
	return r.URL.Query().Get("jobid")
}

func (b BuildController) JobInfo(w http.ResponseWriter, r *http.Request) {
	v, err := worker.GetBuildManager().GetJob(b.jobid(r))
	if err != nil {
		b.replyMsg(w, 500, err.Error())

		return
	}

	b.reply(w, 0, &v)
}

func (b BuildController) WorkerInfo(w http.ResponseWriter, r *http.Request) {
	v, err := worker.GetBuildManager().GetWorkerInfo(b.jobid(r))
	if err != nil {
		b.replyMsg(w, 500, err.Error())

		return
	}

	b.reply(w, 0, &v)
}

func (b BuildController) KillJob(w http.ResponseWriter, r *http.Request) {
	err := worker.GetBuildManager().KillJob(b.jobid(r))
	if err != nil {
		b.replyMsg(w, 500, err.Error())

		return
	}

	b.replyMsg(w, 0, "ok")
}

func (b BuildController) DiscardJob(w http.ResponseWriter, r *http.Request) {
	err := worker.GetBuildManager().DiscardJob(b.jobid(r))
	if err != nil {
		b.replyMsg(w, 500, err.Error())

		return
	}

	b.replyMsg(w, 0, "ok")
}

func (b BuildController) SetBadHostJob(w http.ResponseWriter, r *http.Request) {
	err := worker.GetBuildManager().SetBadHostJob(b.jobid(r))
	if err != nil {
		b.replyMsg(w, 500, err.Error())

		return
	}

	b.replyMsg(w, 0, "ok")
}

func (b BuildController) SetSysrqJob(w http.ResponseWriter, r *http.Request) {
	err := worker.GetBuildManager().SetSysrqJob(b.jobid(r))
	if err != nil {
		b.replyMsg(w, 500, err.Error())

		return
	}

	b.replyMsg(w, 0, "ok")
}
