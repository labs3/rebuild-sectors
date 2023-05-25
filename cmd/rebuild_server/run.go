package main

import (
	"batch_rebuild/services/server"
	"context"

	//"fmt"
	//"io/fs"
	"os"
	"os/signal"
	"syscall"

	//"github.com/filecoin-project/lotus/chain/actors/builtin/miner"

	"github.com/urfave/cli"
)

var RunServerCmd = cli.Command{
	Name:  "run",
	Usage: "start server",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "repo",
			Usage:    "仓库所在父级路径",
			Value:    "",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "miner",
			Usage:    "指定miner号",
			Value:    "",
			Required: true,
		},
		&cli.StringFlag{
			Name:  "listen",
			Usage: "服务监听ip和port",
			Value: "0.0.0.0:7878",
		},
		&cli.StringFlag{
			Name:     "chainaddr",
			Usage:    "连接的同步节点地址",
			Value:    "",
			Required: true,
			EnvVar:   "CHAIN_NODE_ADDR",
		},
		&cli.StringFlag{
			Name:     "chaintoken",
			Usage:    "连接的同步节点token",
			Value:    "",
			Required: true,
			EnvVar:   "CHAIN_AUTH_TOKEN",
		},
		// TODO 指定要恢复的扇区支持1个或多个 / range / csv文件。会校验扇区号是否存在
	},
	Action: func(cctx *cli.Context) error {
		var err error
		/* var repoFiles []fs.DirEntry
		var sectorInfos []sealing.SectorInfo
		var onChainSectors []*miner.SectorOnChainInfo

		repoFiles, err = os.ReadDir(cctx.String("repo"))
		if err != nil{
			return fmt.Errorf("the repo path must specify")
		}
		// 获取miner所有active扇区
		/* onChainSectors, err = services.LoadOnChainSectors(cctx.String("chainaddr"), cctx.String("chaintoken"), cctx.String("miner"))
		if err != nil{
			return fmt.Errorf("loadOnChainSectors errors: %s", err.Error())
		}

		log.Infof("total onchain sectors: %d", len(onChainSectors))
		// 加载所有仓库
		sectorInfos = services.LoadAllRepos(repoFiles, cctx.String("repo"))
		log.Infof("total sectors: %d", len(sectorInfos)) */
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 启动rpc服务监听
		srv := server.InitRpcServer(cctx.String("listen"))

		// 检测worker状态
		go srv.CheckWorkersHealth(ctx)

		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
		sig := <-ch
		log.Warnw("received shutdown", "signal", sig)
		if err = srv.Srv.Shutdown(ctx); err != nil {
			log.Errorf("shutdown failed: %s", err)
		}
		log.Warn("Graceful shutdown successful")

		_ = log.Sync()

		return nil
	},
}
