package worker

import (
	"batch_rebuild/services"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/storage/pipeline/lib/nullreader"
	"github.com/filecoin-project/lotus/storage/sealer/ffiwrapper"
	"github.com/filecoin-project/lotus/storage/sealer/ffiwrapper/basicfs"
	"github.com/filecoin-project/lotus/storage/sealer/sealtasks"
	"github.com/filecoin-project/lotus/storage/sealer/storiface"
	"github.com/ipfs/go-cid"
)

func Log(b []byte) (int, error) {
	return os.Stdout.Write(b)
}

type Executer struct {
	ctx    context.Context
	ttype  sealtasks.TaskType
	sealer *ffiwrapper.Sealer
}

func NewExecuter(ctx context.Context, ttype sealtasks.TaskType, sealdir string) (*Executer, error) {
	sealer, err := ffiwrapper.New(&basicfs.Provider{Root: sealdir})
	if err != nil {
		return nil, err
	}

	return &Executer{
		ctx,
		ttype,
		sealer,
	}, nil
}

func (e *Executer) AddPieceCC(si *services.WorkerTask, sid storiface.SectorRef) error {
	size, err := si.SectorType.SectorSize()
	if err != nil {
		return fmt.Errorf("get sector {%+v} type failed. err: %+v", si.SectorNum, err)
	}
	ssize := abi.PaddedPieceSize(size).Unpadded()
	piece, err := e.sealer.AddPiece(e.ctx, sid, nil, ssize, nullreader.NewNullReader(ssize))
	if err != nil {
		return err
	}

	si.Pieces = append(si.Pieces, api.SectorPiece{
		Piece:    piece,
		DealInfo: nil,
	})

	return nil
}

func (e *Executer) Precommit1(task *services.WorkerTask, sid storiface.SectorRef) (out storiface.PreCommit1Out, err error) {
	pieceInfos := make([]abi.PieceInfo, len(task.Pieces))
	for i, p := range task.Pieces {
		pieceInfos[i] = p.Piece
	}

	preCommit1, err := e.sealer.SealPreCommit1(e.ctx, sid, task.TicketValue, pieceInfos)
	if err != nil {
		return nil, fmt.Errorf("seal precommit(1) failed: %w", err)
	}

	return preCommit1, nil
}

func (e *Executer) ExecP1(task *services.WorkerTask) (out storiface.PreCommit1Out, err error) {
	sid := storiface.SectorRef{
		ID: abi.SectorID{
			Miner:  task.MinerID,
			Number: task.SectorNum,
		},
		ProofType: task.SectorType,
	}

	//log.Infof("executing add piece for sector {%+v}......\n", sectorNumber)
	err = e.AddPieceCC(task, sid)
	if err != nil {
		return nil, fmt.Errorf("sector {%+v} AddPiece failed. err: %+v", task.SectorNum, err)
	}

	// log.Infof("executing precommit1 for sector {%+v}......\n", sectorNumber)
	out, err = e.Precommit1(task, sid)
	if err != nil {
		return nil, fmt.Errorf("sector {%+v} seal pre commit(1) failed: %+v", task.SectorNum, err)
	}

	p1out, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}

	if task.LogP1Out != string(p1out) {
		return nil, fmt.Errorf("p1 calc failed, result doesn't match! expect: %s, actural: %s", task.LogP1Out, string(p1out))
	}

	return out, nil
}

func (e *Executer) ExecP2(task *services.WorkerTask) (out *cid.Cid, err error) {
	//log.Infof("executing precommit2 for sector {%+v}......\n", sectorNumber)
	sid := storiface.SectorRef{
		ID: abi.SectorID{
			Miner:  task.MinerID,
			Number: task.SectorNum,
		},
		ProofType: task.SectorType,
	}
	preCommit2, err := e.sealer.SealPreCommit2(e.ctx, sid, task.P1Out)
	if err != nil {
		return nil, err
	}

	if task.LogCommR != preCommit2.Sealed.String() {
		return nil, fmt.Errorf("p2 calc failed, result doesn't match! expect: %s, actural: %s", task.LogCommR, preCommit2.Sealed.String())
	}

	return &preCommit2.Sealed, nil
}

func (e *Executer) ExecGet(task *services.WorkerTask) (err error) {

	return nil
}
