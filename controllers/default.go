package controllers

import (
	"fmt"
	"strings"

	beego "github.com/beego/beego/v2/server/web"

	"github.com/zengchen1024/obs-worker/sdk/opstatus"
	"github.com/zengchen1024/obs-worker/utils"
	"github.com/zengchen1024/obs-worker/worker"
)

const separator = "\r\n"

type MainController struct {
	beego.Controller
}

func (c *MainController) sendResponse(body interface{}, statusCode int) {
	if statusCode != 0 {
		// if success, don't set status code, otherwise the header set in this.ServeJSON
		// will not work. The reason maybe the same as above.
		c.Ctx.ResponseWriter.WriteHeader(statusCode)
	}

	c.Data["json"] = struct {
		Data interface{} `json:"data"`
	}{
		Data: body,
	}

	c.ServeJSON()
}

// @router / [get]
func (c *MainController) Get() {
	fmt.Println("--haha--")
	c.sendResponse("hello perl", 0)
}

// @router /build [put]
func (c *MainController) NewJob() {
	body := c.Ctx.Input.RequestBody

	job := worker.Job{}
	err := job.Extract(body)
	if err != nil {
		c.reply(opstatus.Status{
			Code:    400,
			Details: err.Error(),
		})

		return
	}

	if err := job.Validate(); err != nil {
		c.reply(opstatus.Status{
			Code:    400,
			Details: err.Error(),
		})

		return
	}

	noBadHost, _ := c.GetInt("nobadhost")
	job.NoBadHost = noBadHost

	jobid := c.GetString("jobid")
	if jobid == "" {
		jobid = utils.GenMD5(body)
	}
	job.Id = jobid

	if err = job.Create(c.GetString("registerserver")); err != nil {
		c.reply(opstatus.Status{
			Code:    500,
			Details: err.Error(),
		})

		return
	}

	c.reply(opstatus.Status{
		Details: "so much work, so little time...",
	})
}

func (c *MainController) reply(s opstatus.Status) {
	data := []string{""}

	code := s.Code
	if code != 0 {
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

	if code != 0 {
		c.Ctx.ResponseWriter.WriteHeader(code)
	}

	c.Ctx.WriteString(strings.Join(data, separator))
}

func (c *MainController) reply1(s string) {
	data := []string{
		"HTTP/1.1 200 OK",
		"Content-Type: text/xml",
		"Cache-Control: no-cache",
		"Connection: close",
	}

	if s != "" {
		data = append(
			data,
			fmt.Sprintf("Content-Length: %d", len(s)),
			separator+s,
		)
	}

	c.Ctx.WriteString(strings.Join(data, separator))
}

// 'PUT:/build $jobid:? buildcode:? workercode:? port:? registerserver:? nobadhost:? *:?' => \&startbuild,
