/*
 * SMF Configuration Factory
 */

package factory

import (
	"os"
	"sync"

	"github.com/omec-project/config5g/proto/client"
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

		gClient := client.ConnectToConfigServer(SmfConfig.Configuration.WebuiUri)
		if gClient == nil {
			panic("Failed to connect to config server")
		}
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
