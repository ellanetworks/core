package factory

import (
	"github.com/omec-project/openapi/models"
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
	WebuiUri        string
	ServiceList     []Service

	// config received from RoC
	DnnList   map[string][]string // sst+sd os key
	SlicePlmn map[string]PlmnSupportItem

	PlmnList []PlmnSupportItem `yaml:"plmnList,omitempty"`
}

type Service struct {
	ServiceName string `yaml:"serviceName"`
	SuppFeat    string `yaml:"suppFeat,omitempty"`
}

type Sbi struct {
	BindingIPv4 string `yaml:"bindingIPv4,omitempty"` // IP used to run the server in the node.
	Port        int    `yaml:"port,omitempty"`
}

type PlmnSupportItem struct {
	PlmnId models.PlmnId `yaml:"plmnId"`
}
