package factory

import (
	"github.com/omec-project/util/logger"
)

var PcfConfig Configuration

func InitConfigFactory(c Configuration) {
	PcfConfig = c
}

type Configuration struct {
	Logger          *logger.Logger
	PcfName         string
	TimeFormat      string
	DefaultBdtRefId string
	AmfUri          string
}

type Service struct {
	ServiceName string
	SuppFeat    string
}
