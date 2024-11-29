package factory

import (
	"github.com/omec-project/openapi/models"
	logger_util "github.com/omec-project/util/logger"
	protos "github.com/yeastengine/config5g/proto/sdcoreConfig"
	"github.com/yeastengine/ella/internal/udr/logger"
)

type Configuration struct {
	Logger          *logger_util.Logger
	Sbi             *Sbi
	Mongodb         *Mongodb
	WebuiUri        string
	PlmnSupportList []PlmnSupportItem
}

type PlmnSupportItem struct {
	PlmnId     models.PlmnId
	SNssaiList []models.Snssai
}

type Sbi struct {
	BindingIPv4 string
	Port        int
}

type Mongodb struct {
	Name           string
	Url            string
	AuthKeysDbName string
	AuthUrl        string
}

var (
	ConfigPodTrigger      chan bool
	ConfigUpdateDbTrigger chan *UpdateDb
)

func init() {
	ConfigPodTrigger = make(chan bool)
}

func (c *Configuration) addSmPolicyInfo(nwSlice *protos.NetworkSlice, dbUpdateChannel chan *UpdateDb) {
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

func (c *Configuration) updateConfig(commChannel chan *protos.NetworkSliceResponse, dbUpdateChannel chan *UpdateDb) bool {
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
					for _, cplmn := range UdrConfig.PlmnSupportList {
						if (cplmn.PlmnId.Mnc == plmn.PlmnId.Mnc) && (cplmn.PlmnId.Mcc == plmn.PlmnId.Mcc) {
							found = true
							break
						}
					}
					if !found {
						UdrConfig.PlmnSupportList = append(UdrConfig.PlmnSupportList, plmn)
					}
				} else {
					logger.GrpcLog.Infoln("Plmn not present in the message ")
				}
			}
			c.addSmPolicyInfo(ns, dbUpdateChannel)
		}
		if !minConfig {
			// first slice Created
			if len(UdrConfig.PlmnSupportList) > 0 {
				minConfig = true
				ConfigPodTrigger <- true
				logger.GrpcLog.Infoln("Send config trigger to main routine")
			}
		} else {
			// all slices deleted
			if len(UdrConfig.PlmnSupportList) == 0 {
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
