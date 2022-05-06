package main

import (
	"flag"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/opensourceways/community-robot-lib/interrupts"
	"github.com/opensourceways/community-robot-lib/logrusutil"
	"github.com/sirupsen/logrus"

	"github.com/zengchen1024/obs-worker/config"
	"github.com/zengchen1024/obs-worker/controllers"
	"github.com/zengchen1024/obs-worker/worker"
)

type options struct {
	config      string
	port        int
	gracePeriod time.Duration
}

func gatherOptions(fs *flag.FlagSet, args ...string) options {
	var o options

	fs.StringVar(
		&o.config, "config", "./conf/build_config.yaml",
		"Path to the config file.",
	)

	fs.IntVar(&o.port, "port", 8080, "http server port")

	fs.DurationVar(
		&o.gracePeriod, "grace-period", 180*time.Second,
		"On shutdown, try to handle remaining events for the specified duration.",
	)

	fs.Parse(args)

	return o
}

func main() {
	logrusutil.ComponentInit("obs-worker")

	o := gatherOptions(
		flag.NewFlagSet(os.Args[0], flag.ExitOnError),
		os.Args[1:]...,
	)

	cfg, err := config.Load(o.config)
	if err != nil {
		logrus.WithError(err).Fatal("load config failed")
	}

	if err := worker.Init(&cfg.Build, o.port); err != nil {
		logrus.WithError(err).Fatal("init worker")
	}

	defer worker.Exit()

	run(&o)
}

func run(o *options) {
	defer interrupts.WaitForGracefulShutdown()

	register()

	httpServer := &http.Server{Addr: ":" + strconv.Itoa(o.port)}

	interrupts.ListenAndServe(httpServer, o.gracePeriod)
}

func register() {
	http.Handle("/build", controllers.BuildController{})
}
