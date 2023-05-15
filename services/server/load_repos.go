package server

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"

	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/lotus/node/repo"
	sealing "github.com/filecoin-project/lotus/storage/pipeline"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log/v2"
)

const SectorStorePrefix = "/sectors"

var log = logging.Logger("server")

func LoadAllRepos(repoFiles []fs.DirEntry, repoPath string) []sealing.SectorInfo {
	var wg = &sync.WaitGroup{}
	var lock = sync.Mutex{}
	var sectorInfos []sealing.SectorInfo
	for _, file := range repoFiles {
		wg.Add(1)
		go func(f fs.DirEntry) {
			defer wg.Done()
			filename := f.Name()
			log.Infof("load repo: %s", filename)
			var filepath string
			if !strings.HasSuffix(repoPath, "/") {
				filepath = fmt.Sprintf("%s/%s", repoPath, filename)
			} else {
				filepath = fmt.Sprintf("%s%s", repoPath, filename)
			}
			infos, err := loadRepo(filepath)
			if err != nil {
				log.Errorf("load repo: %s, err: %s", err.Error())
				os.Exit(-1)
			}
			lock.Lock()
			sectorInfos = append(sectorInfos, infos...)
			lock.Unlock()
		}(file)
	}

	wg.Wait()

	return sectorInfos
}

type repoObj struct {
	lr  repo.LockedRepo
	mds datastore.Batching
}

func loadRepo(repofile string) ([]sealing.SectorInfo, error) {
	repoObj, err := openRepo(repofile)
	if err != nil {
		return nil, err
	}

	states := statestore.New(namespace.Wrap(repoObj.mds, datastore.NewKey(SectorStorePrefix)))
	var sectors = make([]sealing.SectorInfo, 0)
	err = states.List(&sectors)
	if err != nil {
		return nil, err
	}
	//fmt.Println(sectors[0].SectorNumber)

	//获取单个扇区信息
	/* var out sealing.SectorInfo
	err = states.Get(uint64(148)).Get(&out)
	if err != nil{
		log.Error(err)
	}
	fmt.Println(out.PreCommit1Out) */

	repoObj.lr.Close()

	return sectors, nil
}

func openRepo(targetPath string) (*repoObj, error) {
	r, err := repo.NewFS(targetPath)
	if err != nil {
		return nil, err
	}

	if err := r.Init(repo.StorageMiner); err != nil {
		return nil, err
	}

	lr, err := r.Lock(repo.StorageMiner)
	if err != nil {
		return nil, err
	}
	//defer lr.Close()

	mds, err := lr.Datastore(context.TODO(), "/metadata")
	if err != nil {
		return nil, err
	}

	return &repoObj{
		lr:  lr,
		mds: mds,
	}, nil
}
