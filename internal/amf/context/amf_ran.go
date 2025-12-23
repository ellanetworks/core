// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"context"
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

type NGAPSender interface {
	SendNGSetupFailure(ctx context.Context, cause *ngapType.Cause) error
	SendNGSetupResponse(ctx context.Context, guami *models.Guami, plmnSupported *models.PlmnSupportItem, amfName string, amfRelativeCapacity int64) error
	SendNGResetAcknowledge(ctx context.Context, partOfNGInterface *ngapType.UEAssociatedLogicalNGConnectionList) error
	SendErrorIndication(ctx context.Context, amfUeNgapID, ranUeNgapID *int64, cause *ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error
	SendRanConfigurationUpdateAcknowledge(ctx context.Context, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error
	SendRanConfigurationUpdateFailure(ctx context.Context, cause ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error
	SendDownlinkRanConfigurationTransfer(ctx context.Context, transfer *ngapType.SONConfigurationTransfer) error
}

type AmfRan struct {
	RanPresent      int
	RanID           *models.GlobalRanNodeID
	NGAPSender      NGAPSender
	Name            string
	GnbIP           string
	Conn            *sctp.SCTPConn
	SupportedTAList []SupportedTAI
	RanUePool       map[int64]*RanUe // Key: RanUeNgapID
	Log             *zap.Logger
}

type SupportedTAI struct {
	Tai        models.Tai
	SNssaiList []models.Snssai
}

func (ran *AmfRan) Remove() {
	ran.RemoveAllUeInRan()
	AMFSelf().DeleteAmfRan(ran.Conn)
}

func (ran *AmfRan) NewRanUe(ranUeNgapID int64) (*RanUe, error) {
	self := AMFSelf()

	amfUeNgapID, err := self.AllocateAmfUeNgapID()
	if err != nil {
		return nil, fmt.Errorf("error allocating amf ue ngap id: %+v", err)
	}

	ranUe := RanUe{}
	ranUe.AmfUeNgapID = amfUeNgapID
	ranUe.RanUeNgapID = ranUeNgapID
	ranUe.Ran = ran
	ranUe.Log = ran.Log.With(zap.String("AMF_UE_NGAP_ID", fmt.Sprintf("%d", ranUe.AmfUeNgapID)))

	ran.RanUePool[ranUe.RanUeNgapID] = &ranUe

	self.Mutex.Lock()
	defer self.Mutex.Unlock()

	return &ranUe, nil
}

func (ran *AmfRan) RemoveAllUeInRan() {
	for _, ranUe := range ran.RanUePool {
		err := ranUe.Remove()
		if err != nil {
			logger.AmfLog.Error("error removing ran ue", zap.Error(err))
		}
	}
}

func (ran *AmfRan) RanUeFindByRanUeNgapID(ranUeNgapID int64) *RanUe {
	ranUe, ok := ran.RanUePool[ranUeNgapID]
	if ok {
		return ranUe
	}

	ran.Log.Debug("Ran UE not found", zap.Int64("ranUeNgapID", ranUeNgapID))

	return nil
}

func (ran *AmfRan) SetRanID(ranNodeID *ngapType.GlobalRANNodeID) {
	ranID := util.RanIDToModels(*ranNodeID)
	ran.RanPresent = ranNodeID.Present
	ran.RanID = &ranID
}
