package factory

import (
	"sync"

	"github.com/yeastengine/config5g/proto/client"
)

var (
	NssfConfig Config
	ConfigLock sync.RWMutex
)

func InitConfigFactory(c Config) error {
	NssfConfig = c
	commChannel := client.ConfigWatcher(NssfConfig.Configuration.WebuiUri, "nssf")
	go NssfConfig.updateConfig(commChannel)
	return nil
}
