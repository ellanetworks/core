/*
 * NSSF Configuration Factory
 */

package factory

import (
	"os"
	"sync"

	"github.com/yeastengine/config5g/proto/client"
	"gopkg.in/yaml.v2"
)

var (
	NssfConfig Config
	Configured bool
	ConfigLock sync.RWMutex
)

func init() {
	Configured = false
}

// TODO: Support configuration update from REST api
func InitConfigFactory(f string) error {
	if content, err := os.ReadFile(f); err != nil {
		return err
	} else {
		NssfConfig = Config{}

		if yamlErr := yaml.Unmarshal(content, &NssfConfig); yamlErr != nil {
			return yamlErr
		}
		if NssfConfig.Configuration.WebuiUri == "" {
			NssfConfig.Configuration.WebuiUri = "webui:9876"
		}
		commChannel := client.ConfigWatcher(NssfConfig.Configuration.WebuiUri, "nssf")
		go NssfConfig.updateConfig(commChannel)
		Configured = true
	}

	return nil
}
