package main

import (
	"batch_rebuild/services/worker"
	"fmt"
	"os"

	"bitbucket.org/avd/go-ipc/mq"
	"github.com/urfave/cli"
)
// 处理器要启动的子进程
var DoWorkCmd = cli.Command{
	Name:  "do",
	Usage: "do work",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "type",
			Usage:    "指定要执行的任务类型",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "name",
			Usage:    "该子进程监听的处理器消息名",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) (err error) {
		var (
			msg worker.IMessage
			cmq *mq.FastMq
			//ctx, cancle = context.WithCancel(context.Background())
		)
		//defer cancle()
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

		// 监听消息队列，获取任务， 任务类型要和处理器的类型一致
		// 拿到任务，判断该处理器运行数已达上限，未满则启动一个协程去做
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

			switch m.ID() {
			case worker.TaskMsgType:
				// check msg, check 磁盘
			}
		}
	},
}

func logToParent(str string) {
	os.Stdout.WriteString(str)
}
