package main

import (
	"batch_rebuild/services"
	"os"

	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli"
)

var log = logging.Logger("rebuild_server")

func main() {
	logging.SetAllLoggers(logging.LevelDebug)
	app := cli.NewApp()
	app.Name = "rebuild sectors"
	app.Usage = "the server of batch rebuild sectors"
	app.Version = services.VERSION
	app.Authors = []cli.Author{
		{
			Name:  "Jerry",
			Email: "1113821597@qq.com",
		},
	}
	app.EnableBashCompletion = true
	app.Commands = []cli.Command{
		RunServerCmd,
		WorkersCmd,
	}

	if err := app.Run(os.Args); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
