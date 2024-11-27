package factory

import (
	"github.com/omec-project/openapi/models"
	logger_util "github.com/omec-project/util/logger"
	protos "github.com/yeastengine/config5g/proto/sdcoreConfig"
	"github.com/yeastengine/ella/internal/udr/logger"
)

type Config struct {
	Configuration *Configuration      `yaml:"configuration"`
	Logger        *logger_util.Logger `yaml:"logger"`
}

type Configuration struct {
	Sbi             *Sbi              `yaml:"sbi"`
	Mongodb         *Mongodb          `yaml:"mongodb"`
	WebuiUri        string            `yaml:"webuiUri"`
	PlmnSupportList []PlmnSupportItem `yaml:"plmnSupportList,omitempty"`
}

type PlmnSupportItem struct {
	PlmnId     models.PlmnId   `yaml:"plmnId"`
	SNssaiList []models.Snssai `yaml:"snssaiList,omitempty"`
}

type Sbi struct {
	BindingIPv4 string `yaml:"bindingIPv4,omitempty"` // IP used to run the server in the node.
	Port        int    `yaml:"port"`
}

type Mongodb struct {
	Name           string `yaml:"name,omitempty"`
	Url            string `yaml:"url,omitempty"`
	AuthKeysDbName string `yaml:"authKeysDbName"`
	AuthUrl        string `yaml:"authUrl"`
}

var (
	ConfigPodTrigger      chan bool
	ConfigUpdateDbTrigger chan *UpdateDb
)

func init() {
	ConfigPodTrigger = make(chan bool)
}

func (c *Config) addSmPolicyInfo(nwSlice *protos.NetworkSlice, dbUpdateChannel chan *UpdateDb) {
	for _, devGrp := range nwSlice.DeviceGroup {
		for _, imsi := range devGrp.Imsi {
			smPolicyEntry := &SmPolicyUpdateEntry{
				Imsi:   imsi,
				Dnn:    devGrp.IpDomainDetails.DnnName,
				Snssai: nwSlice.Nssai,
			}
			dbUpdate := &UpdateDb{
				SmPolicyTable: smPolicyEntry,
			}
			dbUpdateChannel <- dbUpdate
		}
	}
}

func (c *Config) updateConfig(commChannel chan *protos.NetworkSliceResponse, dbUpdateChannel chan *UpdateDb) bool {
	var minConfig bool
	for rsp := range commChannel {
		logger.GrpcLog.Infoln("Received updateConfig in the udr app : ", rsp)
		for _, ns := range rsp.NetworkSlice {
			logger.GrpcLog.Infoln("Network Slice Name ", ns.Name)
			if ns.Site != nil {
				logger.GrpcLog.Infoln("Network Slice has site name present ")
				site := ns.Site
				logger.GrpcLog.Infoln("Site name ", site.SiteName)
				if site.Plmn != nil {
					logger.GrpcLog.Infoln("Plmn mcc ", site.Plmn.Mcc)
					plmn := PlmnSupportItem{}
					plmn.PlmnId.Mnc = site.Plmn.Mnc
					plmn.PlmnId.Mcc = site.Plmn.Mcc
					var found bool = false
					for _, cplmn := range UdrConfig.Configuration.PlmnSupportList {
						if (cplmn.PlmnId.Mnc == plmn.PlmnId.Mnc) && (cplmn.PlmnId.Mcc == plmn.PlmnId.Mcc) {
							found = true
							break
						}
					}
					if !found {
						UdrConfig.Configuration.PlmnSupportList = append(UdrConfig.Configuration.PlmnSupportList, plmn)
					}
				} else {
					logger.GrpcLog.Infoln("Plmn not present in the message ")
				}
			}
			c.addSmPolicyInfo(ns, dbUpdateChannel)
		}
		if !minConfig {
			// first slice Created
			if len(UdrConfig.Configuration.PlmnSupportList) > 0 {
				minConfig = true
				ConfigPodTrigger <- true
				logger.GrpcLog.Infoln("Send config trigger to main routine")
			}
		} else {
			// all slices deleted
			if len(UdrConfig.Configuration.PlmnSupportList) == 0 {
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
