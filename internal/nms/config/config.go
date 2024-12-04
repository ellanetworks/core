package config

import (
	"github.com/omec-project/util/logger"
)

var Config Configuration

type Configuration struct {
	Logger  *logger.Logger
	CfgPort int
}

func InitConfigFactory(c Configuration) {
	Config = c
}
