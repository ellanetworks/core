/*
 * SMF Configuration Factory
 */

package factory

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"github.com/omec-project/config5g/proto/client"
	"github.com/yeastengine/ella/internal/smf/logger"
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
	if content, err := ioutil.ReadFile(f); err != nil {
		return err
	} else {
		SmfConfig = Config{}

		if yamlErr := yaml.Unmarshal(content, &SmfConfig); yamlErr != nil {
			return yamlErr
		}

		if SmfConfig.Configuration.WebuiUri == "" {
			SmfConfig.Configuration.WebuiUri = "webui:9876"
		}

		if SmfConfig.Configuration.KafkaInfo.EnableKafka == nil {
			enableKafka := true
			SmfConfig.Configuration.KafkaInfo.EnableKafka = &enableKafka
		}

		roc := os.Getenv("MANAGED_BY_CONFIG_POD")
		if roc == "true" {
			gClient := client.ConnectToConfigServer(SmfConfig.Configuration.WebuiUri)
			commChannel := gClient.PublishOnConfigChange(false)
			go SmfConfig.updateConfig(commChannel)
		}
	}

	return nil
}

func InitRoutingConfigFactory(f string) error {
	if content, err := ioutil.ReadFile(f); err != nil {
		return err
	} else {
		UERoutingConfig = RoutingConfig{}

		if yamlErr := yaml.Unmarshal(content, &UERoutingConfig); yamlErr != nil {
			return yamlErr
		}
	}

	return nil
}

func CheckConfigVersion() error {
	currentVersion := SmfConfig.GetVersion()

	if currentVersion != SMF_EXPECTED_CONFIG_VERSION {
		return fmt.Errorf("SMF config version is [%s], but expected is [%s]",
			currentVersion, SMF_EXPECTED_CONFIG_VERSION)
	}

	logger.CfgLog.Infof("SMF config version [%s]", currentVersion)

	currentVersion = UERoutingConfig.GetVersion()

	if currentVersion != UE_ROUTING_EXPECTED_CONFIG_VERSION {
		return fmt.Errorf("UE-Routing config version is [%s], but expected is [%s]",
			currentVersion, UE_ROUTING_EXPECTED_CONFIG_VERSION)
	}

	logger.CfgLog.Infof("UE-Routing config version [%s]", currentVersion)

	return nil
}
