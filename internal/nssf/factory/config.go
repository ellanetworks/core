package factory

import (
	"sync"

	"github.com/omec-project/openapi/models"
	logger_util "github.com/omec-project/util/logger"
)

var (
	NssfConfig Config
	ConfigLock sync.RWMutex
)

type Config struct {
	Configuration *Configuration      `yaml:"configuration"`
	Logger        *logger_util.Logger `yaml:"logger"`
	Subscriptions []Subscription      `yaml:"subscriptions,omitempty"`
}

type Configuration struct {
	NssfName            string                  `yaml:"nssfName,omitempty"`
	Sbi                 *Sbi                    `yaml:"sbi"`
	ServiceNameList     []models.ServiceName    `yaml:"serviceNameList"`
	AmfSetList          []AmfSetConfig          `yaml:"amfSetList"`
	AmfList             []AmfConfig             `yaml:"amfList"`
	TaList              []TaConfig              `yaml:"taList"`
	MappingListFromPlmn []MappingFromPlmnConfig `yaml:"mappingListFromPlmn"`
}

type Sbi struct {
	BindingIPv4 string `yaml:"bindingIPv4,omitempty"` // IP used to run the server in the node.
	Port        int    `yaml:"port"`
}

type AmfConfig struct {
	NfId                           string                                  `yaml:"nfId"`
	SupportedNssaiAvailabilityData []models.SupportedNssaiAvailabilityData `yaml:"supportedNssaiAvailabilityData"`
}

type TaConfig struct {
	Tai                  *models.Tai               `yaml:"tai"`
	AccessType           *models.AccessType        `yaml:"accessType"`
	SupportedSnssaiList  []models.Snssai           `yaml:"supportedSnssaiList"`
	RestrictedSnssaiList []models.RestrictedSnssai `yaml:"restrictedSnssaiList,omitempty"`
}

type AmfSetConfig struct {
	AmfSetId                       string                                  `yaml:"amfSetId"`
	AmfList                        []string                                `yaml:"amfList,omitempty"`
	SupportedNssaiAvailabilityData []models.SupportedNssaiAvailabilityData `yaml:"supportedNssaiAvailabilityData"`
}

type MappingFromPlmnConfig struct {
	OperatorName    string                   `yaml:"operatorName,omitempty"`
	HomePlmnId      *models.PlmnId           `yaml:"homePlmnId"`
	MappingOfSnssai []models.MappingOfSnssai `yaml:"mappingOfSnssai"`
}

type Subscription struct {
	SubscriptionData *models.NssfEventSubscriptionCreateData `yaml:"subscriptionData"`
	SubscriptionId   string                                  `yaml:"subscriptionId"`
}

func InitConfigFactory(c Config) error {
	NssfConfig = c
	return nil
}
