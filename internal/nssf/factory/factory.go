package factory

import (
	"sync"

	"github.com/yeastengine/config5g/proto/client"
)

var (
	NssfConfig Config
	Configured bool
	ConfigLock sync.RWMutex
)

func init() {
	Configured = false
}

func InitConfigFactory(c Config) error {
	NssfConfig = c
	commChannel := client.ConfigWatcher(NssfConfig.Configuration.WebuiUri, "nssf")
	go NssfConfig.updateConfig(commChannel)
	Configured = true
	return nil
}
