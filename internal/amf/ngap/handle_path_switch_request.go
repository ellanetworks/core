// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// appendPathSwitchReleasedItem records a PDU session the core could not switch in the
// PATH SWITCH REQUEST ACKNOWLEDGE PDU Session Resource Released List so the NG-RAN
// releases it; a session left unswitched has no downlink path (TS 38.413 §8.4.4.2).
func appendPathSwitchReleasedItem(ctx context.Context, ueConn *amf.UeConn, list *ngap.PDUSessionResourceReleasedListPSAck, pduSessionID uint8, causeValue int) {
	transfer, err := (&ngap.PathSwitchRequestUnsuccessfulTransfer{
		Cause: ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: causeValue},
	}).Marshal()
	if err != nil {
		logger.WithTrace(ctx, ueConn.Log).Error("failed to build PathSwitchRequestUnsuccessfulTransfer", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
		return
	}

	*list = append(*list, ngap.PDUSessionResourceReleasedItemPSAck{
		PDUSessionID: ngap.PDUSessionID(pduSessionID),
		Transfer:     transfer,
	})
}

func HandlePathSwitchRequest(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.PathSwitchRequest) {
	// TS 38.413: a to-be-switched downlink list that repeats a PDU Session ID is an
	// abnormal condition the AMF rejects with a Path Switch Request Failure.
	if id, dup := duplicatePDUSessionID(msg.PDUSessionResourceToBeSwitchedDLList); dup {
		logger.WithTrace(ctx, ran.Log).Error("duplicate PDU Session ID in PathSwitchRequest to-be-switched list", zap.Int64("pduSessionID", id))
		sendPathSwitchRequestFailure(ctx, ran, msg, ngap.CauseRadioNetworkMultiplePDUSessionIDs)

		return
	}

	ueConn := amfInstance.LookupUeConn(models.AmfUeNgapID(msg.SourceAMFUENGAPID))
	if ueConn == nil {
		logger.WithTrace(ctx, ran.Log).Error("Cannot find UE from sourceAMfUeNgapID", zap.Uint64("source-amf-ue-id", uint64(msg.SourceAMFUENGAPID)))
		sendPathSwitchRequestFailure(ctx, ran, msg, ngap.CauseRadioNetworkUnknownLocalUENGAPID)

		return
	}

	ueConn.TouchLastSeen()
	logger.WithTrace(ctx, ueConn.Log).Debug("Handle Path Switch Request", zap.Uint64("amf-ue-id", uint64(ueConn.AmfUeNgapID)), zap.Uint32("ran-ue-id", uint32(ueConn.RanUeNgapID)))

	amfUe := ueConn.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, ueConn.Log).Error("UeContext is nil")
		sendPathSwitchRequestFailure(ctx, ran, msg, ngap.CauseRadioNetworkUnspecified)

		return
	}

	if !amfUe.SecurityContextIsValid() {
		logger.WithTrace(ctx, ueConn.Log).Error("No Security Context", logger.SUPI(amfUe.Supi().String()))
		sendPathSwitchRequestFailure(ctx, ran, msg, ngap.CauseRadioNetworkUnspecified)

		return
	}

	verifyUESecurityCapabilitiesOnPathSwitch(ctx, ueConn, amfUe, msg.UESecurityCapabilities)

	ueConn.RanUeNgapID = models.RanUeNgapID(msg.RANUENGAPID)

	if msg.UserLocationInformation != nil {
		ueConn.UpdateLocation(ctx, *msg.UserLocationInformation)
	}

	// Claim the {NH,NCC} key chain for the whole path switch: a concurrent N2 handover or
	// NAS SMC (possibly on another gNB's dispatch goroutine) must not advance the same
	// chain in parallel (TS 33.501 §6.9.5). Claimed before the SMF is touched so a rejected
	// path switch changes nothing. Path switch is synchronous, so hold until return.
	if !amfUe.BeginKeyChainProc(procedure.PathSwitch) {
		logger.WithTrace(ctx, ueConn.Log).Warn("Path Switch rejected: a key-changing procedure is in progress")
		sendPathSwitchRequestFailure(ctx, ran, msg, ngap.CauseRadioNetworkUnspecified)

		return
	}

	defer amfUe.EndKeyChainProc(procedure.PathSwitch)

	var (
		switched ngap.PDUSessionResourceSwitchedList
		released ngap.PDUSessionResourceReleasedListPSAck
	)

	for _, item := range msg.PDUSessionResourceToBeSwitchedDLList {
		pduSessionID, ok := validPDUSessionID(int64(item.PDUSessionID))
		if !ok {
			logger.WithTrace(ctx, ueConn.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", int64(item.PDUSessionID)))
			continue
		}

		transfer := item.Transfer

		smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !ok {
			logger.WithTrace(ctx, ueConn.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
			appendPathSwitchReleasedItem(ctx, ueConn, &released, pduSessionID, ngap.CauseRadioNetworkUnknownPDUSessionID)

			continue
		}

		n2Rsp, err := amfInstance.Session.UpdateSmContextXnHandoverPathSwitchReq(ctx, smContext.Ref, transfer)
		if err != nil {
			logger.WithTrace(ctx, ueConn.Log).Error("SendUpdateSmContextXnHandover[PathSwitchRequestTransfer] Error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
			appendPathSwitchReleasedItem(ctx, ueConn, &released, pduSessionID, ngap.CauseRadioNetworkUnspecified)

			continue
		}

		switched = append(switched, ngap.PDUSessionResourceSwitchedItem{
			PDUSessionID: ngap.PDUSessionID(pduSessionID),
			Transfer:     ngap.TransferContainer(n2Rsp),
		})
	}

	for _, item := range msg.PDUSessionResourceFailedToSetup {
		pduSessionID, ok := validPDUSessionID(int64(item.PDUSessionID))
		if !ok {
			logger.WithTrace(ctx, ueConn.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", int64(item.PDUSessionID)))
			continue
		}

		transfer := item.Transfer

		smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !ok {
			logger.WithTrace(ctx, ueConn.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
			continue
		}

		err := amfInstance.Session.UpdateSmContextHandoverFailed(ctx, smContext.Ref, transfer)
		if err != nil {
			logger.WithTrace(ctx, ueConn.Log).Error("SendUpdateSmContextXnHandoverFailed[PathSwitchRequestSetupFailedTransfer] Error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
		}
	}

	// TS 23.502: acknowledge to the target NG-RAN; if no session switched, fail the path switch.
	if len(switched) > 0 {
		// TS 33.501: derive fresh {NH, NCC} but commit them only once the switch is
		// confirmed, so an abandoned path switch never advances the live AS key chain.
		nh, ncc, err := amfUe.AdvancePathSwitchNH()
		if err != nil {
			logger.WithTrace(ctx, ueConn.Log).Error("error advancing NH", zap.Error(err))
			return
		}

		snssaiList, err := amfInstance.ListOperatorSnssai(ctx)
		if err != nil {
			logger.WithTrace(ctx, ueConn.Log).Error("List Operator SNSSAI Error", zap.Error(err))
			return
		}

		// Re-point the UE at the target radio and commit the chain atomically: a UE
		// released during the user-plane switch fails the path switch with the chain
		// left unadvanced.
		if !amfInstance.CommitPathSwitch(amfUe, ueConn, ran, models.RanUeNgapID(msg.RANUENGAPID), nh, ncc) {
			logger.WithTrace(ctx, ueConn.Log).Warn("Path Switch Request: UE released during the user-plane switch")
			sendPathSwitchRequestFailure(ctx, ran, msg, ngap.CauseRadioNetworkUnspecified)

			return
		}

		ueConn.SendPathSwitchRequestAcknowledge(ctx, amfUe.UESecCap(), ncc, nh[:], switched, released, snssaiList)
	} else {
		// TS 38.413: no session switched, so the path switch fails and every requested
		// session is released.
		sendPathSwitchRequestFailure(ctx, ran, msg, ngap.CauseRadioNetworkUnspecified)
	}
}

// verifyUESecurityCapabilitiesOnPathSwitch logs any mismatch between the UE 5G
// security capabilities reported by the target gNB and the AMF's stored values
// (TS 33.501). It never mutates amfUe: the reported values are not trusted on
// the path-switch path.
func verifyUESecurityCapabilitiesOnPathSwitch(
	ctx context.Context,
	ueConn *amf.UeConn,
	amfUe *amf.UeContext,
	received *ngap.UESecurityCapabilities,
) {
	if received == nil {
		return
	}

	reported := ngapToNasUESecurityCapability(received)

	switch amfUe.VerifyUESecurityCapability(reported) {
	case amf.VerifyMatch:
		return
	case amf.VerifyNoStoredValue:
		logger.WithTrace(ctx, ueConn.Log).Warn(
			"received UE security capabilities in PathSwitchRequest but AMF has no stored capabilities for this UE",
		)

		return
	case amf.VerifyMismatch:
		logger.WithTrace(ctx, ueConn.Log).Warn(
			"UE 5G security capabilities reported by target gNB differ from locally stored values; ignoring received values (TS 33.501)",
			zap.Stringer("stored", amfUe.UESecCap()),
			zap.Stringer("received", reported),
		)
	}
}

// ngapToNasUESecurityCapability converts the NGAP UESecurityCapabilities IE to
// the NAS UE security capability the AMF stores (5G-EA1/IA1 at bit 7, EA2/IA2 at
// bit 6, EA3/IA3 at bit 5).
//
// E-UTRA (EEA/EIA) bits carried by the NGAP IE are dropped: this AMF does not
// negotiate E-UTRA algorithms with the UE, so the verify path compares only the
// 5G NR columns.
func ngapToNasUESecurityCapability(received *ngap.UESecurityCapabilities) *fgs.UESecurityCapability {
	enc := byte(received.NREncryptionAlgorithms >> 8)
	integ := byte(received.NRIntegrityProtectionAlgorithms >> 8)

	return &fgs.UESecurityCapability{
		EA: nas.AlgorithmSet((enc>>7&1)<<6 | (enc>>6&1)<<5 | (enc>>5&1)<<4),
		IA: nas.AlgorithmSet((integ>>7&1)<<6 | (integ>>6&1)<<5 | (integ>>5&1)<<4),
	}
}
