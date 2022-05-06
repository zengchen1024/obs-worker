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

type baseController struct{}

func (c baseController) reply(w http.ResponseWriter, s opstatus.Status) {
	data := []string{""}

	if code := s.Code; code != 0 {
		data[0] = fmt.Sprintf("HTTP/1.1 %d Error", code)
	} else {
		s.Code = 200
		data[0] = "HTTP/1.1 200 OK"
	}

	data = append(
		data,
		"Content-Type: text/xml",
		"Cache-Control: no-cache",
		"Connection: close",
	)

	if v, err := s.Marshal(); err == nil {
		data = append(
			data,
			fmt.Sprintf("Content-Length: %d", len(v)),
			separator+v,
		)
	}

	w.WriteHeader(s.Code)

	fmt.Fprint(w, strings.Join(data, separator))
}

type BuildController struct {
	baseController
}

func (b BuildController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	job := worker.Job{}

	jobId, err := b.extract(w, r, &job)
	if err != nil {
		b.reply(w, opstatus.Status{
			Code:    400,
			Details: err.Error(),
		})

		return
	}

	if err := job.Validate(); err != nil {
		b.reply(w, opstatus.Status{
			Code:    400,
			Details: err.Error(),
		})

		return
	}

	q := r.URL.Query()

	//q.Get("nobadhost")

	if v := q.Get("jobid"); v != "" {
		jobId = v
	}

	job.Id = jobId

	if err = job.Create(q.Get("registerserver")); err != nil {
		b.reply(w, opstatus.Status{
			Code:    500,
			Details: err.Error(),
		})

		return
	}

	b.reply(w, opstatus.Status{
		Details: "so much work, so little time...",
	})
}

func (b BuildController) extract(
	w http.ResponseWriter,
	r *http.Request,
	job *worker.Job,
) (string, error) {
	if v := r.Header.Get("expect"); strings.ToLower(v) == "100-continue" {
		fmt.Fprint(w, "HTTP/1.1 100 continue\r\n\r\n")
	}

	v := r.Header.Get("Content-Length")
	n, err := strconv.Atoi(v)
	if err != nil {
		return "", err
	}

	data, err := utils.ReadData(r.Body, "job", int64(n))
	if err != nil {
		return "", err
	}

	if err := job.Extract(data); err != nil {
		return "", err
	}

	return utils.GenMD5(data), nil
}
