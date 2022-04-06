package controllers

import (
	"fmt"

	beego "github.com/beego/beego/v2/server/web"
)

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
