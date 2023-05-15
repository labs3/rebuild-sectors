package worker

import (
	"batch_rebuild/services"
	"sort"

	"github.com/filecoin-project/lotus/storage/sealer/sealtasks"
)

type requestQueue []*services.WorkerTask

var order = map[sealtasks.TaskType]int{
	//sealtasks.TTAddPiece:   9,
	sealtasks.TTPreCommit1: 5,
	sealtasks.TTPreCommit2: 4,
	sealtasks.TTFetch:      -1,
	//sealtasks.TTFinalize:   -2, // most priority
}

func (q requestQueue) Len() int { return len(q) }

func (q requestQueue) Less(i, j int) bool {
	oneMuchLess, muchLess := q[i].TaskType.MuchLess(q[j].TaskType)
	if oneMuchLess {
		return muchLess
	}

	if q[i].Priority != q[j].Priority {
		return q[i].Priority > q[j].Priority
	}

	if q[i].TaskType != q[j].TaskType {
		return q[i].TaskType.Less(q[j].TaskType)
	}

	return q[i].SectorNum < q[j].SectorNum // optimize minerActor.NewSectors bitfield
}

func (q requestQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].Index = i
	q[j].Index = j
}

func (q *requestQueue) Push(x *services.WorkerTask) {
	n := len(*q)
	item := x
	item.Index = n
	*q = append(*q, item)
	sort.Sort(q)
}

func (q *requestQueue) Remove(i int) *services.WorkerTask {
	old := *q
	n := len(old)
	item := old[i]
	old[i] = old[n-1]
	old[n-1] = nil
	item.Index = -1
	*q = old[0 : n-1]
	sort.Sort(q)
	return item
}
