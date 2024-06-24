/*
 * UDM Configuration Factory
 */

package factory

import (
	"os"

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
