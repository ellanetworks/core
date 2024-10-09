package factory

import (
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/logger"
)

type Config struct {
	Info          *Info          `yaml:"info"`
	Configuration *Configuration `yaml:"configuration"`
	Logger        *logger.Logger `yaml:"logger"`
}

type Info struct {
	Version     string `yaml:"version,omitempty"`
	Description string `yaml:"description,omitempty"`
}

type Configuration struct {
	Sbi             *Sbi            `yaml:"sbi,omitempty"`
	ServiceNameList []string        `yaml:"serviceNameList,omitempty"`
	NrfUri          string          `yaml:"nrfUri,omitempty"`
	UdmUri          string          `yaml:"udmUri,omitempty"`
	WebuiUri        string          `yaml:"webuiUri"`
	GroupId         string          `yaml:"groupId,omitempty"`
	PlmnSupportList []models.PlmnId `yaml:"plmnSupportList,omitempty"`
}

type Sbi struct {
	RegisterIPv4 string `yaml:"registerIPv4,omitempty"` // IP that is registered at NRF.
	BindingIPv4  string `yaml:"bindingIPv4,omitempty"`  // IP used to run the server in the node.
	Port         int    `yaml:"port,omitempty"`
}

type Security struct {
	IntegrityOrder []string `yaml:"integrityOrder,omitempty"`
	CipheringOrder []string `yaml:"cipheringOrder,omitempty"`
}
