/*
 * UDM Configuration Factory
 */

package factory

import (
	"fmt"
	"os"

	"github.com/yeastengine/canard/internal/udm/logger"
	"gopkg.in/yaml.v2"
)

var UdmConfig Config

// TODO: Support configuration update from REST api
func InitConfigFactory(f string) error {
	if content, err := os.ReadFile(f); err != nil {
		return err
	} else {
		UdmConfig = Config{}

		if yamlErr := yaml.Unmarshal(content, &UdmConfig); yamlErr != nil {
			return yamlErr
		}
		if UdmConfig.Configuration.WebuiUri == "" {
			UdmConfig.Configuration.WebuiUri = "webui:9876"
		}
	}

	return nil
}

func CheckConfigVersion() error {
	currentVersion := UdmConfig.GetVersion()

	if currentVersion != UDM_EXPECTED_CONFIG_VERSION {
		return fmt.Errorf("config version is [%s], but expected is [%s].",
			currentVersion, UDM_EXPECTED_CONFIG_VERSION)
	}

	logger.CfgLog.Infof("config version [%s]", currentVersion)

	return nil
}
