package server

import (
	"batch_rebuild/services"
	"context"
	"net/http"
	"testing"

	"github.com/filecoin-project/go-jsonrpc"
)

func TestVersion(t *testing.T) {
	ctx, cancle := context.WithCancel(context.Background())
	defer cancle()
	var res services.ServerStruct
	closer, err := jsonrpc.NewMergeClient(ctx, "http://127.0.0.1:7878/rpc/v0", SERVERNAME,
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
