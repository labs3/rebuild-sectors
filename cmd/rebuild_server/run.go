package main

import (
	"batch_rebuild/services/server"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"syscall"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	sealing "github.com/filecoin-project/lotus/storage/pipeline"

	"github.com/urfave/cli"
)

var RunServerCmd = cli.Command{
	Name:  "run",
	Usage: "start server",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "repo",
			Usage:    "仓库所在父级路径",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "miner",
			Usage:    "指定miner号",
			Required: true,
		},
		&cli.StringFlag{
			Name:  "listen",
			Usage: "服务监听ip和port",
			Value: "0.0.0.0:7878",
		},
		/* &cli.StringFlag{
			Name:     "chainaddr",
			Usage:    "连接的同步节点地址",
			Required: true,
			EnvVar:   "CHAIN_NODE_ADDR",
		},
		&cli.StringFlag{
			Name:   "chaintoken",
			Usage:  "连接的同步节点token",
			EnvVar: "CHAIN_AUTH_TOKEN",
		}, */
		&cli.StringFlag{
			Name:     "json",
			Usage:    "扇区信息json文件路径，详情见rebuild_sectors.json模板",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
		var err error
		var repoFiles []fs.DirEntry
		var localSectors []sealing.SectorInfo
		//var onChainSectors []*miner.SectorOnChainInfo

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		minerAddr, err := address.NewFromString(cctx.String("miner"))
		if err != nil {
			return fmt.Errorf("parse miner failed %s", err.Error())
		}

		mid, err := address.IDFromAddress(minerAddr)
		if err != nil {
			return fmt.Errorf("miner address transfer to ID address failed %s", err.Error())
		}

		// 读取要rebuild的扇区信息
		rebuildSectors, err := server.ParseRebuildSectors(cctx.String("json"))
		if err != nil {
			return fmt.Errorf("parse rebuild sectors info failed %s", err.Error())
		}

		if rebuildSectors.Miner != cctx.String("miner") {
			return fmt.Errorf("parameter miner is not equal to rebuild miner")
		}

		// 加载所有仓库
		repoFiles, err = os.ReadDir(cctx.String("repo"))
		if err != nil {
			return fmt.Errorf("the repo path must specify")
		}
		localSectors = server.LoadAllRepos(repoFiles, cctx.String("repo"))
		log.Infof("total local sectors: %d", len(localSectors))

		// 获取miner所有active扇区
		/* onChainSectors, err = server.LoadOnChainSectors(ctx, cctx.String("chainaddr"), cctx.String("chaintoken"), cctx.String("miner"))
		if err != nil {
			return fmt.Errorf("loadOnChainSectors errors: %s", err.Error())
		}
		log.Infof("total onchain sectors: %d", len(onChainSectors)) */

		// check sectors and extract sector info
		sectors, err := server.CheckAndExtractInfo(abi.ActorID(mid), localSectors, rebuildSectors)
		if err != nil {
			return fmt.Errorf("checkAndExtractInfo errors: %s", err.Error())
		}

		log.Infof("total %d sectors need to rebuild, and found %d on chain, %d local repo", len(rebuildSectors.Sectors))
		// 启动rpc服务监听
		srv := server.InitRpcServer(ctx, cctx.String("listen"), sectors)

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
