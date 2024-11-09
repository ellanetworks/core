package factory

import (
	"sync"
)

var (
	SmfConfig         Config
	UERoutingConfig   RoutingConfig
	SmfConfigSyncLock sync.Mutex
)

func InitConfigFactory(c Config) error {
	SmfConfig = c
	return nil
}

func InitRoutingConfigFactory(c RoutingConfig) {
	UERoutingConfig = c
}
