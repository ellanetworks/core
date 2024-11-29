package factory

import (
	"github.com/yeastengine/config5g/proto/client"
	protos "github.com/yeastengine/config5g/proto/sdcoreConfig"
)

var UdrConfig Configuration

type UpdateDb struct {
	SmPolicyTable *SmPolicyUpdateEntry
}

type SmPolicyUpdateEntry struct {
	Snssai *protos.NSSAI
	Imsi   string
	Dnn    string
}

func InitConfigFactory(c Configuration) {
	UdrConfig = c
	commChannel := client.ConfigWatcher(UdrConfig.WebuiUri, "udr")
	ConfigUpdateDbTrigger = make(chan *UpdateDb, 10)
	go UdrConfig.updateConfig(commChannel, ConfigUpdateDbTrigger)
}
