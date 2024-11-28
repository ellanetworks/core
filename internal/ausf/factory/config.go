package factory

import (
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/logger"
)

var AusfConfig Config

type Config struct {
	Configuration *Configuration `yaml:"configuration"`
	Logger        *logger.Logger `yaml:"logger"`
}

func InitConfigFactory(c Config) {
	AusfConfig = c
}

type Configuration struct {
	Sbi             *Sbi            `yaml:"sbi,omitempty"`
	ServiceNameList []string        `yaml:"serviceNameList,omitempty"`
	UdmUri          string          `yaml:"udmUri,omitempty"`
	GroupId         string          `yaml:"groupId,omitempty"`
	PlmnSupportList []models.PlmnId `yaml:"plmnSupportList,omitempty"`
}

type Sbi struct {
	BindingIPv4 string `yaml:"bindingIPv4,omitempty"` // IP used to run the server in the node.
	Port        int    `yaml:"port,omitempty"`
}

type Security struct {
	IntegrityOrder []string `yaml:"integrityOrder,omitempty"`
	CipheringOrder []string `yaml:"cipheringOrder,omitempty"`
}
