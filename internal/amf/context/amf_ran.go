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

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/ngap/ngapType"
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
	RanPresent      int
	GlobalRanID     *models.GlobalRanNodeID
	Name            string
	AnType          models.AccessType
	GnbIP           string
	GnbID           string
	Conn            net.Conn
	SupportedTAList []SupportedTAI
	RanUeList       []*RanUe
	Log             *zap.SugaredLogger
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
	AmfSelf().DeleteAmfRan(ran.Conn)
	ran.Log.Infof("removed RAN Context [ID: %+v]", ran.RanID())
}

func (ran *AmfRan) NewRanUe(ranUeNgapID int64) (*RanUe, error) {
	ranUe := RanUe{}
	self := AmfSelf()
	amfUeNgapID, err := self.AllocateAmfUeNgapID()
	if err != nil {
		return nil, fmt.Errorf("error allocation amf ue ngap id: %v", err)
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

func (ran *AmfRan) SetRanID(RanNodeID *ngapType.GlobalRANNodeID) {
	ranID := util.RanIDToModels(*RanNodeID)
	ran.RanPresent = RanNodeID.Present
	ran.GlobalRanID = &ranID
	if RanNodeID.Present == ngapType.GlobalRANNodeIDPresentGlobalN3IWFID {
		ran.AnType = models.AccessTypeNon3GPPAccess
	} else {
		ran.AnType = models.AccessType3GPPAccess
	}

	// Setting RanId in String format with ":" separation of each field
	if ranID.PlmnID != nil {
		ran.GnbID = ranID.PlmnID.Mcc + ":" + ranID.PlmnID.Mnc + ":"
	}
	if ranID.GnbID != nil {
		ran.GnbID += ranID.GnbID.GNBValue
	}
}

func (ran *AmfRan) RanID() string {
	switch ran.RanPresent {
	case RanPresentGNbID:
		return fmt.Sprintf("<PlmnID: %+v, GNbID: %s>", *ran.GlobalRanID.PlmnID, ran.GlobalRanID.GnbID.GNBValue)
	case RanPresentN3IwfID:
		return fmt.Sprintf("<PlmnID: %+v, N3IwfID: %s>", *ran.GlobalRanID.PlmnID, ran.GlobalRanID.N3IwfID)
	case RanPresentNgeNbID:
		return fmt.Sprintf("<PlmnID: %+v, NgeNbID: %s>", *ran.GlobalRanID.PlmnID, ran.GlobalRanID.NgeNbID)
	default:
		return ""
	}
}
