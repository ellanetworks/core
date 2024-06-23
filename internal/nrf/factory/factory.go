/*
 * NRF Configuration Factory
 */

package factory

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/sirupsen/logrus"
	"github.com/yeastengine/config5g/proto/client"
	"github.com/yeastengine/ella/internal/nrf/logger"
)

var ManagedByConfigPod bool

var NrfConfig Config

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
}

// TODO: Support configuration update from REST api
func InitConfigFactory(f string) error {
	if content, err := os.ReadFile(f); err != nil {
		return err
	} else {
		NrfConfig = Config{}

		if yamlErr := yaml.Unmarshal(content, &NrfConfig); yamlErr != nil {
			return yamlErr
		}
		if NrfConfig.Configuration.WebuiUri == "" {
			NrfConfig.Configuration.WebuiUri = "webui:9876"
		}
		initLog.Infof("DefaultPlmnId Mnc %v , Mcc %v \n", NrfConfig.Configuration.DefaultPlmnId.Mnc, NrfConfig.Configuration.DefaultPlmnId.Mcc)
		roc := os.Getenv("MANAGED_BY_CONFIG_POD")
		if roc == "true" {
			initLog.Infoln("MANAGED_BY_CONFIG_POD is true")
			commChannel := client.ConfigWatcher(NrfConfig.Configuration.WebuiUri)
			ManagedByConfigPod = true
			go NrfConfig.updateConfig(commChannel)
		}
	}

	return nil
}

func CheckConfigVersion() error {
	currentVersion := NrfConfig.GetVersion()

	if currentVersion != NRF_EXPECTED_CONFIG_VERSION {
		return fmt.Errorf("config version is [%s], but expected is [%s].",
			currentVersion, NRF_EXPECTED_CONFIG_VERSION)
	}

	logger.CfgLog.Infof("config version [%s]", currentVersion)

	return nil
}
