package engine

import (
	"github.com/nconghau/MiniDBGo/internal/lsm"
)

// DB Engine interface
type Engine interface {
	Put(key, value []byte) error
	Update(key, value []byte) error
	Delete(key []byte) error
	Get(key []byte) ([]byte, error)
	IterKeys() ([]string, error)
	DumpDB(path string) error
	RestoreDB(path string) error
	Compact() error

	Close() error
	GetMetrics() map[string]int64
	IterKeysWithLimit(limit int) ([]string, error)
}

// Open create LSMEngine default backend
func Open(path string) (Engine, error) {
	return lsm.OpenLSM(path)
}
