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
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/ngap/ngapConvert"
	"github.com/omec-project/ngap/ngapType"
	"github.com/omec-project/openapi/models"
	"go.uber.org/zap"
)

const (
	RanPresentGNbId   = 1
	RanPresentNgeNbId = 2
	RanPresentN3IwfId = 3
	RanConnected      = "Connected"
	RanDisconnected   = "Disconnected"
)

type AmfRan struct {
	RanPresent int
	RanId      *models.GlobalRanNodeId
	Name       string
	AnType     models.AccessType
	GnbIp      string `json:"-"`
	GnbId      string // RanId in string format, i.e.,mcc:mnc:gnbid
	/* socket Connect*/
	Conn net.Conn `json:"-"`
	/* Supported TA List */
	SupportedTAList []SupportedTAI

	/* RAN UE List */
	RanUeList []*RanUe `json:"-"` // RanUeNgapId as key

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
	AMF_Self().DeleteAmfRan(ran.Conn)
	ran.Log.Infof("removed RAN Context [ID: %+v]", ran.RanID())
}

func (ran *AmfRan) NewRanUe(ranUeNgapID int64) (*RanUe, error) {
	ranUe := RanUe{}
	self := AMF_Self()
	amfUeNgapID, err := self.AllocateAmfUeNgapID()
	if err != nil {
		ran.Log.Errorln("Alloc Amf ue ngap id failed", err)
		return nil, fmt.Errorf("Allocate AMF UE NGAP ID error: %+v", err)
	}
	ranUe.AmfUeNgapId = amfUeNgapID
	ranUe.RanUeNgapId = ranUeNgapID
	ranUe.Ran = ran
	ranUe.Log = ran.Log.With(logger.FieldAmfUeNgapID, fmt.Sprintf("AMF_UE_NGAP_ID:%d", ranUe.AmfUeNgapId))
	ran.RanUeList = append(ran.RanUeList, &ranUe)
	self.RanUePool.Store(ranUe.AmfUeNgapId, &ranUe)
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
		if ranUe.RanUeNgapId == ranUeNgapID {
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

func (ran *AmfRan) SetRanId(ranNodeId *ngapType.GlobalRANNodeID) {
	ranId := ngapConvert.RanIdToModels(*ranNodeId)
	ran.RanPresent = ranNodeId.Present
	ran.RanId = &ranId
	if ranNodeId.Present == ngapType.GlobalRANNodeIDPresentGlobalN3IWFID {
		ran.AnType = models.AccessType_NON_3_GPP_ACCESS
	} else {
		ran.AnType = models.AccessType__3_GPP_ACCESS
	}

	// Setting RanId in String format with ":" separation of each field
	if ranId.PlmnId != nil {
		ran.GnbId = ranId.PlmnId.Mcc + ":" + ranId.PlmnId.Mnc + ":"
	}
	if ranId.GNbId != nil {
		ran.GnbId += ranId.GNbId.GNBValue
	}
}

func (ran *AmfRan) ConvertGnbIdToRanId(gnbId string) (ranNodeId *models.GlobalRanNodeId) {
	var ranId *models.GlobalRanNodeId = &models.GlobalRanNodeId{}
	val := strings.Split(gnbId, ":")
	if len(val) != 3 {
		return nil
	}
	ranId.PlmnId = &models.PlmnId{Mcc: val[0], Mnc: val[1]}
	ranId.GNbId = &models.GNbId{GNBValue: val[2]}
	ran.RanPresent = RanPresentGNbId
	return ranId
}

func (ran *AmfRan) RanID() string {
	switch ran.RanPresent {
	case RanPresentGNbId:
		return fmt.Sprintf("<PlmnID: %+v, GNbID: %s>", *ran.RanId.PlmnId, ran.RanId.GNbId.GNBValue)
	case RanPresentN3IwfId:
		return fmt.Sprintf("<PlmnID: %+v, N3IwfID: %s>", *ran.RanId.PlmnId, ran.RanId.N3IwfId)
	case RanPresentNgeNbId:
		return fmt.Sprintf("<PlmnID: %+v, NgeNbID: %s>", *ran.RanId.PlmnId, ran.RanId.NgeNbId)
	default:
		return ""
	}
}
