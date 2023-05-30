package config

import (
	"github.com/BurntSushi/toml"
)

type WorkerConfig struct {
	Path
	Limit
	Processors map[string][]*ProcessorConfig
}

type Limit struct {
	Pc1, Pc2, Get int
}

type Path struct {
	Seal, Store string
}

type ProcessorConfig struct {
	MemPreferred string
	Cpuset       string
	Concurrent   int
	Envs         map[string]string
}

func InitWorkerConfig(path string) (wconfig *WorkerConfig, err error) {
	wconfig = &WorkerConfig{}
	_, err = toml.DecodeFile(path, &wconfig)
	return
}
