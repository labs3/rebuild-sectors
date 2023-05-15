package config

import "testing"

func TestInitWorkerConfig(t *testing.T) {
	wcfg, err := InitWorkerConfig("config.toml")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(wcfg.Processors)
	t.Log(len(wcfg.Processors["Pc1"]))
	t.Log(wcfg.Processors["Pc1"][1].MemPreferred)
}
