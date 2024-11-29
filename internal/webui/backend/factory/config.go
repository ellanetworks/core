package factory

import (
	"github.com/omec-project/util/logger"
)

var WebUIConfig Configuration

type Configuration struct {
	Logger  *logger.Logger
	Mongodb *Mongodb
	CfgPort int
}

type Mongodb struct {
	Name           string
	Url            string
	AuthKeysDbName string
	AuthUrl        string
}

func InitConfigFactory(c Configuration) {
	WebUIConfig = c
}
