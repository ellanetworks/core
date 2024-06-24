package factory

import (
	"os"
	"sync"

	"github.com/yeastengine/config5g/proto/client"
	"gopkg.in/yaml.v2"
)

var (
	SmfConfig         Config
	UERoutingConfig   RoutingConfig
	UpdatedSmfConfig  UpdateSmfConfig
	SmfConfigSyncLock sync.Mutex
)

// TODO: Support configuration update from REST api
func InitConfigFactory(f string) error {
	if content, err := os.ReadFile(f); err != nil {
		return err
	} else {
		SmfConfig = Config{}

		if yamlErr := yaml.Unmarshal(content, &SmfConfig); yamlErr != nil {
			return yamlErr
		}

		if SmfConfig.Configuration.WebuiUri == "" {
			SmfConfig.Configuration.WebuiUri = "webui:9876"
		}

		gClient := client.ConnectToConfigServer(SmfConfig.Configuration.WebuiUri, "smf")
		commChannel := gClient.PublishOnConfigChange(false)
		go SmfConfig.updateConfig(commChannel)
	}

	return nil
}

func InitRoutingConfigFactory(f string) error {
	if content, err := os.ReadFile(f); err != nil {
		return err
	} else {
		UERoutingConfig = RoutingConfig{}

		if yamlErr := yaml.Unmarshal(content, &UERoutingConfig); yamlErr != nil {
			return yamlErr
		}
	}

	return nil
}
