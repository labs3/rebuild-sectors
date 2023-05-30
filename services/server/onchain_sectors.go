package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/lotus/api/v1api"
	"github.com/filecoin-project/lotus/chain/actors/builtin/miner"
	"github.com/filecoin-project/lotus/chain/types"
)

func LoadOnChainSectors(ctx context.Context, chainAddr, authToken, miner string) ([]*miner.SectorOnChainInfo, error) {
	var apis v1api.FullNodeStruct
	headers := http.Header{"Authorization": []string{"Bearer " + authToken}}
	closer, err := jsonrpc.NewMergeClient(ctx, "http://"+chainAddr+"/rpc/v0", "Filecoin",
		[]interface{}{&apis.Internal, &apis.CommonStruct.Internal}, headers)
	if err != nil {
		return nil, fmt.Errorf("connecting with lotus failed: %s", err.Error())
	}
	defer closer()

	minerAddr, err := address.NewFromString(miner)
	if err != nil {
		return nil, err
	}
	onChainSectors, err := apis.StateMinerActiveSectors(ctx, minerAddr, types.EmptyTSK)
	if err != nil {
		return nil, err
	}
	return onChainSectors, nil
}
