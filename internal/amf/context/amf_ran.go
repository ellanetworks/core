// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/ngap/ngapConvert"
	"github.com/omec-project/ngap/ngapType"
	"github.com/omec-project/openapi/models"
	"go.uber.org/zap"
)

const (
	RanPresentGNbID   = 1
	RanPresentNgeNbID = 2
	RanPresentN3IwfID = 3
	RanConnected      = "Connected"
	RanDisconnected   = "Disconnected"
)

type AmfRan struct {
	RanPresent int
	RanId      *models.GlobalRanNodeId
	Name       string
	AnType     models.AccessType
	GnbIP      string `json:"-"`
	GnbID      string // RanId in string format, i.e.,mcc:mnc:gnbid
	/* socket Connect*/
	Conn net.Conn `json:"-"`
	/* Supported TA List */
	SupportedTAList []SupportedTAI

	/* RAN UE List */
	RanUeList []*RanUe `json:"-"` // RanUeNgapID as key

	/* logger */
	Log *zap.SugaredLogger `json:"-"`
}

type SupportedTAI struct {
	Tai        models.Tai
	SNssaiList []models.Snssai
}

func NewSupportedTAI() (tai SupportedTAI) {
	tai.SNssaiList = make([]models.Snssai, 0, MaxNumOfSlice)
	return
}

func NewSupportedTAIList() []SupportedTAI {
	return make([]SupportedTAI, 0, MaxNumOfTAI*MaxNumOfBroadcastPLMNs)
}

func (ran *AmfRan) Remove() {
	ran.RemoveAllUeInRan()
	AMFSelf().DeleteAmfRan(ran.Conn)
	ran.Log.Infof("removed RAN Context [ID: %+v]", ran.RanID())
}

func (ran *AmfRan) NewRanUe(ranUeNgapID int64) (*RanUe, error) {
	ranUe := RanUe{}
	self := AMFSelf()
	amfUeNgapID, err := self.AllocateAmfUeNgapID()
	if err != nil {
		ran.Log.Errorln("Alloc Amf ue ngap id failed", err)
		return nil, fmt.Errorf("could not allocate AMF UE NGAP ID")
	}
	ranUe.AmfUeNgapID = amfUeNgapID
	ranUe.RanUeNgapID = ranUeNgapID
	ranUe.Ran = ran
	ranUe.Log = ran.Log.With(logger.FieldAmfUeNgapID, fmt.Sprintf("AMF_UE_NGAP_ID:%d", ranUe.AmfUeNgapID))
	ran.RanUeList = append(ran.RanUeList, &ranUe)
	self.RanUePool.Store(ranUe.AmfUeNgapID, &ranUe)
	return &ranUe, nil
}

func (ran *AmfRan) RemoveAllUeInRan() {
	for _, ranUe := range ran.RanUeList {
		if err := ranUe.Remove(); err != nil {
			logger.AmfLog.Errorf("Remove RanUe error: %v", err)
		}
	}
}

func (ran *AmfRan) RanUeFindByRanUeNgapIDLocal(ranUeNgapID int64) *RanUe {
	for _, ranUe := range ran.RanUeList {
		if ranUe.RanUeNgapID == ranUeNgapID {
			return ranUe
		}
	}
	ran.Log.Infof("RanUe not found [RanUeNgapID: %d]", ranUeNgapID)
	return nil
}

func (ran *AmfRan) RanUeFindByRanUeNgapID(ranUeNgapID int64) *RanUe {
	ranUe := ran.RanUeFindByRanUeNgapIDLocal(ranUeNgapID)

	if ranUe != nil {
		return ranUe
	}

	return nil
}

func (ran *AmfRan) SetRanID(ranNodeID *ngapType.GlobalRANNodeID) {
	ranID := ngapConvert.RanIdToModels(*ranNodeID)
	ran.RanPresent = ranNodeID.Present
	ran.RanId = &ranID
	if ranNodeID.Present == ngapType.GlobalRANNodeIDPresentGlobalN3IWFID {
		ran.AnType = models.AccessType_NON_3_GPP_ACCESS
	} else {
		ran.AnType = models.AccessType__3_GPP_ACCESS
	}

	// Setting RanId in String format with ":" separation of each field
	if ranID.PlmnId != nil {
		ran.GnbID = ranID.PlmnId.Mcc + ":" + ranID.PlmnId.Mnc + ":"
	}
	if ranID.GNbId != nil {
		ran.GnbID += ranID.GNbId.GNBValue
	}
}

func (ran *AmfRan) RanID() string {
	switch ran.RanPresent {
	case RanPresentGNbID:
		return fmt.Sprintf("<PlmnID: %+v, GNbID: %s>", *ran.RanId.PlmnId, ran.RanId.GNbId.GNBValue)
	case RanPresentN3IwfID:
		return fmt.Sprintf("<PlmnID: %+v, N3IwfID: %s>", *ran.RanId.PlmnId, ran.RanId.N3IwfId)
	case RanPresentNgeNbID:
		return fmt.Sprintf("<PlmnID: %+v, NgeNbID: %s>", *ran.RanId.PlmnId, ran.RanId.NgeNbId)
	default:
		return ""
	}
}
