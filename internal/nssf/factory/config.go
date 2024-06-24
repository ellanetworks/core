/*
 * NSSF Configuration Factory
 */

package factory

import (
	"strconv"

	"github.com/omec-project/openapi/models"
	logger_util "github.com/omec-project/util/logger"
	protos "github.com/yeastengine/config5g/proto/sdcoreConfig"
	"github.com/yeastengine/ella/internal/nssf/logger"
)

const (
	NSSF_EXPECTED_CONFIG_VERSION = "1.0.0"
)

type Config struct {
	Info          *Info               `yaml:"info"`
	Configuration *Configuration      `yaml:"configuration"`
	Logger        *logger_util.Logger `yaml:"logger"`
	Subscriptions []Subscription      `yaml:"subscriptions,omitempty"`
}

type Info struct {
	Version     string `yaml:"version"`
	Description string `yaml:"description,omitempty"`
}

const (
	NSSF_DEFAULT_IPV4     = "127.0.0.31"
	NSSF_DEFAULT_PORT     = "8000"
	NSSF_DEFAULT_PORT_INT = 8000
)

type Configuration struct {
	NssfName                 string                  `yaml:"nssfName,omitempty"`
	Sbi                      *Sbi                    `yaml:"sbi"`
	ServiceNameList          []models.ServiceName    `yaml:"serviceNameList"`
	NrfUri                   string                  `yaml:"nrfUri"`
	WebuiUri                 string                  `yaml:"webuiUri"`
	SupportedPlmnList        []models.PlmnId         `yaml:"supportedPlmnList,omitempty"`
	SupportedNssaiInPlmnList []SupportedNssaiInPlmn  `yaml:"supportedNssaiInPlmnList"`
	NsiList                  []NsiConfig             `yaml:"nsiList,omitempty"`
	AmfSetList               []AmfSetConfig          `yaml:"amfSetList"`
	AmfList                  []AmfConfig             `yaml:"amfList"`
	TaList                   []TaConfig              `yaml:"taList"`
	MappingListFromPlmn      []MappingFromPlmnConfig `yaml:"mappingListFromPlmn"`
}

type Sbi struct {
	// Currently only support IPv4 and thus `Ipv4Addr` field shall not be empty
	RegisterIPv4 string `yaml:"registerIPv4,omitempty"` // IP that is registered at NRF.
	// IPv6Addr string `yaml:"ipv6Addr,omitempty"`
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

type SupportedNssaiInPlmn struct {
	PlmnId              *models.PlmnId  `yaml:"plmnId"`
	SupportedSnssaiList []models.Snssai `yaml:"supportedSnssaiList"`
}

type NsiConfig struct {
	Snssai             *models.Snssai          `yaml:"snssai"`
	NsiInformationList []models.NsiInformation `yaml:"nsiInformationList"`
}

type AmfSetConfig struct {
	AmfSetId                       string                                  `yaml:"amfSetId"`
	AmfList                        []string                                `yaml:"amfList,omitempty"`
	NrfAmfSet                      string                                  `yaml:"nrfAmfSet,omitempty"`
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

var ConfigPodTrigger chan bool

func init() {
	ConfigPodTrigger = make(chan bool)
}

func (c *Config) updateConfig(commChannel chan *protos.NetworkSliceResponse) bool {
	var minConfig bool
	for rsp := range commChannel {
		logger.GrpcLog.Infoln("Received updateConfig in the nssf app : ", rsp)
		for _, ns := range rsp.NetworkSlice {
			logger.GrpcLog.Infoln("Network Slice Name ", ns.Name)
			if ns.Site != nil {
				logger.GrpcLog.Infoln("Network Slice has site name present ")
				site := ns.Site
				logger.GrpcLog.Infoln("Site name ", site.SiteName)
				if site.Plmn != nil {
					logger.GrpcLog.Infoln("Plmn mcc ", site.Plmn.Mcc)
					logger.GrpcLog.Infoln("Plmn mnc ", site.Plmn.Mnc)
					plmn := new(models.PlmnId)
					plmn.Mnc = site.Plmn.Mnc
					plmn.Mcc = site.Plmn.Mcc
					sNssaiInPlmns := SupportedNssaiInPlmn{}
					sNssaiInPlmns.PlmnId = plmn
					nssai := new(models.Snssai)
					val, err := strconv.ParseInt(ns.Nssai.Sst, 10, 64)
					if err != nil {
						logger.GrpcLog.Infoln("Error in parsing sst ", err)
					}
					nssai.Sst = int32(val)
					nssai.Sd = ns.Nssai.Sd
					logger.GrpcLog.Infoln("Slice Sst ", ns.Nssai.Sst)
					logger.GrpcLog.Infoln("Slice Sd ", ns.Nssai.Sd)
					sNssaiInPlmns.SupportedSnssaiList = append(sNssaiInPlmns.SupportedSnssaiList, *nssai)
					var found bool = false
					for _, cplmn := range NssfConfig.Configuration.SupportedPlmnList {
						if (cplmn.Mnc == plmn.Mnc) && (cplmn.Mcc == plmn.Mcc) {
							found = true
							break
						}
					}
					if !found {
						NssfConfig.Configuration.SupportedPlmnList = append(NssfConfig.Configuration.SupportedPlmnList, *plmn)
						NssfConfig.Configuration.SupportedNssaiInPlmnList = append(NssfConfig.Configuration.SupportedNssaiInPlmnList, sNssaiInPlmns)
					}
				} else {
					logger.GrpcLog.Infoln("Plmn not present in the message ")
				}
			}
		}
		if !minConfig {
			// first slice Created
			if (len(NssfConfig.Configuration.SupportedPlmnList) > 0) &&
				(len(NssfConfig.Configuration.SupportedNssaiInPlmnList) > 0) {
				minConfig = true
				ConfigPodTrigger <- true
				logger.GrpcLog.Infoln("Send config trigger to main routine")
			}
		} else {
			// all slices deleted
			if (len(NssfConfig.Configuration.SupportedPlmnList) > 0) &&
				(len(NssfConfig.Configuration.SupportedNssaiInPlmnList) > 0) {
				minConfig = false
				ConfigPodTrigger <- false
				logger.GrpcLog.Infoln("Send config trigger to main routine")
			} else {
				ConfigPodTrigger <- true
				logger.GrpcLog.Infoln("Send config trigger to main routine")
			}
		}
	}
	return true
}

func (c *Config) GetVersion() string {
	if c.Info != nil && c.Info.Version != "" {
		return c.Info.Version
	}
	return ""
}
