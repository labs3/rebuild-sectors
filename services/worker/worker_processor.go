package worker

import (
	"batch_rebuild/config"
	"batch_rebuild/services"
	"batch_rebuild/utils"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"bitbucket.org/avd/go-ipc/mq"
	"github.com/filecoin-project/lotus/storage/sealer/sealtasks"
)

// Processor local worker的处理器管理类。localworker有多个不同的处理器实例，每个实例对应的启动一个子进程(executer)，执行同一类任务
type Processor struct {
	Ctx      context.Context
	ID       int8
	TaskType sealtasks.TaskType
	memNode  string
	cpus     string
	cNums    int8 // 并发数
	runLock  sync.RWMutex
	runNums  int8 // 运行数
	mq       *mq.FastMq
	cmd      *exec.Cmd
	wapi     *LocalWorker
	wcfg     *config.WorkerConfig
}

func NewProcessor(ctx context.Context, ID int8, tt sealtasks.TaskType, procfg config.ProcessorConfig,
	workerApi *LocalWorker, cfg *config.WorkerConfig) (pro *Processor, err error) {
	pro = &Processor{
		Ctx:      ctx,
		ID:       ID,
		TaskType: tt,
		memNode:  procfg.MemPreferred,
		cpus:     procfg.Cpuset,
		cNums:    int8(procfg.Concurrent),
		runLock:  sync.RWMutex{},
		wapi:     workerApi,
		wcfg:     cfg,
	}

	// 创建与该处理器通信的消息队列
	pro.mq, err = mq.CreateFastMq(pro.Name(), os.O_EXCL, 0644, utils.MaxQueueSize, utils.MaxMsgSize)

	if err != nil {
		return nil, err
	}

	return
}

func (p *Processor) StartChild() error {
	name, err := os.Executable()
	if err != nil {
		return err
	}
	// 创建子进程
	cmd := exec.CommandContext(p.Ctx, name, "exec", "--ttype", string(p.TaskType), "--name", p.Name(), "--sealdir", p.wcfg.Seal, "--storedir", p.wcfg.Store)
	cmd.Stdin, cmd.Stdout = io.Pipe()
	p.cmd = cmd
	go p.readSubProOutput()
	// TODO 设置环境变量
	err = cmd.Start()
	if err != nil {
		return err
	}

	log.Debugf("%s start: %s", p.Name(), cmd.String())
	// 监听消息队列，确认子进程启动成功
	msg := NewAckMsg()
	msgBody, _ := msg.Encode()
	buf := make([]byte, len(msgBody))
	n, err := p.mq.Receive(buf)
	if err != nil {
		return err
	}
	log.Debugf("%s parent mq receive %s", p.Name(), string(buf))

	if n != len(msgBody) || !bytes.Equal(buf, msgBody) {
		return fmt.Errorf("processor %s start failed", p.Name())
	}

	// 根据配置将此子进程加入cgroup
	if err = utils.NewCgroup(p.Name(), p.cpus, p.memNode); err != nil {
		return fmt.Errorf("create cgroup %s %s", p.Name(), err.Error())
	}

	if err = utils.AddTaskToCgroup(p.Name(), cmd.Process.Pid); err != nil {
		return fmt.Errorf("add task to cgroup %s %s", p.Name(), err.Error())
	}

	go p.handleMsg()

	return nil
}

// TODO 启动的协程， 监听p.ctx的退出
func (p *Processor) readSubProOutput() {
	t := time.NewTicker(3 * time.Second)
	for {
		select {
		case <-t.C:
			{
				buf := make([]byte, 4096)
				n, err := p.cmd.Stdin.Read(buf)
				if err != nil && err != io.EOF {
					log.Errorf("%s read stdout pipe %s", p.Name(), err.Error())
					continue
				}

				if n > 0 {
					log.Infof("%s: %s", p.Name(), string(buf[:n]))
				}
			}
		case <-p.Ctx.Done():
			return
		}
	}
}

func (p *Processor) DestroyMq() error {
	return p.mq.Destroy()
}

func (p *Processor) Name() string {
	return fmt.Sprintf("%s-Pro-%d", p.TaskType.Short(), p.ID)
}

// 任务发送成功则给该处理器及服务端的统计worker信息， runcount + 1
// 运行完成，runcount - 1,
// 1. 完成的是PC1任务，则可以获取新PC1任务（通过管道gettask)， 并且将扇区P2任务发送到innerpipe
// 2. 完成的是PC2任务，则将GET任务发送到innerpipe
// 3. 完成的是GET任务，则该任务执行结束,向服务端标记该扇区已经重算完成
// 4. 任一任务失败，则发送到innerpipe，重新分配

// sendMsg 给子进程发送消息
func (p *Processor) sendMsg(msg IMessage) error {
	data, err := PackMsg(msg)
	if err != nil {
		return err
	}
	err = p.mq.Send(data)
	if err != nil {
		return err
	}

	return p.addRunCount()
}

// handleMsg 监听子进程发送的消息
func (p *Processor) handleMsg() {
	for {
		buf := make([]byte, utils.MaxMsgSize)
		_, err := p.mq.Receive(buf)
		if err != nil {
			log.Errorf("receive msg %s from %s", err, p.Name())
			continue
		}

		msg, err := UnpackMsg(buf)
		if err != nil {
			log.Errorf("unpack msg %s from %s", err, p.Name())
			continue
		}

		task, ok := msg.(*TaskMsg)
		if !ok {
			log.Errorf("message is not %s, from executor", msg.Name())
			continue
		}

		if task.WTask.Status == services.Failed {
			p.wapi.innerpipe <- task.WTask
			continue
		}

		switch task.WTask.TaskType {
		case sealtasks.TTPreCommit1:
			p.wapi.gettask <- struct{}{}
			p.wapi.innerpipe <- &services.WorkerTask{
				TaskType:   sealtasks.TTPreCommit2,
				MinerID:    task.WTask.MinerID,
				SectorNum:  task.WTask.SectorNum,
				SectorType: task.WTask.SectorType,
				LogCommR:   task.WTask.LogCommR,
				P1Out:      task.WTask.P1Out,
				Status:     services.Created,
				Priority:   TasksOrder[sealtasks.TTPreCommit2],
			}
		case sealtasks.TTPreCommit2:
			p.wapi.innerpipe <- &services.WorkerTask{
				TaskType:   sealtasks.TTFetch,
				MinerID:    task.WTask.MinerID,
				SectorNum:  task.WTask.SectorNum,
				SectorType: task.WTask.SectorType,
				Status:     services.Created,
				Priority:   TasksOrder[sealtasks.TTFetch],
			}
		case sealtasks.TTFetch:
			if err = p.wapi.markServerSectorFinished(p.Ctx, task.WTask.SectorNum); err != nil {
				log.Errorf("processor %s mark server sector %d failed: %s", p.Name(), task.WTask.SectorNum, err.Error())
			}
		}

		if err = p.reduceRunCount(); err != nil {
			log.Errorf("processor %s reduce run count failed: %s", p.Name(), err.Error())
		}
	}
}

// addRunCount 该processor和server端运行数都加一
func (p *Processor) addRunCount() error {
	p.runLock.Lock()
	defer p.runLock.Unlock()

	if p.runNums >= p.cNums {
		return fmt.Errorf("processor %s run count %d, exceed max cnums: %d", p.Name(), p.runNums, p.cNums)
	}
	p.runNums += 1

	err := p.wapi.changeServerRunCount(p.Ctx, p.TaskType, 1)

	if err != nil {
		return err
	}
	return nil
}

// reduceRunCount() 该processor和server端运行数都减一
func (p *Processor) reduceRunCount() error {
	p.runLock.Lock()
	defer p.runLock.Unlock()

	if p.runNums <= 0 {
		p.runNums = 0
		return nil
	}

	p.runNums -= 1

	err := p.wapi.changeServerRunCount(p.Ctx, p.TaskType, -1)

	if err != nil {
		return err
	}
	return nil
}

func (p *Processor) getRunCount() int8 {
	p.runLock.RLock()
	defer p.runLock.RUnlock()
	return p.runNums
}
