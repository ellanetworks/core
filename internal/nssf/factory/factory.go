/*
 * NSSF Configuration Factory
 */

package factory

import (
	"fmt"
	"os"
	"sync"

	"github.com/yeastengine/config5g/proto/client"
	"github.com/yeastengine/ella/internal/nssf/logger"
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
		roc := os.Getenv("MANAGED_BY_CONFIG_POD")
		if roc == "true" {
			logger.CfgLog.Infoln("MANAGED_BY_CONFIG_POD is true")
			commChannel := client.ConfigWatcher(NssfConfig.Configuration.WebuiUri, "nssf")
			go NssfConfig.updateConfig(commChannel)
		} else {
			go func() {
				logger.CfgLog.Infoln("Use helm chart config ")
				ConfigPodTrigger <- true
			}()
		}
		Configured = true
	}

	return nil
}

func CheckConfigVersion() error {
	currentVersion := NssfConfig.GetVersion()

	if currentVersion != NSSF_EXPECTED_CONFIG_VERSION {
		return fmt.Errorf("config version is [%s], but expected is [%s].",
			currentVersion, NSSF_EXPECTED_CONFIG_VERSION)
	}

	logger.CfgLog.Infof("config version [%s]", currentVersion)

	return nil
}
