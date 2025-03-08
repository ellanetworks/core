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
	RanPresentGNbId   = 1
	RanPresentNgeNbId = 2
	RanPresentN3IwfId = 3
)

type AmfRan struct {
	RanPresent      int
	RanId           *models.GlobalRanNodeId
	Name            string
	AnType          models.AccessType
	GnbIp           string
	GnbId           string // RanId in string format, i.e.,mcc:mnc:gnbid
	Conn            net.Conn
	SupportedTAList []SupportedTAI
	RanUeList       []*RanUe // RanUeNgapId as key
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
	AMFSelf().DeleteAmfRan(ran.Conn)
}

func (ran *AmfRan) NewRanUe(ranUeNgapID int64) (*RanUe, error) {
	ranUe := RanUe{}
	self := AMFSelf()
	amfUeNgapID, err := self.AllocateAmfUeNgapID()
	if err != nil {
		return nil, fmt.Errorf("error allocating amf ue ngap id: %+v", err)
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
		err := ranUe.Remove()
		if err != nil {
			logger.AmfLog.Errorf("error removing ran ue: %+v", err)
		}
	}
}

func (ran *AmfRan) RanUeFindByRanUeNgapIDLocal(ranUeNgapID int64) *RanUe {
	for _, ranUe := range ran.RanUeList {
		if ranUe.RanUeNgapId == ranUeNgapID {
			return ranUe
		}
	}
	ran.Log.Infof("ran ue not found: %d", ranUeNgapID)
	return nil
}

func (ran *AmfRan) RanUeFindByRanUeNgapID(ranUeNgapID int64) *RanUe {
	ranUe := ran.RanUeFindByRanUeNgapIDLocal(ranUeNgapID)
	return ranUe
}

func (ran *AmfRan) SetRanID(ranNodeID *ngapType.GlobalRANNodeID) {
	ranId := util.RanIdToModels(*ranNodeID)
	ran.RanPresent = ranNodeID.Present
	ran.RanId = &ranId
	if ranNodeID.Present == ngapType.GlobalRANNodeIDPresentGlobalN3IWFID {
		ran.AnType = models.AccessType_NON_3_GPP_ACCESS
	} else {
		ran.AnType = models.AccessType__3_GPP_ACCESS
	}

	if ranId.PlmnId != nil {
		ran.GnbId = ranId.PlmnId.Mcc + ":" + ranId.PlmnId.Mnc + ":"
	}
	if ranId.GNbId != nil {
		ran.GnbId += ranId.GNbId.GNBValue
	}
}
