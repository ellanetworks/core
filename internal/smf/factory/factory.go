package factory

import (
	"sync"

	"github.com/yeastengine/config5g/proto/client"
)

var (
	SmfConfig         Config
	UERoutingConfig   RoutingConfig
	UpdatedSmfConfig  UpdateSmfConfig
	SmfConfigSyncLock sync.Mutex
)

func InitConfigFactory(c Config) error {
	SmfConfig = c
	gClient := client.ConnectToConfigServer(SmfConfig.Configuration.WebuiUri, "smf")
	commChannel := gClient.PublishOnConfigChange(false)
	go SmfConfig.updateConfig(commChannel)

	return nil
}

func InitRoutingConfigFactory(c RoutingConfig) {
	UERoutingConfig = c
}
