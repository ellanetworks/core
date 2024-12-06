package factory

import (
	"github.com/omec-project/util/logger"
)

var UdrConfig Configuration

type Configuration struct {
	Logger *logger.Logger
}

func InitConfigFactory(c Configuration) {
	UdrConfig = c
}
