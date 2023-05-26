package services

import (
	"context"
	"net/http"
	"reflect"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/storage/sealer/sealtasks"
	"github.com/filecoin-project/lotus/storage/sealer/storiface"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
)

const (
	PermRead  auth.Permission = "read" // default
	PermWrite auth.Permission = "write"
	PermSign  auth.Permission = "sign"
	PermAdmin auth.Permission = "admin"

	WORKERNAME string = "RemoteWorker"
	VERSION    string = "0.0.1"
	PONG       string = "OK"
)

var ErrNotSupported = xerrors.New("method not supported")

var AllPermissions = []auth.Permission{PermRead, PermWrite, PermSign, PermAdmin}
var DefaultPerms = []auth.Permission{PermRead, PermWrite}

var _internalField = "Internal"

type ServerAPI interface {
	Version(context.Context) string                                                      //perm:read
	RegisterWorker(context.Context, string, map[sealtasks.TaskType]*TaskNumConfig) error //perm:write
	ListWorkers(context.Context) []*Worker                                               //perm:read
	GetTask(context.Context) (*WorkerTask, error)                                        //perm:read
	ChangeRunCount(context.Context, string, sealtasks.TaskType, int) error               //perm:write
	MaskSectorFinished(context.Context, abi.SectorNumber) error                          //perm:write
}

type RemoteWorkerAPI interface {
	Version(context.Context) string //perm:read
	Ping(context.Context) string    //perm:read
}

// GetInternalStructs extracts all pointers to 'Internal' sub-structs from the provided pointer to a proxy struct
func GetInternalStructs(in interface{}) []interface{} {
	return getInternalStructs(reflect.ValueOf(in).Elem())
}

func getInternalStructs(rv reflect.Value) []interface{} {
	var out []interface{}

	internal := rv.FieldByName(_internalField)
	ii := internal.Addr().Interface()
	out = append(out, ii)

	for i := 0; i < rv.NumField(); i++ {
		if rv.Type().Field(i).Name == _internalField {
			continue
		}

		sub := getInternalStructs(rv.Field(i))

		out = append(out, sub...)
	}

	return out
}

func permissionedProxies(in, out interface{}) {
	outs := GetInternalStructs(out)
	for _, o := range outs {
		//log.Debugf("api: %+v\n", o)
		auth.PermissionedProxy(AllPermissions, DefaultPerms, in, o)
	}
}

func PermissionedAPI(api ServerAPI) ServerAPI {
	var out ServerStruct
	permissionedProxies(api, &out)
	//log.Debugf("version: %+v", out.Version(context.Background()))
	return &out
}

func PermissionedWorkerAPI(api RemoteWorkerAPI) RemoteWorkerAPI {
	var out RemoteWorkerStruct
	permissionedProxies(api, &out)
	//log.Debugf("version: %+v", out.Version(context.Background()))
	return &out
}

// Server API
type ServerStruct struct {
	Internal struct {
		Version            func(p0 context.Context) string                                                     `perm:"read"`
		RegisterWorker     func(p0 context.Context, p1 string, p2 map[sealtasks.TaskType]*TaskNumConfig) error `perm:"write"`
		ListWorkers        func(p0 context.Context) []*Worker                                                  `perm:"read"`
		GetTask            func(p0 context.Context) (*WorkerTask, error)                                       `perm:"read"`
		ChangeRunCount     func(p0 context.Context, p1 string, p2 sealtasks.TaskType, p3 int) error            `perm:"write"`
		MaskSectorFinished func(p0 context.Context, p1 abi.SectorNumber) error                                 `perm:"write"`
	}
}

func (s *ServerStruct) Version(p0 context.Context) string {
	if s.Internal.Version == nil {
		return "no version"
	}
	return s.Internal.Version(p0)
}

func (s *ServerStruct) RegisterWorker(p0 context.Context, p1 string, p2 map[sealtasks.TaskType]*TaskNumConfig) error {
	return s.Internal.RegisterWorker(p0, p1, p2)
}

func (s *ServerStruct) ListWorkers(p0 context.Context) []*Worker {
	return s.Internal.ListWorkers(p0)
}

func (s *ServerStruct) GetTask(p0 context.Context) (*WorkerTask, error) {
	return s.Internal.GetTask(p0)
}

func (s *ServerStruct) ChangeRunCount(p0 context.Context, p1 string, p2 sealtasks.TaskType, p3 int) error {
	return s.Internal.ChangeRunCount(p0, p1, p2, p3)
}

func (s *ServerStruct) MaskSectorFinished(p0 context.Context, p1 abi.SectorNumber) error {
	return s.Internal.MaskSectorFinished(p0, p1)
}

type TaskNumConfig struct {
	LimitCount int
	RunCount   int
}

type Worker struct {
	Waddr       string
	Enable      bool
	TasksConfig map[sealtasks.TaskType]*TaskNumConfig
	WorkerRPC   RemoteWorkerAPI      `json:"-"`
	Closer      jsonrpc.ClientCloser `json:"-"`
}

type JobStatus int

const (
	Created JobStatus = iota
	Assigned
	Success
	Failed
)

type WorkerTask struct {
	// input
	TaskType    sealtasks.TaskType
	MinerID     abi.ActorID
	SectorNum   abi.SectorNumber
	SectorType  abi.RegisteredSealProof
	TicketValue abi.SealRandomness
	LogP1Out    string
	LogCommR    string

	// calculate output
	Pieces   []api.SectorPiece
	P1Out    storiface.PreCommit1Out
	CommROut *cid.Cid

	// task status 0-刚创建, 1-已分配, 2-执行成功, 3-执行失败需重试
	Status JobStatus

	Index    int
	Priority int
}

// Remote Worker API
type RemoteWorkerStruct struct {
	Internal struct {
		Version func(p0 context.Context) string `perm:"read"`
		Ping    func(p0 context.Context) string `perm:"read"`
	}
}

func NewRemoteWorkerRPC(ctx context.Context, url string) (RemoteWorkerAPI, jsonrpc.ClientCloser, error) {
	var res RemoteWorkerStruct
	//log.Debug(url)
	closer, err := jsonrpc.NewMergeClient(ctx, url, WORKERNAME,
		[]interface{}{&res.Internal},
		http.Header{},
	)
	return &res, closer, err
}

func (s *RemoteWorkerStruct) Version(p0 context.Context) string {
	if s.Internal.Version == nil {
		return "no version"
	}
	return s.Internal.Version(p0)
}

func (s *RemoteWorkerStruct) Ping(p0 context.Context) string {
	return s.Internal.Ping(p0)
}

var _ ServerAPI = new(ServerStruct)
var _ RemoteWorkerAPI = new(RemoteWorkerStruct)
