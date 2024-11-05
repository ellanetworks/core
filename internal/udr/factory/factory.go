package factory

import (
	protos "github.com/yeastengine/config5g/proto/sdcoreConfig"
)

var UdrConfig Config

type SmPolicyUpdateEntry struct {
	Snssai *protos.NSSAI
	Imsi   string
	Dnn    string
}

func InitConfigFactory(c Config) {
	UdrConfig = c
}
