package factory

import (
	"github.com/omec-project/util/logger"
)

var UdrConfig Configuration

type Configuration struct {
	Logger *logger.Logger
	Sbi    *Sbi
}

type Sbi struct {
	BindingIPv4 string
	Port        int
}

func InitConfigFactory(c Configuration) {
	UdrConfig = c
}
