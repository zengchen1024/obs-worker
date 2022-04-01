package routers

import (
	beego "github.com/beego/beego/v2/server/web"
	"obs-worker/controllers"
)

func init() {
	beego.Router("/sources", &controllers.MainController{})
}
