package routers

import (
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context/param"
)

func init() {

    beego.GlobalControllerRouter["github.com/zengchen1024/obs-worker/controllers:MainController"] = append(beego.GlobalControllerRouter["github.com/zengchen1024/obs-worker/controllers:MainController"],
        beego.ControllerComments{
            Method: "Get",
            Router: "/",
            AllowHTTPMethods: []string{"get"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/zengchen1024/obs-worker/controllers:MainController"] = append(beego.GlobalControllerRouter["github.com/zengchen1024/obs-worker/controllers:MainController"],
        beego.ControllerComments{
            Method: "NewJob",
            Router: "/",
            AllowHTTPMethods: []string{"put"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

}
