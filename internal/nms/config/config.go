package config

import (
	"github.com/omec-project/util/logger"
)

var Config Configuration

type Configuration struct {
	Logger *logger.Logger
	// Mongodb *Mongodb
	CfgPort int
}

// type Mongodb struct {
// 	Name string
// 	Url  string
// }

func InitConfigFactory(c Configuration) {
	Config = c
}
