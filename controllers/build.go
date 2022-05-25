package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/zengchen1024/obs-worker/sdk/opstatus"
	"github.com/zengchen1024/obs-worker/utils"
	"github.com/zengchen1024/obs-worker/worker"
)

const separator = "\r\n"

type respData interface {
	Marshal() ([]byte, error)
}

type baseController struct{}

func (c baseController) replyMsg(w http.ResponseWriter, code int, s string) {
	if code == 0 {
		code = 200
	}

	c.reply(w, code, &opstatus.Status{
		Code:    code,
		Details: s,
	})
}

func (c baseController) reply(w http.ResponseWriter, code int, r respData) {
	header := []string{""}

	if code == 0 || (code >= 200 && code < 300) {
		code = 200
		header[0] = "HTTP/1.1 200 OK"
	} else {
		header[0] = fmt.Sprintf("HTTP/1.1 %d Error", code)
	}

	header = append(
		header,
		"Content-Type: text/xml",
		"Cache-Control: no-cache",
		"Connection: close",
	)

	var data []byte
	if r != nil {
		if v, err := r.Marshal(); err == nil {
			header = append(
				header,
				fmt.Sprintf("Content-Length: %d", len(v)),
			)

			data = v
		}
	}

	header = append(header, separator)

	w.WriteHeader(code)

	fmt.Fprint(w, strings.Join(header, separator), data)
}

type BuildController struct {
	baseController
}

func (b BuildController) Build(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		b.replyMsg(w, 405, "method is not allowed")

		return
	}

	utils.LogInfo("get job")

	job := worker.Job{}

	jobId, err := b.extract(w, r, &job)
	if err != nil {
		utils.LogErr("extract job failed, err:%s", err.Error())

		b.replyMsg(w, 400, err.Error())

		return
	}

	if err := job.Validate(); err != nil {
		utils.LogErr("invalid job, err:%s", err.Error())

		b.replyMsg(w, 400, err.Error())

		return
	}

	q := r.URL.Query()

	//q.Get("nobadhost")

	if v := q.Get("jobid"); v != "" {
		jobId = v
	}

	job.Id = jobId

	if err = job.Create(q.Get("registerserver")); err != nil {
		utils.LogErr("create job failed, err:%s", err.Error())

		b.replyMsg(w, 500, err.Error())

		return
	}

	utils.LogInfo(
		"start job: %s, %s/%s/%s",
		jobId, job.Project, job.Repository, job.Package,
	)

	b.replyMsg(w, 0, "so much work, so little time...")
}

func (b BuildController) extract(
	w http.ResponseWriter,
	r *http.Request,
	job *worker.Job,
) (string, error) {
	defer r.Body.Close()

	if v := r.Header.Get("expect"); strings.ToLower(v) == "100-continue" {
		fmt.Fprint(w, "HTTP/1.1 100 continue\r\n\r\n")
	}

	v := r.Header.Get("Content-Length")
	n, err := strconv.Atoi(v)
	if err != nil {
		return "", err
	}

	data, err := utils.ReadData(r.Body, int64(n))
	if err != nil {
		return "", err
	}

	if err := job.Extract(data); err != nil {
		return "", err
	}

	return utils.GenMD5(data), nil
}
