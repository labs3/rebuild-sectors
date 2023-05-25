package worker

import (
	"batch_rebuild/config"
	"batch_rebuild/services"
	"batch_rebuild/utils"
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/filecoin-project/lotus/lib/rpcenc"
	"github.com/filecoin-project/lotus/storage/sealer/sealtasks"
	"github.com/gorilla/mux"
	logging "github.com/ipfs/go-log/v2"
)

type WorkerStatus int8

var log = logging.Logger("worker")

type LocalWorker struct {
	addr       string
	Srv        *http.Server // worker server
	tc         map[sealtasks.TaskType]*services.TaskNumConfig
	innerpipe  chan *services.WorkerTask
	gettask    chan struct{}
	SrvAPI     services.ServerAPI // rebuild server
	Config     *config.WorkerConfig
	ProLock    sync.RWMutex
	Processors []*Processor
	queueLock  sync.RWMutex
	schedQueue *requestQueue
}

func (r *LocalWorker) Version(ctx context.Context) string {
	return services.VERSION
}

func (r *LocalWorker) Ping(ctx context.Context) string {
	return services.PONG
}

// AcquireTask 获取P1任务
func (r *LocalWorker) AcquireTask(ctx context.Context, num int) {
	for i := 0; i < num; i++ {
		task, err := r.SrvAPI.GetTask(ctx)
		if err != nil {
			log.Error(err)
			continue
		}

		r.innerpipe <- task
	}
}

func (r *LocalWorker) changeServerRunCount(ctx context.Context, ttype sealtasks.TaskType, num int) error {
	return r.SrvAPI.ChangeRunCount(ctx, r.addr, ttype, num)
}

func (r *LocalWorker) Sched(ctx context.Context) {
	tick := time.NewTicker(1 * time.Minute)
	for {
		select {
		case t := <-r.innerpipe:
			r.queueLock.Lock()
			r.schedQueue.Push(t)
			r.queueLock.Unlock()
		case <-r.gettask:
			r.AcquireTask(ctx, 1)
		case <-tick.C:
			r.assignTask()
		case <-ctx.Done():
			log.Warn("worker sched exited")
			return
		}
	}
}

func (r *LocalWorker) DestroyMqs() {
	var err error
	r.ProLock.RLock()
	for _, p := range r.Processors {
		err = p.DestroyMq()
		if err != nil {
			log.Errorf("%s destory mq %s", p.Name(), err.Error())
		}
		err = utils.DestoryCgroup(p.Name())
		if err != nil {
			log.Errorf("%s destory cgroup %s", p.Name(), err.Error())
		}
	}
	r.ProLock.RUnlock()
}

func InitWorkerServer(ctx context.Context, address string, tc map[sealtasks.TaskType]*services.TaskNumConfig, cfg *config.WorkerConfig) *LocalWorker {
	workerApi := &LocalWorker{
		addr:       address,
		tc:         tc,
		innerpipe:  make(chan *services.WorkerTask, 100),
		gettask:    make(chan struct{}),
		Config:     cfg,
		ProLock:    sync.RWMutex{},
		queueLock:  sync.RWMutex{},
		schedQueue: &requestQueue{},
	}
	mux := mux.NewRouter()
	_, readerServerOpt := rpcenc.ReaderParamDecoder()
	rpcServer := jsonrpc.NewServer(readerServerOpt)
	rpcServer.Register(services.WORKERNAME, services.PermissionedWorkerAPI(workerApi))
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
			log.Errorf("listen address %s failed: %s", address, err.Error())
		}
		log.Infof("worker server listen address %s", address)
		srv.Serve(nl)
	}()

	workerApi.Srv = srv

	go workerApi.Sched(ctx)

	// 根据配置文件，创建处理器
	for tt, pro := range cfg.Processors {
		for i, procfg := range pro {
			p, err := NewProcessor(ctx, int8(i), utils.StrToTaskType(tt), *procfg, workerApi, cfg)
			if err != nil {
				log.Errorf("NewProcessor %s", err.Error())
				continue
			}
			log.Infof("create processor %s success", p.Name())
			err = p.StartChild()
			if err != nil {
				log.Errorf("StartChild %s", err.Error())
				continue
			}
			log.Infof("start processor %s success", p.Name())
			workerApi.ProLock.Lock()
			workerApi.Processors = append(workerApi.Processors, p)
			workerApi.ProLock.Unlock()
		}

	}

	return workerApi
}

var _ services.RemoteWorkerAPI = &LocalWorker{}
