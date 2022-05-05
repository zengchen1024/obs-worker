package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/astaxie/beego"
	"github.com/opensourceways/community-robot-lib/interrupts"

	"github.com/zengchen1024/obs-worker/config"
	_ "github.com/zengchen1024/obs-worker/routers"
	"github.com/zengchen1024/obs-worker/utils"
	"github.com/zengchen1024/obs-worker/worker"
)

func main() {
	p := beego.AppConfig.String("build_config")

	cfg, err := config.Load(p)
	if err != nil {
		utils.LogErr("load config failed, err:%v\n", err)
		return
	}

	port, err := beego.AppConfig.Int("httpport")
	if err != nil || port == 0 {
		utils.LogErr("must set http port")
		os.Exit(0)
	}

	if err := worker.Init(&cfg.Build, port); err != nil {
		fmt.Println(err)
		return
	}

	defer worker.Exit()

	run()
}

func run() {
	defer interrupts.WaitForGracefulShutdown()

	interrupts.OnInterrupt(func() {
		shutdown()
	})

	beego.Run()
}

func shutdown() {
	utils.LogInfo("server shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := beego.BeeApp.Server.Shutdown(ctx); err != nil {
		utils.LogErr("error to shut down server, err:%v", err.Error())
	}

	cancel()
}
