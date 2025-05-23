// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package message

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/ngap/ngapConvert"
	"github.com/omec-project/ngap/ngapType"
	"go.uber.org/zap"
)

func AppendPDUSessionResourceSetupListSUReq(list *ngapType.PDUSessionResourceSetupListSUReq,
	pduSessionID int32, snssai models.Snssai, nasPDU []byte, transfer []byte,
) {
	var item ngapType.PDUSessionResourceSetupItemSUReq
	item.PDUSessionID.Value = int64(pduSessionID)
	snssaiNgap, err := util.SNssaiToNgap(snssai)
	if err != nil {
		logger.AmfLog.Error("Convert SNssai to NGAP failed", zap.Error(err))
		return
	}
	item.SNSSAI = snssaiNgap
	item.PDUSessionResourceSetupRequestTransfer = transfer
	if nasPDU != nil {
		item.PDUSessionNASPDU = new(ngapType.NASPDU)
		item.PDUSessionNASPDU.Value = nasPDU
	}
	list.List = append(list.List, item)
}

func AppendPDUSessionResourceSetupListHOReq(list *ngapType.PDUSessionResourceSetupListHOReq,
	pduSessionID int32, snssai models.Snssai, transfer []byte,
) {
	var item ngapType.PDUSessionResourceSetupItemHOReq
	item.PDUSessionID.Value = int64(pduSessionID)
	snssaiNgap, err := util.SNssaiToNgap(snssai)
	if err != nil {
		logger.AmfLog.Error("Convert SNssai to NGAP failed", zap.Error(err))
		return
	}
	item.SNSSAI = snssaiNgap
	item.HandoverRequestTransfer = transfer
	list.List = append(list.List, item)
}

func AppendPDUSessionResourceSetupListCxtReq(list *ngapType.PDUSessionResourceSetupListCxtReq, pduSessionID int32, snssai models.Snssai, nasPDU []byte, transfer []byte) {
	var item ngapType.PDUSessionResourceSetupItemCxtReq
	item.PDUSessionID.Value = int64(pduSessionID)
	snssaiNgap, err := util.SNssaiToNgap(snssai)
	if err != nil {
		logger.AmfLog.Error("Convert SNssai to NGAP failed", zap.Error(err))
		return
	}
	item.SNSSAI = snssaiNgap
	if nasPDU != nil {
		item.NASPDU = new(ngapType.NASPDU)
		item.NASPDU.Value = nasPDU
	}
	item.PDUSessionResourceSetupRequestTransfer = transfer
	list.List = append(list.List, item)
}

func AppendPDUSessionResourceModifyListModReq(list *ngapType.PDUSessionResourceModifyListModReq,
	pduSessionID int32, nasPDU []byte, transfer []byte,
) {
	var item ngapType.PDUSessionResourceModifyItemModReq
	item.PDUSessionID.Value = int64(pduSessionID)
	item.PDUSessionResourceModifyRequestTransfer = transfer
	if nasPDU != nil {
		item.NASPDU = new(ngapType.NASPDU)
		item.NASPDU.Value = nasPDU
	}
	list.List = append(list.List, item)
}

func AppendPDUSessionResourceModifyListModCfm(list *ngapType.PDUSessionResourceModifyListModCfm,
	pduSessionID int64, transfer []byte,
) {
	var item ngapType.PDUSessionResourceModifyItemModCfm
	item.PDUSessionID.Value = pduSessionID
	item.PDUSessionResourceModifyConfirmTransfer = transfer
	list.List = append(list.List, item)
}

func AppendPDUSessionResourceToReleaseListRelCmd(list *ngapType.PDUSessionResourceToReleaseListRelCmd,
	pduSessionID int32, transfer []byte,
) {
	var item ngapType.PDUSessionResourceToReleaseItemRelCmd
	item.PDUSessionID.Value = int64(pduSessionID)
	item.PDUSessionResourceReleaseCommandTransfer = transfer
	list.List = append(list.List, item)
}

func BuildIEMobilityRestrictionList(ue *context.AmfUe) ngapType.MobilityRestrictionList {
	mobilityRestrictionList := ngapType.MobilityRestrictionList{}
	plmnID, err := util.PlmnIDToNgap(ue.PlmnID)
	if err != nil {
		logger.AmfLog.Error("Convert PLMN ID to NGAP failed", zap.Error(err))
		return mobilityRestrictionList
	}
	mobilityRestrictionList.ServingPLMN = *plmnID

	if ue.AccessAndMobilitySubscriptionData != nil && len(ue.AccessAndMobilitySubscriptionData.RatRestrictions) > 0 {
		mobilityRestrictionList.RATRestrictions = new(ngapType.RATRestrictions)
		ratRestrictions := mobilityRestrictionList.RATRestrictions
		for _, ratType := range ue.AccessAndMobilitySubscriptionData.RatRestrictions {
			item := ngapType.RATRestrictionsItem{}
			plmnID, err := util.PlmnIDToNgap(ue.PlmnID)
			if err != nil {
				logger.AmfLog.Error("Convert PLMN ID to NGAP failed", zap.Error(err))
				continue
			}
			item.PLMNIdentity = *plmnID
			item.RATRestrictionInformation = util.RATRestrictionInformationToNgap(ratType)
			ratRestrictions.List = append(ratRestrictions.List, item)
		}
	}

	if ue.AccessAndMobilitySubscriptionData != nil && len(ue.AccessAndMobilitySubscriptionData.ForbiddenAreas) > 0 {
		mobilityRestrictionList.ForbiddenAreaInformation = new(ngapType.ForbiddenAreaInformation)
		forbiddenAreaInformation := mobilityRestrictionList.ForbiddenAreaInformation
		for _, info := range ue.AccessAndMobilitySubscriptionData.ForbiddenAreas {
			item := ngapType.ForbiddenAreaInformationItem{}
			plmnID, err := util.PlmnIDToNgap(ue.PlmnID)
			if err != nil {
				logger.AmfLog.Error("Convert PLMN ID to NGAP failed", zap.Error(err))
				continue
			}
			item.PLMNIdentity = *plmnID
			for _, tac := range info.Tacs {
				tacBytes, err := hex.DecodeString(tac)
				if err != nil {
					logger.AmfLog.Error("DecodeString tac error", zap.Error(err))
				}
				tacNgap := ngapType.TAC{}
				tacNgap.Value = tacBytes
				item.ForbiddenTACs.List = append(item.ForbiddenTACs.List, tacNgap)
			}
			forbiddenAreaInformation.List = append(forbiddenAreaInformation.List, item)
		}
	}

	if ue.AmPolicyAssociation.ServAreaRes != nil {
		mobilityRestrictionList.ServiceAreaInformation = new(ngapType.ServiceAreaInformation)
		serviceAreaInformation := mobilityRestrictionList.ServiceAreaInformation

		item := ngapType.ServiceAreaInformationItem{}
		plmnID, err := util.PlmnIDToNgap(ue.PlmnID)
		if err != nil {
			logger.AmfLog.Error("Convert PLMN ID to NGAP failed", zap.Error(err))
			return mobilityRestrictionList
		}
		item.PLMNIdentity = *plmnID
		var tacList []ngapType.TAC
		for _, area := range ue.AmPolicyAssociation.ServAreaRes.Areas {
			for _, tac := range area.Tacs {
				tacBytes, err := hex.DecodeString(tac)
				if err != nil {
					logger.AmfLog.Error("DecodeString tac error", zap.Error(err))
				}
				tacNgap := ngapType.TAC{}
				tacNgap.Value = tacBytes
				tacList = append(tacList, tacNgap)
			}
		}
		if ue.AmPolicyAssociation.ServAreaRes.RestrictionType == models.RestrictionTypeAllowedAreas {
			item.AllowedTACs = new(ngapType.AllowedTACs)
			item.AllowedTACs.List = append(item.AllowedTACs.List, tacList...)
		} else {
			item.NotAllowedTACs = new(ngapType.NotAllowedTACs)
			item.NotAllowedTACs.List = append(item.NotAllowedTACs.List, tacList...)
		}
		serviceAreaInformation.List = append(serviceAreaInformation.List, item)
	}
	return mobilityRestrictionList
}

func BuildUnavailableGUAMIList(guamiList []models.Guami) (unavailableGUAMIList ngapType.UnavailableGUAMIList) {
	for _, guami := range guamiList {
		item := ngapType.UnavailableGUAMIItem{}
		plmnID, err := util.PlmnIDToNgap(*guami.PlmnID)
		if err != nil {
			logger.AmfLog.Error("Convert PLMN ID to NGAP failed", zap.Error(err))
			continue
		}
		item.GUAMI.PLMNIdentity = *plmnID
		regionID, setID, ptrID := ngapConvert.AmfIdToNgap(guami.AmfID)
		item.GUAMI.AMFRegionID.Value = regionID
		item.GUAMI.AMFSetID.Value = setID
		item.GUAMI.AMFPointer.Value = ptrID
		unavailableGUAMIList.List = append(unavailableGUAMIList.List, item)
	}
	return
}
