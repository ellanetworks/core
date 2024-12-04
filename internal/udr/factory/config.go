package factory

import (
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/logger"
)

var UdrConfig Configuration

type Configuration struct {
	Logger *logger.Logger
	Sbi    *Sbi
}

type PlmnSupportItem struct {
	PlmnId     models.PlmnId
	SNssaiList []models.Snssai
}

type Sbi struct {
	BindingIPv4 string
	Port        int
}

func InitConfigFactory(c Configuration) {
	UdrConfig = c
}
