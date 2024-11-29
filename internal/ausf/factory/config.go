package factory

import (
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/logger"
)

var AusfConfig Configuration

func InitConfigFactory(c Configuration) {
	AusfConfig = c
}

type Configuration struct {
	Logger          *logger.Logger
	Sbi             *Sbi
	ServiceNameList []string
	UdmUri          string
	GroupId         string
	PlmnSupportList []models.PlmnId
}

type Sbi struct {
	BindingIPv4 string
	Port        int
}

type Security struct {
	IntegrityOrder []string
	CipheringOrder []string
}
