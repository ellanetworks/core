// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package message

import (
	"fmt"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func AppendPDUSessionResourceSetupListSUReq(list *ngapType.PDUSessionResourceSetupListSUReq,
	pduSessionID uint8, snssai *models.Snssai, nasPDU []byte, transfer []byte,
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

func AppendPDUSessionResourceSetupListHOReq(list *ngapType.PDUSessionResourceSetupListHOReq, pduSessionID uint8, snssai *models.Snssai, transfer []byte) {
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

func AppendPDUSessionResourceSetupListCxtReq(list *ngapType.PDUSessionResourceSetupListCxtReq, pduSessionID uint8, snssai *models.Snssai, nasPDU []byte, transfer []byte) {
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

func AppendPDUSessionResourceToReleaseListRelCmd(list *ngapType.PDUSessionResourceToReleaseListRelCmd,
	pduSessionID uint8, transfer []byte,
) {
	var item ngapType.PDUSessionResourceToReleaseItemRelCmd
	item.PDUSessionID.Value = int64(pduSessionID)
	item.PDUSessionResourceReleaseCommandTransfer = transfer
	list.List = append(list.List, item)
}

func BuildIEMobilityRestrictionList(plmnID models.PlmnID) (*ngapType.MobilityRestrictionList, error) {
	plmnIDNGAP, err := util.PlmnIDToNgap(plmnID)
	if err != nil {
		return nil, fmt.Errorf("could not convert PLMN ID to NGAP: %s", err)
	}

	return &ngapType.MobilityRestrictionList{
		ServingPLMN: *plmnIDNGAP,
	}, nil
}

func BuildUnavailableGUAMIList(guami *models.Guami) (unavailableGUAMIList ngapType.UnavailableGUAMIList) {
	item := ngapType.UnavailableGUAMIItem{}

	plmnID, err := util.PlmnIDToNgap(*guami.PlmnID)
	if err != nil {
		logger.AmfLog.Error("Convert PLMN ID to NGAP failed", zap.Error(err))
		return
	}

	item.GUAMI.PLMNIdentity = *plmnID
	regionID, setID, ptrID := ngapConvert.AmfIdToNgap(guami.AmfID)
	item.GUAMI.AMFRegionID.Value = regionID
	item.GUAMI.AMFSetID.Value = setID
	item.GUAMI.AMFPointer.Value = ptrID
	unavailableGUAMIList.List = append(unavailableGUAMIList.List, item)
	return
}
