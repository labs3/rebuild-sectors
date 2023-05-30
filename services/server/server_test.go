package server

import (
	"batch_rebuild/services"
	"batch_rebuild/utils"
	"context"
	"net/http"
	"testing"

	"github.com/filecoin-project/go-jsonrpc"
)

func TestParseRebuildSectors(t *testing.T) {
	sectors, err := ParseRebuildSectors("rebuild_sectors.json")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(sectors.Miner)
	t.Log(sectors.Sectors)
}

func TestVersion(t *testing.T) {
	ctx, cancle := context.WithCancel(context.Background())
	defer cancle()
	var res services.ServerStruct
	closer, err := jsonrpc.NewMergeClient(ctx, utils.BuildRPCURL("127.0.0.1:7878"), SERVERNAME,
		[]interface{}{&res.Internal},
		http.Header{},
	)

	if err != nil {
		t.Log(err)
		return
	}

	v := res.Version(ctx)
	t.Log(v)

	closer()
}
