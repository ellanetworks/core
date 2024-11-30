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
	Sbi             *Sbi
	TimeFormat      string
	DefaultBdtRefId string
	AmfUri          string
	UdrUri          string
	ServiceList     []Service
}

type Service struct {
	ServiceName string
	SuppFeat    string
}

type Sbi struct {
	BindingIPv4 string
	Port        int
}
