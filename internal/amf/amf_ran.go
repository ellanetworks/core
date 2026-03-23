// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package amf

import (
	"context"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

const (
	RanPresentGNbID   = 1
	RanPresentNgeNbID = 2
	RanPresentN3IwfID = 3
)

type NGAPSender interface {
	SendToRan(ctx context.Context, packet []byte, msgType send.NGAPProcedure) error
	SendNGSetupFailure(ctx context.Context, cause *ngapType.Cause) error
	SendNGSetupResponse(ctx context.Context, guami *models.Guami, plmnSupported *models.PlmnSupportItem, amfName string, amfRelativeCapacity int64) error
	SendNGResetAcknowledge(ctx context.Context, partOfNGInterface *ngapType.UEAssociatedLogicalNGConnectionList) error
	SendErrorIndication(ctx context.Context, cause *ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error
	SendRanConfigurationUpdateAcknowledge(ctx context.Context, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error
	SendRanConfigurationUpdateFailure(ctx context.Context, cause ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error
	SendDownlinkRanConfigurationTransfer(ctx context.Context, transfer *ngapType.SONConfigurationTransfer) error
	SendPathSwitchRequestFailure(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, pduSessionResourceReleasedList *ngapType.PDUSessionResourceReleasedListPSFail, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error
	SendAMFStatusIndication(ctx context.Context, unavailableGUAMIList ngapType.UnavailableGUAMIList) error
	SendUEContextReleaseCommand(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, causePresent int, cause aper.Enumerated) error
	SendDownlinkNasTransport(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, nasPdu []byte, mobilityRestrictionList *ngapType.MobilityRestrictionList) error
	SendPDUSessionResourceReleaseCommand(ctx context.Context, amfUENgapID int64, ranUENgapID int64, nasPdu []byte, pduSessionResourceReleasedList ngapType.PDUSessionResourceToReleaseListRelCmd) error
	SendHandoverCancelAcknowledge(ctx context.Context, amfUENgapID int64, ranUENgapID int64) error
	SendPDUSessionResourceModifyConfirm(ctx context.Context, amfUENgapID int64, ranUENgapID int64, pduSessionResourceModifyConfirmList ngapType.PDUSessionResourceModifyListModCfm, pduSessionResourceFailedToModifyList ngapType.PDUSessionResourceFailedToModifyListModCfm) error
	SendPDUSessionResourceSetupRequest(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, ambrUplink string, ambrDownlink string, nasPdu []byte, pduSessionResourceSetupRequestList ngapType.PDUSessionResourceSetupListSUReq) error
	SendHandoverPreparationFailure(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, cause ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error
	SendLocationReportingControl(ctx context.Context, amfUENgapID int64, ranUENgapID int64, eventType ngapType.EventType) error
	SendHandoverCommand(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, handOverType ngapType.HandoverType, pduSessionResourceHandoverList ngapType.PDUSessionResourceHandoverList, pduSessionResourceToReleaseList ngapType.PDUSessionResourceToReleaseListHOCmd, container ngapType.TargetToSourceTransparentContainer) error
	SendInitialContextSetupRequest(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, ambrUplink string, ambrDownlink string, allowedNssai *models.Snssai, kgnb []byte, plmnID models.PlmnID, ueRadioCapability string, ueRadioCapabilityForPaging *models.UERadioCapabilityForPaging, ueSecurityCapability *nasType.UESecurityCapability, nasPdu []byte, pduSessionResourceSetupRequestList *ngapType.PDUSessionResourceSetupListCxtReq, supportedGUAMI *models.Guami) error
	SendPathSwitchRequestAcknowledge(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, ueSecurityCapability *nasType.UESecurityCapability, ncc uint8, nh []byte, pduSessionResourceSwitchedList ngapType.PDUSessionResourceSwitchedList, pduSessionResourceReleasedList ngapType.PDUSessionResourceReleasedListPSAck, supportedPLMN *models.PlmnSupportItem) error
	SendHandoverRequest(ctx context.Context, amfUeNgapID int64, handOverType ngapType.HandoverType, uplinkAmbr string, downlinkAmbr string, ueSecurityCapability *nasType.UESecurityCapability, ncc uint8, nh []byte, cause ngapType.Cause, pduSessionResourceSetupListHOReq ngapType.PDUSessionResourceSetupListHOReq, sourceToTargetTransparentContainer ngapType.SourceToTargetTransparentContainer, supportedPLMN *models.PlmnSupportItem, supportedGUAMI *models.Guami) error
}

// Radio represents one SCTP association to a gNB.
// All mutations happen on the single goroutine serving this connection.
// Do not access Radio fields from other goroutines without synchronization.
type Radio struct {
	RanPresent    int
	RanID         *models.GlobalRanNodeID
	NGAPSender    NGAPSender
	Name          string
	Conn          *sctp.SCTPConn
	ConnectedAt   time.Time
	LastSeenAt    time.Time
	SupportedTAIs []SupportedTAI
	mu            sync.RWMutex     // protects RanUEs
	RanUEs        map[int64]*RanUe // Key: RanUeNgapID
	Log           *zap.Logger
}

type SupportedTAI struct {
	Tai        models.Tai
	SNssaiList []models.Snssai
}

func (r *Radio) RemoveAllUeInRan() {
	r.mu.RLock()

	ues := make([]*RanUe, 0, len(r.RanUEs))
	for _, ranUe := range r.RanUEs {
		ues = append(ues, ranUe)
	}

	r.mu.RUnlock()

	for _, ranUe := range ues {
		err := ranUe.Remove()
		if err != nil {
			logger.AmfLog.Error("error removing ran ue", zap.Error(err))
		}
	}
}

func (r *Radio) FindUEByRanUeNgapID(ranUeNgapID int64) *RanUe {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ranUe, ok := r.RanUEs[ranUeNgapID]
	if ok {
		return ranUe
	}

	return nil
}

// FindUEByAmfUeNgapID returns the RAN UE with the given AMF UE NGAP ID, or nil.
func (r *Radio) FindUEByAmfUeNgapID(amfUeNgapID int64) *RanUe {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, ranUe := range r.RanUEs {
		if ranUe.AmfUeNgapID == amfUeNgapID {
			return ranUe
		}
	}

	return nil
}

func (r *Radio) SetRanID(ranNodeID *ngapType.GlobalRANNodeID) {
	ranID := util.RanIDToModels(*ranNodeID)
	r.RanPresent = ranNodeID.Present
	r.RanID = &ranID
}

func (r *Radio) TouchLastSeen() {
	r.LastSeenAt = time.Now()
}

// NodeID returns the RAN node identifier string regardless of radio type.
func (r *Radio) NodeID() string {
	if r.RanID == nil {
		return ""
	}

	switch r.RanPresent {
	case RanPresentGNbID:
		if r.RanID.GNbID != nil {
			return r.RanID.GNbID.GNBValue
		}
	case RanPresentNgeNbID:
		return r.RanID.NgeNbID
	case RanPresentN3IwfID:
		return r.RanID.N3IwfID
	}

	return ""
}

func (r *Radio) RanNodeTypeName() string {
	switch r.RanPresent {
	case RanPresentGNbID:
		return "gNB"
	case RanPresentNgeNbID:
		return "ng-eNB"
	case RanPresentN3IwfID:
		return "N3IWF"
	default:
		return "Unknown"
	}
}

func (r *Radio) ConnectedSubscribers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	supis := make([]string, 0, len(r.RanUEs))
	for _, ranUe := range r.RanUEs {
		if ranUe.AmfUe != nil && ranUe.AmfUe.Supi.IsValid() && ranUe.AmfUe.Supi.IsIMSI() {
			supis = append(supis, ranUe.AmfUe.Supi.IMSI())
		}
	}

	return supis
}
