// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

type RelAction int

const (
	UeContextN2NormalRelease RelAction = iota
	UeContextReleaseHandover
	UeContextReleaseUeContext
	UeContextReleaseDueToNwInitiatedDeregistraion
)

// RanUe represents one UE's radio-level state on a single Radio.
// It has no mutex of its own. It is protected either by the owning Radio's
// single SCTP goroutine, or by UeContext.Mutex when accessed via UeContext.RanUe().
//
// UeContext.RanUe() acquires a read-lock internally and returns a consistent
// snapshot. Callers must capture the returned pointer in a local variable
// and reuse it — never call RanUe() twice in the same code path.
type RanUe struct {
	RanUeNgapID      int64
	AmfUeNgapID      int64
	HandOverType     ngapType.HandoverType
	Tai              models.Tai
	Location         models.UserLocation
	amfUe            *UeContext
	radio            *Radio
	ReleaseAction    RelAction
	UeContextRequest bool
	ICS              ICSState
	Log              *zap.Logger
	freeNgapID       func(int64)
}

// ICSState tracks the AMF-side progress of the NGAP Initial Context Setup
// procedure for one RanUe.
type ICSState int

const (
	// ICSNotStarted: AMF has not sent InitialContextSetupRequest yet.
	ICSNotStarted ICSState = iota
	// ICSPending: InitialContextSetupRequest sent, awaiting response.
	ICSPending
	// ICSCompleted: InitialContextSetupResponse received.
	ICSCompleted
)

// Radio returns the Radio this RanUe is associated with, or nil.
func (ranUe *RanUe) Radio() *Radio {
	if ranUe == nil {
		return nil
	}

	return ranUe.radio
}

// UeContext returns the currently attached UeContext, or nil.
func (ranUe *RanUe) UeContext() *UeContext {
	if ranUe == nil {
		return nil
	}

	return ranUe.amfUe
}

// TouchLastSeen propagates a last-seen timestamp to the associated UeContext.
// Safe to call on nil receivers or when UeContext/Radio is nil.
func (ranUe *RanUe) TouchLastSeen() {
	if ranUe == nil || ranUe.amfUe == nil {
		return
	}

	radioName := ""
	if ranUe.radio != nil {
		radioName = ranUe.radio.Name
	}

	ranUe.amfUe.TouchLastSeen(radioName)
}

func (ranUe *RanUe) ngapSender() (NGAPSender, error) {
	if ranUe == nil {
		return nil, fmt.Errorf("ran ue is nil")
	}

	if ranUe.radio == nil {
		return nil, fmt.Errorf("radio is nil")
	}

	if ranUe.radio.NGAPSender == nil {
		return nil, fmt.Errorf("ngap sender is nil")
	}

	return ranUe.radio.NGAPSender, nil
}

func (ranUe *RanUe) SendDownlinkNasTransport(ctx context.Context, nasPdu []byte, mobilityRestrictionList *ngapType.MobilityRestrictionList) error {
	sender, err := ranUe.ngapSender()
	if err != nil {
		return err
	}

	return sender.SendDownlinkNasTransport(ctx, ranUe.AmfUeNgapID, ranUe.RanUeNgapID, nasPdu, mobilityRestrictionList)
}

func (ranUe *RanUe) SendUEContextReleaseCommand(ctx context.Context, causePresent int, cause aper.Enumerated) error {
	sender, err := ranUe.ngapSender()
	if err != nil {
		return err
	}

	return sender.SendUEContextReleaseCommand(ctx, ranUe.AmfUeNgapID, ranUe.RanUeNgapID, causePresent, cause)
}

func (ranUe *RanUe) SendPDUSessionResourceSetupRequest(ctx context.Context, ambrUp string, ambrDown string, nasPdu []byte, list ngapType.PDUSessionResourceSetupListSUReq) error {
	sender, err := ranUe.ngapSender()
	if err != nil {
		return err
	}

	return sender.SendPDUSessionResourceSetupRequest(ctx, ranUe.AmfUeNgapID, ranUe.RanUeNgapID, ambrUp, ambrDown, nasPdu, list)
}

func (ranUe *RanUe) SendPDUSessionResourceReleaseCommand(ctx context.Context, nasPdu []byte, list ngapType.PDUSessionResourceToReleaseListRelCmd) error {
	sender, err := ranUe.ngapSender()
	if err != nil {
		return err
	}

	return sender.SendPDUSessionResourceReleaseCommand(ctx, ranUe.AmfUeNgapID, ranUe.RanUeNgapID, nasPdu, list)
}

func (ranUe *RanUe) SendInitialContextSetupRequest(
	ctx context.Context,
	ambrUp string,
	ambrDown string,
	allowedNssai []models.Snssai,
	kgnb []byte,
	plmnID models.PlmnID,
	ueRadioCapability []byte,
	ueRadioCapabilityForPaging *models.UERadioCapabilityForPaging,
	ueSecurityCapability *nasType.UESecurityCapability,
	nasPdu []byte,
	pduSessionResourceSetupRequestList *ngapType.PDUSessionResourceSetupListCxtReq,
	supportedGUAMI *models.Guami,
) error {
	sender, err := ranUe.ngapSender()
	if err != nil {
		return err
	}

	return sender.SendInitialContextSetupRequest(
		ctx,
		ranUe.AmfUeNgapID,
		ranUe.RanUeNgapID,
		ambrUp,
		ambrDown,
		allowedNssai,
		kgnb,
		plmnID,
		ueRadioCapability,
		ueRadioCapabilityForPaging,
		ueSecurityCapability,
		nasPdu,
		pduSessionResourceSetupRequestList,
		supportedGUAMI,
	)
}

func (ranUe *RanUe) SendPDUSessionResourceModifyConfirm(
	ctx context.Context,
	pduSessionResourceModifyConfirmList ngapType.PDUSessionResourceModifyListModCfm,
	pduSessionResourceFailedToModifyList ngapType.PDUSessionResourceFailedToModifyListModCfm,
) error {
	sender, err := ranUe.ngapSender()
	if err != nil {
		return err
	}

	return sender.SendPDUSessionResourceModifyConfirm(
		ctx,
		ranUe.AmfUeNgapID,
		ranUe.RanUeNgapID,
		pduSessionResourceModifyConfirmList,
		pduSessionResourceFailedToModifyList,
	)
}

func (ranUe *RanUe) SendPDUSessionResourceModifyRequest(
	ctx context.Context,
	pduSessionResourceModifyList ngapType.PDUSessionResourceModifyListModReq,
) error {
	sender, err := ranUe.ngapSender()
	if err != nil {
		return err
	}

	return sender.SendPDUSessionResourceModifyRequest(
		ctx,
		ranUe.AmfUeNgapID,
		ranUe.RanUeNgapID,
		pduSessionResourceModifyList,
	)
}

func (ranUe *RanUe) SendHandoverPreparationFailure(ctx context.Context, cause ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	sender, err := ranUe.ngapSender()
	if err != nil {
		return err
	}

	return sender.SendHandoverPreparationFailure(ctx, ranUe.AmfUeNgapID, ranUe.RanUeNgapID, cause, criticalityDiagnostics)
}

func (ranUe *RanUe) SendHandoverCancelAcknowledge(ctx context.Context) error {
	sender, err := ranUe.ngapSender()
	if err != nil {
		return err
	}

	return sender.SendHandoverCancelAcknowledge(ctx, ranUe.AmfUeNgapID, ranUe.RanUeNgapID)
}

func (ranUe *RanUe) SendHandoverRequest(
	ctx context.Context,
	handOverType ngapType.HandoverType,
	uplinkAmbr string,
	downlinkAmbr string,
	ueSecurityCapability *nasType.UESecurityCapability,
	ncc uint8,
	nh []byte,
	cause ngapType.Cause,
	pduSessionResourceSetupListHOReq ngapType.PDUSessionResourceSetupListHOReq,
	sourceToTargetTransparentContainer ngapType.SourceToTargetTransparentContainer,
	snssaiList []models.Snssai,
	supportedGUAMI *models.Guami,
) error {
	sender, err := ranUe.ngapSender()
	if err != nil {
		return err
	}

	return sender.SendHandoverRequest(
		ctx,
		ranUe.AmfUeNgapID,
		handOverType,
		uplinkAmbr,
		downlinkAmbr,
		ueSecurityCapability,
		ncc,
		nh,
		cause,
		pduSessionResourceSetupListHOReq,
		sourceToTargetTransparentContainer,
		snssaiList,
		supportedGUAMI,
	)
}

// Remove tears down the RAN UE: it releases the NAS signalling connection
// to the bound AMF UE (with the given cause), removes the RAN UE from the
// radio's UE table, and frees its NGAP ID.
// abortHandoverIfPreparedTarget ends an in-flight N2 handover when this RanUe is
// its prepared target and the target is being removed (the target gNB association
// was reset or lost). The procedure is ended on the source — which stops the
// supervision guard — and the FSM cleared at once, rather than leaving a stale
// handover until the guard deadline. The source is left in place (its own handover
// timers abort it on the radio), mirroring the MME's ReclaimConns.
func (ranUe *RanUe) abortHandoverIfPreparedTarget(ctx context.Context) {
	ue := ranUe.amfUe
	if ue == nil || ranUe.radio.amf.HandoverTarget(ue) != ranUe {
		return
	}

	if conn := ue.NasConn(); conn != nil {
		conn.Procedures.End(procedure.N2Handover)
	}

	ranUe.radio.amf.ClearHandover(ue)

	logger.WithTrace(ctx, ranUe.Log).Info("aborted in-flight N2 handover: target association removed")
}

func (ranUe *RanUe) Remove(ctx context.Context) error {
	if ranUe == nil {
		return fmt.Errorf("ran ue is nil")
	}

	ranUe.abortHandoverIfPreparedTarget(ctx)

	if ranUe.amfUe != nil {
		ranUe.amfUe.ReleaseNasConnection(ranUe)
	}

	ran := ranUe.radio
	if ran == nil {
		return fmt.Errorf("ran not found in ranUe")
	}

	ran.amf.mu.Lock()
	delete(ran.amf.ranUEs, ranUe.AmfUeNgapID)
	ran.amf.mu.Unlock()

	if ranUe.freeNgapID != nil {
		ranUe.freeNgapID(ranUe.AmfUeNgapID)
	}

	logger.AmfLog.Info("ran ue removed",
		zap.Int64("amfUeNgapID", ranUe.AmfUeNgapID),
		zap.Int64("ranUeNgapID", ranUe.RanUeNgapID),
	)

	return nil
}

func (ranUe *RanUe) SwitchToRan(newRan *Radio, ranUeNgapID int64) error {
	if ranUe == nil {
		return fmt.Errorf("ran ue is nil")
	}

	if newRan == nil {
		return fmt.Errorf("new ran is nil")
	}

	// The global ranUEs index is keyed by the unchanged AMF UE NGAP ID, so the
	// switch only re-points the UE at its new radio and RAN UE NGAP ID.
	newRan.amf.mu.Lock()
	ranUe.radio = newRan
	ranUe.RanUeNgapID = ranUeNgapID
	newRan.amf.mu.Unlock()

	ranUe.Log = newRan.Log.With(logger.AmfUeNgapID(ranUe.AmfUeNgapID))

	ranUe.Log.Info("ran ue switched to new Ran", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))

	return nil
}

func (ranUe *RanUe) UpdateLocation(ctx context.Context, amf *AMF, userLocationInformation *ngapType.UserLocationInformation) {
	if userLocationInformation == nil {
		return
	}

	curTime := time.Now().UTC()

	switch userLocationInformation.Present {
	case ngapType.UserLocationInformationPresentUserLocationInformationEUTRA:
		locationInfoEUTRA := userLocationInformation.UserLocationInformationEUTRA

		if ranUe.Location.EutraLocation == nil {
			ranUe.Location.EutraLocation = new(models.EutraLocation)
		}

		tAI := locationInfoEUTRA.TAI
		plmnID := util.PlmnIDToModels(tAI.PLMNIdentity)
		tac := hex.EncodeToString(tAI.TAC.Value)

		if ranUe.Location.EutraLocation.Tai == nil {
			ranUe.Location.EutraLocation.Tai = new(models.Tai)
		}

		ranUe.Location.EutraLocation.Tai.PlmnID = &plmnID
		ranUe.Location.EutraLocation.Tai.Tac = tac
		ranUe.Tai = *ranUe.Location.EutraLocation.Tai

		eUTRACGI := locationInfoEUTRA.EUTRACGI
		ePlmnID := util.PlmnIDToModels(eUTRACGI.PLMNIdentity)
		eutraCellID := ngapConvert.BitStringToHex(&eUTRACGI.EUTRACellIdentity.Value)

		if ranUe.Location.EutraLocation.Ecgi == nil {
			ranUe.Location.EutraLocation.Ecgi = new(models.Ecgi)
		}

		ranUe.Location.EutraLocation.Ecgi.PlmnID = &ePlmnID
		ranUe.Location.EutraLocation.Ecgi.EutraCellID = eutraCellID

		ranUe.Location.EutraLocation.UeLocationTimestamp = &curTime
		if locationInfoEUTRA.TimeStamp != nil {
			ranUe.Location.EutraLocation.AgeOfLocationInformation = ngapConvert.TimeStampToInt32(
				locationInfoEUTRA.TimeStamp.Value)
		}

		if ranUe.amfUe != nil {
			ranUe.amfUe.Location = ranUe.Location
			ranUe.amfUe.Tai = *ranUe.amfUe.Location.EutraLocation.Tai
		}
	case ngapType.UserLocationInformationPresentUserLocationInformationNR:
		locationInfoNR := userLocationInformation.UserLocationInformationNR

		if ranUe.Location.NrLocation == nil {
			ranUe.Location.NrLocation = new(models.NrLocation)
		}

		tAI := locationInfoNR.TAI
		plmnID := util.PlmnIDToModels(tAI.PLMNIdentity)
		tac := hex.EncodeToString(tAI.TAC.Value)

		if ranUe.Location.NrLocation.Tai == nil {
			ranUe.Location.NrLocation.Tai = new(models.Tai)
		}

		ranUe.Location.NrLocation.Tai.PlmnID = &plmnID
		ranUe.Location.NrLocation.Tai.Tac = tac
		ranUe.Tai = *ranUe.Location.NrLocation.Tai

		nRCGI := locationInfoNR.NRCGI
		nRPlmnID := util.PlmnIDToModels(nRCGI.PLMNIdentity)
		nRCellID := ngapConvert.BitStringToHex(&nRCGI.NRCellIdentity.Value)

		if ranUe.Location.NrLocation.Ncgi == nil {
			ranUe.Location.NrLocation.Ncgi = new(models.Ncgi)
		}

		ranUe.Location.NrLocation.Ncgi.PlmnID = &nRPlmnID
		ranUe.Location.NrLocation.Ncgi.NrCellID = nRCellID

		ranUe.Location.NrLocation.UeLocationTimestamp = &curTime
		if locationInfoNR.TimeStamp != nil {
			ranUe.Location.NrLocation.AgeOfLocationInformation = ngapConvert.TimeStampToInt32(locationInfoNR.TimeStamp.Value)
		}

		if ranUe.amfUe != nil {
			ranUe.amfUe.Location = ranUe.Location
			ranUe.amfUe.Tai = *ranUe.amfUe.Location.NrLocation.Tai
		}
	case ngapType.UserLocationInformationPresentUserLocationInformationN3IWF:
		locationInfoN3IWF := userLocationInformation.UserLocationInformationN3IWF

		if ranUe.Location.N3gaLocation == nil {
			ranUe.Location.N3gaLocation = new(models.N3gaLocation)
		}

		ip := locationInfoN3IWF.IPAddress
		port := locationInfoN3IWF.PortNumber

		ipv4Addr, ipv6Addr := ngapConvert.IPAddressToString(ip)

		ranUe.Location.N3gaLocation.UeIpv4Addr = ipv4Addr
		ranUe.Location.N3gaLocation.UeIpv6Addr = ipv6Addr
		ranUe.Location.N3gaLocation.PortNumber = ngapConvert.PortNumberToInt(port)

		operatorInfo, err := amf.OperatorInfo(ctx)
		if err != nil {
			logger.AmfLog.Error("Error getting supported TAI list", zap.Error(err))
			return
		}

		tmp, err := strconv.ParseUint(operatorInfo.Tais[0].Tac, 10, 32)
		if err != nil {
			logger.AmfLog.Error("Error parsing TAC", zap.String("Tac", operatorInfo.Tais[0].Tac), zap.Error(err))
		}

		ranUe.Location.N3gaLocation.N3gppTai = &models.Tai{
			PlmnID: operatorInfo.Tais[0].PlmnID,
			Tac:    fmt.Sprintf("%06x", tmp),
		}

		ranUe.Tai = *ranUe.Location.N3gaLocation.N3gppTai

		if ranUe.amfUe != nil {
			ranUe.amfUe.Location = ranUe.Location
			ranUe.amfUe.Tai = *ranUe.Location.N3gaLocation.N3gppTai
		}
	case ngapType.UserLocationInformationPresentNothing:
	}
}

// NewRanUeForTest creates a RanUe and registers it in the AMF's ranUEs index.
// It is intended for use in external test packages only. If the radio is not yet
// bound to an AMF, a throwaway one is created so a handler invoked with this same
// radio resolves the UE; tests that share a specific AMF must BindAMFForTest first.
func NewRanUeForTest(radio *Radio, ranUeNgapID, amfUeNgapID int64, log *zap.Logger) *RanUe {
	if radio.amf == nil {
		radio.amf = New(nil, nil, nil)
	}

	ranUe := &RanUe{
		RanUeNgapID: ranUeNgapID,
		AmfUeNgapID: amfUeNgapID,
		radio:       radio,
		Log:         log,
	}

	radio.amf.mu.Lock()
	radio.amf.ranUEs[amfUeNgapID] = ranUe
	radio.amf.mu.Unlock()

	return ranUe
}
