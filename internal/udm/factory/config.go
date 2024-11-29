package factory

import (
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/logger"
)

var UdmConfig Configuration

func InitConfigFactory(c Configuration) {
	UdmConfig = c
}

type Configuration struct {
	Logger          *logger.Logger
	UdmName         string
	Sbi             *Sbi
	ServiceNameList []string
	UdrUri          string
	Keys            *Keys
	PlmnSupportList []models.PlmnId
	PlmnList        []PlmnSupportItem
}

type Sbi struct {
	BindingIPv4 string
	Port        int
}

type Keys struct {
	UdmProfileAHNPrivateKey string
	UdmProfileAHNPublicKey  string
	UdmProfileBHNPrivateKey string
	UdmProfileBHNPublicKey  string
}

type PlmnSupportItem struct {
	PlmnId models.PlmnId
}
