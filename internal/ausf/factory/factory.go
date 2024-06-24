package factory

import (
	"os"

	"gopkg.in/yaml.v2"
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
