package utils

import (
	"testing"
)

func TestNew(t *testing.T) {
	err := NewCgroup("pc1-pro-1", "0-2", "0")
	if err != nil {
		t.Fatal(err)
	}
}

func TestDestory(t *testing.T) {
	err := DestoryCgroup("")
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddTask(t *testing.T) {
	err := AddTaskToCgroup("pc1-pro-1", 94607)
	if err != nil {
		t.Fatal(err)
	}
}
