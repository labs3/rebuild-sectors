package main

import (
	"batch_rebuild/services/worker"
	"context"
	"fmt"
	"os"

	"bitbucket.org/avd/go-ipc/mq"
	"github.com/filecoin-project/lotus/storage/sealer/sealtasks"
	"github.com/mitchellh/go-homedir"
	"github.com/urfave/cli"
)

// 处理器要启动的子进程
var DoWorkCmd = cli.Command{
	Name:  "exec",
	Usage: "execute work",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "ttype",
			Usage:    "指定要执行的任务类型",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "name",
			Usage:    "该子进程监听的处理器消息名",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "sealdir",
			Usage:    "封装扇区的路径",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "storedir",
			Usage:    "提供扇区时空证明的存储路径",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) (err error) {
		var (
			msg worker.IMessage
			cmq *mq.FastMq
			ctx, cancle = context.WithCancel(context.Background())
		)
		defer cancle()

		// 子进程打开对应处理器的消息队列
		cmq, err = mq.OpenFastMq(cctx.String("name"), 0)
		if err != nil {
			return
		}
		// 给父进程发送ACK消息，说明处理器启动成功
		msg = worker.NewAckMsg()
		data, _ := msg.Encode()
		err = cmq.Send(data)
		if err != nil {
			return
		}

		// 创建执行器
		sdir, err := homedir.Expand(cctx.String("sealdir"))
		if err != nil {
			return
		}
		err = os.MkdirAll(sdir, 0775)
		if err != nil {
			return
		}
		exec, err := worker.NewExecuter(ctx, sealtasks.TaskType(cctx.String("ttype")), sdir)
		if err != nil {
			return
		}

		// 监听消息队列，获取任务
		prefix := fmt.Sprintf("child processor %s", cctx.String("name"))

		for {
			buf := make([]byte, 65535)
			_, err = cmq.Receive(buf)
			if err != nil {
				logToParent(fmt.Sprintf("%s receive: %s", prefix, err.Error()))
				continue
			}

			m, err := worker.UnpackMsg(buf)
			if err != nil {
				logToParent(fmt.Sprintf("%s unpack msg: %s", prefix, err.Error()))
				continue
			}

			task, ok := m.(*worker.TaskMsg)
			if !ok {
				logToParent(fmt.Sprintf("%s receive msg: %s, expect msg: %s", prefix, m.Name(), (&worker.TaskMsg{}).Name()))
				continue
			}

			// TODO 任务类型要和处理器的类型一致

			// 启动一个协程去计算不同任务类型, 并通过mq返回结果给父进程
			go doWork(cmq, exec, task, prefix)
		}
	},
}

func logToParent(str string) {
	os.Stdout.WriteString(str)
}

func doWork(cmq *mq.FastMq, exec *worker.Executer, task *worker.TaskMsg, prefix string) (err error) {
	switch task.WTask.TaskType {
	case sealtasks.TTPreCommit1:
		task.WTask.P1Out, err = exec.ExecP1(task.WTask)
	case sealtasks.TTPreCommit2:
		task.WTask.CommROut, err = exec.ExecP2(task.WTask)
	case sealtasks.TTFetch:
		err = exec.ExecGet(task.WTask)
	}
	
	task.WTask.Status = 2
	if err != nil {
		task.WTask.Status = 3
		logToParent(fmt.Sprintf("%s calculate sector %d(%s) failed: %s", prefix, task.WTask.SectorNum, task.WTask.TaskType, err.Error()))
	}

	data, err := worker.PackMsg(task)
	if err != nil {
		// TODO 在log文件里记录子进程日志
		return
	}

	if err = cmq.Send(data); err != nil {
		// TODO 在log文件里记录子进程日志
		return
	}

	return nil
}
