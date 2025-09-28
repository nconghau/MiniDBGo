package engine

import (
	"github.com/nconghau/MiniDBGo/internal/lsm"
)

// Engine là interface chung cho DB engines
type Engine interface {
	Put(key, value []byte) error
	Update(key, value []byte) error
	Delete(key []byte) error
	Get(key []byte) ([]byte, error)
	IterKeys() ([]string, error)
	DumpDB(path string) error
	RestoreDB(path string) error
	Compact() error
}

// Open tạo một LSMEngine làm default backend
func Open(path string) (Engine, error) {
	return lsm.OpenLSM(path)
}
