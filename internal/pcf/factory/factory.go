/*
 * PCF Configuration Factory
 */

package factory

import (
	"os"

	"gopkg.in/yaml.v2"
)

var PcfConfig Config

// TODO: Support configuration update from REST api
func InitConfigFactory(f string) error {
	if content, err := os.ReadFile(f); err != nil {
		return err
	} else {
		PcfConfig = Config{}

		if yamlErr := yaml.Unmarshal(content, &PcfConfig); yamlErr != nil {
			return yamlErr
		}
		if PcfConfig.Configuration.WebuiUri == "" {
			PcfConfig.Configuration.WebuiUri = "webui:9876"
		}
	}

	return nil
}
