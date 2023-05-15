package main

import (
	"batch_rebuild/config"
	"batch_rebuild/services"
	"batch_rebuild/services/server"
	"batch_rebuild/services/worker"
	"batch_rebuild/utils"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/filecoin-project/lotus/storage/sealer/sealtasks"
	"github.com/urfave/cli"
)

// worker server启动后，从server端或之后从自身拿任务，放入队列排队
// worker server启动各任务对应的子进程，然后子进程等待worker server分配任务， 父子进程通过消息队列通信

var RunWorkerCmd = cli.Command{
	Name:  "run",
	Usage: "start worker",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "server",
			Usage:    "指定连接server的(ip:port)",
			Value:    "",
			Required: true,
		},
		&cli.StringFlag{
			Name:  "listen",
			Usage: "worker监听的ip和port",
			Value: "127.0.0.1:7879",
		},
		&cli.StringFlag{
			Name:     "config",
			Usage:    "config file location",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) (err error) {
		var cfg *config.WorkerConfig
		if cfg, err = config.InitWorkerConfig(cctx.String("config")); err != nil {
			return err
		}

		if cfg.Seal == "" || cfg.Store == "" {
			return fmt.Errorf("seal or store path must be set")
		}
		ctx, cancle := context.WithCancel(context.Background())
		defer cancle()
		var tc = map[sealtasks.TaskType]*services.TaskNumConfig{}
		tc[sealtasks.TTPreCommit1] = &services.TaskNumConfig{LimitCount: cfg.Pc1}
		tc[sealtasks.TTPreCommit2] = &services.TaskNumConfig{LimitCount: cfg.Pc2}
		tc[sealtasks.TTFetch] = &services.TaskNumConfig{LimitCount: cfg.Get}
		// 启动worker端rpc服务监听
		srv := worker.InitWorkerServer(ctx, cctx.String("listen"), tc, cfg)
		// 连接rebuild server 注册该worker
		api, closer, err := server.NewServerRPCClient(ctx, utils.BuildRPCURL(cctx.String("server")))
		if err != nil {
			return err
		}
		defer closer()

		err = api.RegisterWorker(ctx, utils.BuildRPCURL(cctx.String("listen")), tc)
		if err != nil {
			return err
		}

		srv.SrvAPI = api

		log.Info("start to get p1 task...")
		srv.AcquireTask(ctx, cctx.Int("p1"))

		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
		sig := <-ch
		log.Warnw("received shutdown", "signal", sig)

		log.Warn("clear all processors mq and cgroup...")
		srv.DestroyMqs()

		if err = srv.Srv.Shutdown(ctx); err != nil {
			log.Errorf("shutdown failed: %s", err)
		}
		log.Info("graceful shutdown worker successful")

		_ = log.Sync()
		return nil
	},
}
