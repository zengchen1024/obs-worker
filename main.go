package main

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/opensourceways/community-robot-lib/interrupts"

	"github.com/zengchen1024/obs-worker/config"
	"github.com/zengchen1024/obs-worker/controllers"
	"github.com/zengchen1024/obs-worker/utils"
	"github.com/zengchen1024/obs-worker/worker"
)

func main() {
	cfg, err := config.Load("./conf/build_config.yaml")
	if err != nil {
		utils.LogErr("load config failed, err:%v\n", err)
		return
	}

	if err := worker.Init(&cfg.Build, 8080); err != nil {
		fmt.Println(err)
		return
	}

	defer worker.Exit()

	run(8080)
}

func run(port int) {
	defer interrupts.WaitForGracefulShutdown()

	register()

	httpServer := &http.Server{Addr: ":" + strconv.Itoa(port)}

	interrupts.ListenAndServe(httpServer, 300)
}

func register() {
	http.Handle("/build", controllers.BuildController{})
}
