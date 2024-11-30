package factory

import (
	"sync"
)

var (
	SmfConfig         Configuration
	UERoutingConfig   RoutingConfig
	SmfConfigSyncLock sync.Mutex
)

func InitConfigFactory(c Configuration) error {
	SmfConfig = c
	// gClient := client.ConnectToConfigServer(SmfConfig.WebuiUri, "smf")
	// commChannel := gClient.PublishOnConfigChange(false)
	// go SmfConfig.updateConfig(commChannel)

	return nil
}

func InitRoutingConfigFactory(c RoutingConfig) {
	UERoutingConfig = c
}
