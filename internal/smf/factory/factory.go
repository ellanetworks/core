package factory

import (
	"sync"
)

var (
	SmfConfig         Configuration
	UERoutingConfig   RoutingConfig
	SmfConfigSyncLock sync.Mutex
)

func InitConfigFactory(c Configuration) {
	SmfConfig = c
}

func InitRoutingConfigFactory(c RoutingConfig) {
	UERoutingConfig = c
}
