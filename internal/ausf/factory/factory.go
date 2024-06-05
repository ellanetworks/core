/*
 * AUSF Configuration Factory
 */

package factory

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/yeastengine/canard/internal/ausf/logger"
)

var AusfConfig Config

// TODO: Support configuration update from REST api
func InitConfigFactory(f string) error {
	if content, err := os.ReadFile(f); err != nil {
		return err
	} else {
		AusfConfig = Config{}

		if yamlErr := yaml.Unmarshal(content, &AusfConfig); yamlErr != nil {
			return yamlErr
		}
		if AusfConfig.Configuration.WebuiUri == "" {
			AusfConfig.Configuration.WebuiUri = "webui:9876"
		}
	}

	return nil
}

func CheckConfigVersion() error {
	currentVersion := AusfConfig.GetVersion()

	if currentVersion != AUSF_EXPECTED_CONFIG_VERSION {
		return fmt.Errorf("config version is [%s], but expected is [%s].",
			currentVersion, AUSF_EXPECTED_CONFIG_VERSION)
	}
	logger.CfgLog.Infof("config version [%s]", currentVersion)

	return nil
}
