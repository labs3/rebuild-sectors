package utils

import (
	"fmt"

	"github.com/containerd/cgroups"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const CGROUP_GROUP_NAME string = "rebuild-worker"

func subpath(name string) cgroups.Path {
	subname := fmt.Sprintf("%s/%s", CGROUP_GROUP_NAME, name)
	return cgroups.StaticPath(subname)
}
func NewCgroup(name, cpuset, memnode string) error {
	path := subpath(name)
	strpath, _ := path(cgroups.Cpuset)
	res := specs.LinuxResources{
		CPU: &specs.LinuxCPU{
			Cpus: cpuset,
			Mems: memnode,
		},
	}

	control := cgroups.NewCpuset("/sys/fs/cgroup")
	err := control.Create(strpath, &res)
	if err != nil {
		return err
	}
	return nil
}

func AddTaskToCgroup(name string, pid int) error {
	path := subpath(name)
	cg, err := cgroups.Load(cgroups.V1, path)
	if err != nil {
		return err
	}

	err = cg.AddTask(cgroups.Process{Pid: pid}, cgroups.Cpuset)
	return err
}

func DestoryCgroup(name string) error {
	path := subpath(name)
	cg, err := cgroups.Load(cgroups.V1, path)
	if err != nil {
		return err
	}

	return cg.Delete()
}
