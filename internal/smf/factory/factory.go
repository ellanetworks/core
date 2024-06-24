package factory

import (
	"sync"
)

var (
	SmfConfig         Config
	UERoutingConfig   RoutingConfig
	UpdatedSmfConfig  UpdateSmfConfig
	SmfConfigSyncLock sync.Mutex
)

func InitConfigFactory(c Config) {
	SmfConfig = c
}

func InitRoutingConfigFactory(c RoutingConfig) {
	UERoutingConfig = c
}
