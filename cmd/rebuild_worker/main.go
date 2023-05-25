package main

import (
	"batch_rebuild/services"
	"os"

	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli"
)

var log = logging.Logger("rebuild_worker")

func main() {
	logging.SetAllLoggers(logging.LevelDebug)
	app := cli.NewApp()
	app.Name = "rebuild worker"
	app.Usage = "the worker of rebuild sectors"
	app.Version = services.VERSION
	app.Authors = []cli.Author{
		{
			Name:  "Jerry",
			Email: "",
		},
	}
	app.EnableBashCompletion = true
	app.Commands = []cli.Command{
		RunWorkerCmd,
		DoWorkCmd,
		// TODO 手动触发让worker获取任务
	}

	if err := app.Run(os.Args); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
