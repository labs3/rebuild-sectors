package server

import (
	"batch_rebuild/services"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/filecoin-project/go-state-types/abi"
	sealing "github.com/filecoin-project/lotus/storage/pipeline"
	"github.com/filecoin-project/lotus/storage/sealer/sealtasks"
	"github.com/tidwall/gjson"
)

func ParseRebuildSectors(jsonFile string) (rebuildSectors *services.RebuildSectors, err error) {
	rebuildSectors = &services.RebuildSectors{}
	data, err := os.ReadFile(jsonFile)
	if err != nil && err != io.EOF {
		return
	}
	if !gjson.ValidBytes(data) {
		return nil, errors.New("read a invalid rebuild_sectors.json file")
	}

	err = json.Unmarshal([]byte(gjson.ParseBytes(data).Raw), rebuildSectors)
	if err != nil {
		return
	}

	return
}

func CheckAndExtractInfo(miner abi.ActorID, localSectors []sealing.SectorInfo, 
		rebuildSectors *services.RebuildSectors) (tasks map[abi.SectorNumber]*services.WorkerTask, err error) {
	tasks = make(map[abi.SectorNumber]*services.WorkerTask)
	temp := make(map[abi.SectorNumber]sealing.SectorInfo)
	for _, sectorInfo := range localSectors {
		temp[sectorInfo.SectorNumber] = sectorInfo
	}

	for _, rebSector := range rebuildSectors.Sectors {
		info, ok := temp[abi.SectorNumber(rebSector.SectorNumber)]
		if !ok {
			log.Warnf("want to rebuild sector %d, but it does not exist in local repos", rebSector.SectorNumber)
			continue
		}

		tasks[abi.SectorNumber(rebSector.SectorNumber)] = &services.WorkerTask{
			TaskType:    sealtasks.TTPreCommit1,
			MinerID:     miner,
			SectorNum:   abi.SectorNumber(rebSector.SectorNumber),
			SectorType:  info.SectorType,
			TicketValue: info.TicketValue,
			LogP1Out:    info.PreCommit1Out,
			LogCommR:    info.CommR,
			Deals:       rebSector.Deals,
			Priority:    services.TasksOrder[sealtasks.TTPreCommit1],
			Status:      services.Created,
		}
	}

	return
}
