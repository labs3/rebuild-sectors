package main

import (
	"batch_rebuild/services/server"
	"batch_rebuild/utils"
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/filecoin-project/lotus/storage/sealer/sealtasks"
	"github.com/urfave/cli"
)

var WorkersCmd = cli.Command{
	Name:  "workers",
	Usage: "workers relative operations",
	Subcommands: []cli.Command{
		listWorkersCmd,
	},
}

var listWorkersCmd = cli.Command{
	Name:  "list",
	Usage: "list workers info",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "server",
			Usage:    "server ip:port",
			Value:    "",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx, cancle := context.WithCancel(context.Background())
		defer cancle()

		api, closer, err := server.NewServerRPCClient(ctx, utils.BuildRPCURL(cctx.String("server")))
		if err != nil {
			return err
		}
		defer closer()

		list := api.ListWorkers(ctx)
		// TODO 统计每类任务的总数
		fmt.Println("Workers List:")
		tw := tabwriter.NewWriter(os.Stdout, 2, 20, 2, ' ', 0)
		_, _ = fmt.Fprintf(tw, "Addr \t Enable \t PC1L \t PC1R \t PC2L \t PC2R \t GETL \t GETR\n")
		for _, w := range list {
			pc1L := w.TasksConfig[sealtasks.TTPreCommit1].LimitCount
			pc1R := w.TasksConfig[sealtasks.TTPreCommit1].RunCount
			pc2L := w.TasksConfig[sealtasks.TTPreCommit2].LimitCount
			pc2R := w.TasksConfig[sealtasks.TTPreCommit2].RunCount
			getL := w.TasksConfig[sealtasks.TTFetch].LimitCount
			getR := w.TasksConfig[sealtasks.TTFetch].RunCount

			fmt.Fprintf(tw, "%s \t %v \t %d \t %d \t %d \t %d \t %d \t %d\n", w.Waddr, w.Enable, pc1L, pc1R, pc2L, pc2R, getL, getR)
		}

		return tw.Flush()
	},
}
