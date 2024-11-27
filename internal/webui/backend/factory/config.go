package factory

import (
	"github.com/omec-project/util/logger"
)

type Config struct {
	Configuration *Configuration `yaml:"configuration"`
	Logger        *logger.Logger `yaml:"logger"`
}

type Configuration struct {
	Mongodb *Mongodb `yaml:"mongodb"`
	CfgPort int      `yaml:"cfgport,omitempty"`
}

type Mongodb struct {
	Name           string `yaml:"name,omitempty"`
	Url            string `yaml:"url,omitempty"`
	AuthKeysDbName string `yaml:"authKeysDbName"`
	AuthUrl        string `yaml:"authUrl"`
}
