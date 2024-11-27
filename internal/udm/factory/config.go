package factory

import (
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/logger"
)

var UdmConfig Config

func InitConfigFactory(c Config) {
	UdmConfig = c
}

type Config struct {
	Configuration *Configuration `yaml:"configuration"`
	Logger        *logger.Logger `yaml:"logger"`
}

type Configuration struct {
	UdmName         string            `yaml:"udmName,omitempty"`
	Sbi             *Sbi              `yaml:"sbi,omitempty"`
	ServiceNameList []string          `yaml:"serviceNameList,omitempty"`
	UdrUri          string            `yaml:"udrUri,omitempty"`
	WebuiUri        string            `yaml:"webuiUri"`
	Keys            *Keys             `yaml:"keys,omitempty"`
	PlmnSupportList []models.PlmnId   `yaml:"plmnSupportList,omitempty"`
	PlmnList        []PlmnSupportItem `yaml:"plmnList,omitempty"`
}

type Sbi struct {
	BindingIPv4 string `yaml:"bindingIPv4,omitempty"` // IP used to run the server in the node.
	Port        int    `yaml:"port,omitempty"`
}

type Keys struct {
	UdmProfileAHNPrivateKey string `yaml:"udmProfileAHNPrivateKey,omitempty"`
	UdmProfileAHNPublicKey  string `yaml:"udmProfileAHNPublicKey,omitempty"`
	UdmProfileBHNPrivateKey string `yaml:"udmProfileBHNPrivateKey,omitempty"`
	UdmProfileBHNPublicKey  string `yaml:"udmProfileBHNPublicKey,omitempty"`
}

type PlmnSupportItem struct {
	PlmnId models.PlmnId `yaml:"plmnId"`
}
