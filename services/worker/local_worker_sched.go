package worker

import (
	"github.com/filecoin-project/lotus/storage/sealer/sealtasks"
)

func (r *LocalWorker) assignTask() {
	r.queueLock.RLock()
	defer r.queueLock.RUnlock()

	log.Infof("start to assign task, there have %d tasks will be scheduled", r.schedQueue.Len())
	for sqi := 0; sqi < r.schedQueue.Len(); sqi++ {
		task := (*r.schedQueue)[sqi]
		log.Infof("the %dth task %+v， sector %d will be scheduled", sqi, task.TaskType, task.SectorNum)
		// 1. 判断worker该任务类型运行数是否已达上限
		totalRunCount := r.getTotalRunCount(task.TaskType)
		totalLimitCount := r.getTotalLimitCount(task.TaskType)
		if totalRunCount >= totalLimitCount {
			log.Warnf("sector %d(%s) cannot do at the moment, total_limit %d, total_run %d", task.SectorNum, task.TaskType, totalLimitCount, totalRunCount)
			continue
		}
		// 2. TODO 如果是PC1任务，判断下磁盘空间，不足指定空间，则不分配P1，给正在进行的P2留有空间，避免失败
		if task.TaskType == sealtasks.TTPreCommit1 {
			log.Debug("check space")
		}

		// 3. 选择processor
		pro := r.selectProcessor(task.TaskType)
		if pro == nil {
			log.Warnf("sector %d(%s) does not have appropriate processor", task.SectorNum, task.TaskType)
			continue
		}

		// 4. 给processor发送任务
		err := pro.sendMsg(NewTaskMsg(task))
		if err != nil {
			log.Warnf("sector %d(%s) failed to send task to the %s", task.SectorNum, task.TaskType, pro.Name())
			continue
		}

		r.schedQueue.Remove(sqi) // 将已分配的任务从任务列表删除
		sqi--
	}
}

// getTotalRunCount 获取worker上指定任务类型正在运行的数量
func (r *LocalWorker) getTotalRunCount(ttype sealtasks.TaskType) (count int8) {
	r.ProLock.RLock()
	defer r.ProLock.RUnlock()

	for _, p := range r.Processors {
		if ttype == p.TaskType {
			count += p.getRunCount()
		}
	}
	return
}

// getTotalRunCount 获取worker上指定任务类型的限制数量
func (r *LocalWorker) getTotalLimitCount(ttype sealtasks.TaskType) (count int8) {
	return int8(r.tc[ttype].LimitCount)
}

func (r *LocalWorker) selectProcessor(ttype sealtasks.TaskType) (p *Processor) {
	r.ProLock.RLock()
	defer r.ProLock.RUnlock()

	for _, p := range r.Processors {
		if ttype == p.TaskType && p.getRunCount() < p.cNums {
			return p
		}
	}
	return
}

/*
func (r *LocalWorker) tryRunP1(t *services.WorkerTask) {
	// 当前磁盘剩余量不足固定数量，则不运行，给已运行的任务留下空间

	// 判断是否有可以运行的处理器

	// 给对应处理器监听的消息队列发送任务
}

// 收到P2，先看是否可运行，不可则加入到本地P2队列，排队等待，一次只能做1/2个P2
// P2运行，交给P2处理器，结束后，向server汇报，记录待落盘总数，根据存储性能，设置最大落盘数
func (r *LocalWorker) tryRunP2(t *services.WorkerTask) {
}

// 收到落盘任务，根据服务端统计的最大落盘数 - 正在落盘数，得到可落盘数 > 0,则执行落盘
// 否则加入待落盘队列，排队
func (r *LocalWorker) tryRunGet(t *services.WorkerTask) {
}
*/
