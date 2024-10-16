package factory

import (
	"sync"
)

var (
	NssfConfig Config
	Configured bool
	ConfigLock sync.RWMutex
)

func init() {
	Configured = false
}

func InitConfigFactory(c Config) error {
	NssfConfig = c
	Configured = true
	return nil
}
