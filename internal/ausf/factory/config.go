package factory

import (
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
	Sbi             *Sbi     `yaml:"sbi,omitempty"`
	ServiceNameList []string `yaml:"serviceNameList,omitempty"`
	UdmUri          string   `yaml:"udmUri,omitempty"`
	GroupId         string   `yaml:"groupId,omitempty"`
}

type Sbi struct {
	BindingIPv4 string `yaml:"bindingIPv4,omitempty"` // IP used to run the server in the node.
	Port        int    `yaml:"port,omitempty"`
}
