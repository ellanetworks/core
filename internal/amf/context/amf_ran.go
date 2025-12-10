// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"fmt"

	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

const (
	RanPresentGNbID   = 1
	RanPresentNgeNbID = 2
	RanPresentN3IwfID = 3
)

type AmfRan struct {
	RanPresent      int
	RanID           *models.GlobalRanNodeID
	Name            string
	GnbIP           string
	GnbID           string // RanID in string format, i.e.,mcc:mnc:gnbid
	Conn            *sctp.SCTPConn
	SupportedTAList []SupportedTAI
	RanUeList       []*RanUe // RanUeNgapID as key
	Log             *zap.Logger
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
}

func (ran *AmfRan) NewRanUe(ranUeNgapID int64) (*RanUe, error) {
	ranUe := RanUe{}
	self := AMFSelf()
	amfUeNgapID, err := self.AllocateAmfUeNgapID()
	if err != nil {
		return nil, fmt.Errorf("error allocating amf ue ngap id: %+v", err)
	}
	ranUe.AmfUeNgapID = amfUeNgapID
	ranUe.RanUeNgapID = ranUeNgapID
	ranUe.Ran = ran
	ranUe.Log = ran.Log.With(zap.String("AMF_UE_NGAP_ID", fmt.Sprintf("%d", ranUe.AmfUeNgapID)))
	ran.RanUeList = append(ran.RanUeList, &ranUe)
	self.RanUePool.Store(ranUe.AmfUeNgapID, &ranUe)
	return &ranUe, nil
}

func (ran *AmfRan) RemoveAllUeInRan() {
	for _, ranUe := range ran.RanUeList {
		err := ranUe.Remove()
		if err != nil {
			logger.AmfLog.Error("error removing ran ue", zap.Error(err))
		}
	}
}

func (ran *AmfRan) RanUeFindByRanUeNgapIDLocal(ranUeNgapID int64) *RanUe {
	for _, ranUe := range ran.RanUeList {
		if ranUe.RanUeNgapID == ranUeNgapID {
			return ranUe
		}
	}
	ran.Log.Debug("Ran UE not found", zap.Int64("ranUeNgapID", ranUeNgapID))
	return nil
}

func (ran *AmfRan) RanUeFindByRanUeNgapID(ranUeNgapID int64) *RanUe {
	ranUe := ran.RanUeFindByRanUeNgapIDLocal(ranUeNgapID)
	return ranUe
}

func (ran *AmfRan) SetRanID(ranNodeID *ngapType.GlobalRANNodeID) {
	ranID := util.RanIDToModels(*ranNodeID)
	ran.RanPresent = ranNodeID.Present
	ran.RanID = &ranID

	if ranID.PlmnID != nil {
		ran.GnbID = ranID.PlmnID.Mcc + ":" + ranID.PlmnID.Mnc + ":"
	}

	if ranID.GNbID != nil {
		ran.GnbID += ranID.GNbID.GNBValue
	}
}
