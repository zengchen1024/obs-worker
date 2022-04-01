package main

import (
	_ "obs-worker/routers"
	beego "github.com/beego/beego/v2/server/web"
)

func main() {
	beego.Run()
}

