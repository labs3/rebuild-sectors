package server

import (
	"batch_rebuild/services"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/lib/rpcenc"
	"github.com/filecoin-project/lotus/storage/sealer/sealtasks"
	"github.com/gorilla/mux"
)

const VERSION = "0.0.1"
const SERVERNAME = "RebuildServer"

type Server struct {
	ctx     context.Context
	Srv     *http.Server
	wlock   sync.RWMutex
	workers map[string]*services.Worker
	slock   sync.RWMutex
	sectors map[abi.SectorNumber]*services.WorkerTask
}

func (s *Server) Version(ctx context.Context) string {
	return VERSION
}

func (s *Server) RegisterWorker(ctx context.Context, addr string, tc map[sealtasks.TaskType]*services.TaskNumConfig) error {
	s.wlock.Lock()
	defer s.wlock.Unlock()

	if _, ok := s.workers[addr]; !ok {
		api, closer, err := services.NewRemoteWorkerRPC(ctx, addr)
		if err != nil {
			return err
		}

		ver := api.Version(ctx)

		if ver != VERSION {
			return fmt.Errorf("the worker version %s is inconsistent with the service version %s", ver, VERSION)
		}

		s.workers[addr] = &services.Worker{
			Waddr:       addr,
			Enable:      true,
			TasksConfig: tc,
			WorkerRPC:   api,
			Closer:      closer,
		}
	}

	return nil
}

func (s *Server) MaskSectorStatus(ctx context.Context, sectorNum abi.SectorNumber, ttype sealtasks.TaskType, status services.JobStatus) error {
	// TODO 将status数字对应文案描述
	log.Infof("sector %d(%s) is %d!", sectorNum, ttype, services.StatusText[status])
	s.slock.Lock()
	defer s.slock.Unlock()

	task, ok := s.sectors[sectorNum]
	if !ok {
		log.Warnf("mark a not exsit sector %d", sectorNum)
		return nil
	}

	task.TaskType = ttype
	task.Status = status

	return nil
}

func (s *Server) ChangeWorkerTaskConfig(ctx context.Context) error {
	// TODO 修改worker的TasksConfig
	return nil
}

func (s *Server) ChangeRunCount(ctx context.Context, workerAddr string, ttype sealtasks.TaskType, num int) error {
	s.wlock.Lock()
	defer s.wlock.Unlock()

	for _, w := range s.workers {
		if w.Waddr == workerAddr {
			w.TasksConfig[ttype].RunCount += num
		}
	}

	return nil
}

func (s *Server) ListWorkers(ctx context.Context) []*services.Worker {
	s.wlock.Lock()
	defer s.wlock.Unlock()

	ws := make([]*services.Worker, 0, len(s.workers))
	for _, w := range s.workers {
		ws = append(ws, w)
	}
	return ws
}

// Server端指定要恢复的扇区，并与local repo校验
// CC扇区直接分配去做 / DC扇区，指定car url，分配给worker后来拉取数据
// worker在启动时会调用此接口获取任务，之后当P1做完，P1运行数减一时，则再次请求获取任务
func (s *Server) GetTask(ctx context.Context) (*services.WorkerTask, error) {
	s.slock.RLock()
	defer s.slock.RUnlock()

	for _, task := range s.sectors {
		if task.TaskType == sealtasks.TTPreCommit1 && task.Status == services.Created {
			task.Status = services.ServerAssigned
			return task, nil
		}
	}

	return nil, errors.New("no task")
}

func (s *Server) CheckWorkersHealth(ctx context.Context) {
	t := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ctx.Done():
			log.Warn("check the health of workers exited!")
			return
		case <-t.C:
			s.heatBeat(ctx)
		}
	}
}

func (s *Server) heatBeat(ctx context.Context) {
	s.wlock.Lock()
	defer s.wlock.Unlock()

	wg := &sync.WaitGroup{}
	for _, c := range s.workers {
		wg.Add(1)
		go func(w *services.Worker) {
			defer wg.Done()

			count := 0
			for count < 3 {
				ctx, cancle := context.WithTimeout(ctx, 2*time.Second)
				defer cancle()
				resp := w.WorkerRPC.Ping(ctx)
				if resp == services.PONG {
					if !w.Enable {
						w.Enable = true
					}
					return
				}

				//访问该worker失败，将该worker禁用
				w.Enable = false
				log.Errorf("Access worker %s(%dth) failed, please check it.", w.Waddr, count)
				count++
				time.Sleep(2 * time.Second)
			}

			// TODO server是否要清理该worker的记录？
		}(c)
	}

	wg.Wait()
}

func NewServerRPCClient(ctx context.Context, url string) (services.ServerAPI, jsonrpc.ClientCloser, error) {
	var res services.ServerStruct

	closer, err := jsonrpc.NewMergeClient(ctx, url, SERVERNAME,
		[]interface{}{&res.Internal},
		http.Header{},
	)
	return &res, closer, err
}

func InitRpcServer(ctx context.Context, address string, sectors map[abi.SectorNumber]*services.WorkerTask) *Server {
	srvApi := &Server{
		ctx:     ctx,
		wlock:   sync.RWMutex{},
		workers: make(map[string]*services.Worker),
		slock:   sync.RWMutex{},
		sectors: sectors,
	}
	mux := mux.NewRouter()
	_, readerServerOpt := rpcenc.ReaderParamDecoder()
	rpcServer := jsonrpc.NewServer(readerServerOpt)
	rpcServer.Register(SERVERNAME, services.PermissionedAPI(srvApi))
	mux.Handle("/rpc/v0", rpcServer)
	mux.PathPrefix("/").Handler(http.DefaultServeMux) // pprof
	ah := &auth.Handler{
		Verify: nil,
		Next:   mux.ServeHTTP,
	}
	srv := &http.Server{
		Handler: ah,
	}
	go func() {
		nl, err := net.Listen("tcp", address)
		if err != nil {
			log.Errorf("listen address %s failed: %s", address, err)
		}
		log.Infof("server listen address %s", address)
		srv.Serve(nl)
	}()

	srvApi.Srv = srv

	return srvApi
}

var _ services.ServerAPI = new(Server)
