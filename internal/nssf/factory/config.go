package factory

import (
	"sync"

	"github.com/omec-project/openapi/models"
	logger_util "github.com/omec-project/util/logger"
)

var (
	NssfConfig Configuration
	ConfigLock sync.RWMutex
)

type Configuration struct {
	Logger              *logger_util.Logger
	Subscriptions       []Subscription
	NssfName            string
	Sbi                 *Sbi
	ServiceNameList     []models.ServiceName
	AmfSetList          []AmfSetConfig
	AmfList             []AmfConfig
	TaList              []TaConfig
	MappingListFromPlmn []MappingFromPlmnConfig
}

type Sbi struct {
	BindingIPv4 string
	Port        int
}

type AmfConfig struct {
	NfId                           string
	SupportedNssaiAvailabilityData []models.SupportedNssaiAvailabilityData
}

type TaConfig struct {
	Tai                  *models.Tai
	AccessType           *models.AccessType
	SupportedSnssaiList  []models.Snssai
	RestrictedSnssaiList []models.RestrictedSnssai
}

type AmfSetConfig struct {
	AmfSetId                       string
	AmfList                        []string
	SupportedNssaiAvailabilityData []models.SupportedNssaiAvailabilityData
}

type MappingFromPlmnConfig struct {
	OperatorName    string
	HomePlmnId      *models.PlmnId
	MappingOfSnssai []models.MappingOfSnssai
}

type Subscription struct {
	SubscriptionData *models.NssfEventSubscriptionCreateData
	SubscriptionId   string
}

func InitConfigFactory(c Configuration) error {
	NssfConfig = c
	return nil
}
