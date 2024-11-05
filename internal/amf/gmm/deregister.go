package gmm

import (
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/fsm"
	"github.com/yeastengine/ella/internal/amf/context"
	"github.com/yeastengine/ella/internal/amf/logger"
)

const IMSI_PREFIX = "imsi-"

func SendDeregistrationMessage(imsi string, snssai models.Snssai) {
	logger.CfgLog.Infof("Delete subscriber from Network Slice [sst:%v sd:%v]", snssai.Sst, snssai.Sd)
	amfSelf := context.AMF_Self()
	ue, ok := amfSelf.AmfUeFindBySupi(IMSI_PREFIX + imsi)
	if !ok {
		logger.CfgLog.Warnf("UE not found")
		return
	}
	configMsg := context.ConfigMsg{
		Supi: imsi,
		Sst:  snssai.Sst,
		Sd:   snssai.Sd,
	}
	ue.SetEventChannel(nil)
	ue.EventChannel.UpdateConfigHandler(UeConfigSliceDeleteHandler)
	ue.EventChannel.SubmitMessage(configMsg)
}

func UeConfigSliceDeleteHandler(supi string, sst int32, sd string) {
	amfSelf := context.AMF_Self()
	ue, _ := amfSelf.AmfUeFindBySupi(IMSI_PREFIX + supi)
	if len(ue.AllowedNssai[models.AccessType__3_GPP_ACCESS]) == 1 {
		if ue.AllowedNssai[models.AccessType__3_GPP_ACCESS][0].AllowedSnssai.Sst == sst &&
			ue.AllowedNssai[models.AccessType__3_GPP_ACCESS][0].AllowedSnssai.Sd == sd {
			err := GmmFSM.SendEvent(ue.State[models.AccessType__3_GPP_ACCESS], NwInitiatedDeregistrationEvent, fsm.ArgsType{
				ArgAmfUe:      ue,
				ArgAccessType: models.AccessType__3_GPP_ACCESS,
			})
			if err != nil {
				logger.CfgLog.Errorln(err)
			}
		} else {
			logger.CfgLog.Infof("Deleted slice not matched with slice info in UEContext")
		}
	} else {
		snssai := models.Snssai{
			Sst: sst,
			Sd:  sd,
		}
		err := GmmFSM.SendEvent(ue.State[models.AccessType__3_GPP_ACCESS], SliceInfoDeleteEvent, fsm.ArgsType{
			ArgAmfUe:      ue,
			ArgAccessType: models.AccessType__3_GPP_ACCESS,
			ArgNssai:      snssai,
		})
		if err != nil {
			logger.CfgLog.Errorln(err)
		}
	}
}
