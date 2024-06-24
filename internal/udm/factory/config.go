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
	UdmName         string            `yaml:"udmName,omitempty"`
	Sbi             *Sbi              `yaml:"sbi,omitempty"`
	ServiceNameList []string          `yaml:"serviceNameList,omitempty"`
	NrfUri          string            `yaml:"nrfUri,omitempty"`
	WebuiUri        string            `yaml:"webuiUri"`
	Keys            *Keys             `yaml:"keys,omitempty"`
	PlmnSupportList []models.PlmnId   `yaml:"plmnSupportList,omitempty"`
	PlmnList        []PlmnSupportItem `yaml:"plmnList,omitempty"`
}

type Sbi struct {
	RegisterIPv4 string `yaml:"registerIPv4,omitempty"` // IP that is registered at NRF.
	// IPv6Addr string `yaml:"ipv6Addr,omitempty"`
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
