package worker

import (
	"batch_rebuild/services"

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

		task.Status = services.WorkerAssigned
		if err = r.markServerSectorStatus(r.ctx, task.SectorNum, task.TaskType, services.WorkerAssigned); err != nil {
			log.Errorf("processor %s mark server sector %d(%s) to %d status failed: %s", 
				pro.Name(), task.SectorNum, task.TaskType, services.StatusText[services.WorkerAssigned], err.Error())
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
