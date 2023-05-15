package utils

import "github.com/filecoin-project/lotus/storage/sealer/sealtasks"

const (
	// TimeFmt ..
	TimeFmt = "2006-01-02 15:04:05"
	// YearMonth ..
	YearMonth = "2006-01"
	// MaxQueueSize
	MaxQueueSize = 512
	// MaxMsgSize
	MaxMsgSize = 1024
)

func BuildRPCURL(addr string) string {
	return "http://" + addr + "/rpc/v0"
}

func StrToTaskType(tt string) (taskType sealtasks.TaskType) {
	switch tt {
	case "Pc1":
		taskType = sealtasks.TTPreCommit1
	case "Pc2":
		taskType = sealtasks.TTPreCommit2
	case "Get":
		taskType = sealtasks.TTFetch
	}

	return
}
