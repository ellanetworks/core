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
		ueConn.Log(ctx).Error("failed to build PathSwitchRequestUnsuccessfulTransfer", zap.Error(err), logger.PDUSessionID(pduSessionID))
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
		ran.Log(ctx).Error("duplicate PDU Session ID in PathSwitchRequest to-be-switched list", logger.PDUSessionID(uint8(id)))
		sendPathSwitchRequestFailure(ctx, ran, msg, ngap.CauseRadioNetworkMultiplePDUSessionIDs)

		return
	}

	ueConn := amfInstance.LookupUeConn(models.AmfUeNgapID(msg.SourceAMFUENGAPID))
	if ueConn == nil {
		ran.Log(ctx).Error("Cannot find UE from sourceAMfUeNgapID", zap.Uint64("source_amf_ue_ngap_id", uint64(msg.SourceAMFUENGAPID)))
		sendPathSwitchRequestFailure(ctx, ran, msg, ngap.CauseRadioNetworkUnknownLocalUENGAPID)

		return
	}

	ueConn.TouchLastSeen()
	ueConn.Log(ctx).Debug("Handle Path Switch Request")

	amfUe := ueConn.UeContext()
	if amfUe == nil {
		ueConn.Log(ctx).Error("UeContext is nil")
		sendPathSwitchRequestFailure(ctx, ran, msg, ngap.CauseRadioNetworkUnspecified)

		return
	}

	if !amfUe.SecurityContextIsValid() {
		ueConn.Log(ctx).Error("No Security Context")
		sendPathSwitchRequestFailure(ctx, ran, msg, ngap.CauseRadioNetworkUnspecified)

		return
	}

	verifyUESecurityCapabilitiesOnPathSwitch(ctx, ueConn, amfUe, msg.UESecurityCapabilities)

	if !amfUe.BeginKeyChainProc(procedure.PathSwitch) {
		ueConn.Log(ctx).Warn("Path Switch rejected: a key-changing procedure is in progress")
		sendPathSwitchRequestFailure(ctx, ran, msg, ngap.CauseRadioNetworkUnspecified)

		return
	}

	defer amfUe.EndKeyChainProc(procedure.PathSwitch)

	for _, item := range msg.PDUSessionResourceFailedToSetup {
		pduSessionID, ok := validPDUSessionID(int64(item.PDUSessionID))
		if !ok {
			ueConn.Log(ctx).Error("invalid PDU session ID from gNB, skipping", logger.PDUSessionID(uint8(item.PDUSessionID)))
			continue
		}

		smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !ok {
			ueConn.Log(ctx).Error("SmContext not found", logger.PDUSessionID(pduSessionID))
			continue
		}

		if err := amfInstance.Session.UpdateSmContextXnHandoverFailed(ctx, smContext.Ref, item.Transfer); err != nil {
			ueConn.Log(ctx).Error("SendUpdateSmContextXnHandoverFailed[PathSwitchRequestSetupFailedTransfer] Error", zap.Error(err), logger.PDUSessionID(pduSessionID))
		}
	}

	nh, ncc, err := amfUe.AdvancePathSwitchNH()
	if err != nil {
		ueConn.Log(ctx).Error("error advancing NH", zap.Error(err))
		sendPathSwitchRequestFailure(ctx, ran, msg, ngap.CauseRadioNetworkUnspecified)

		return
	}

	snssaiList, err := amfInstance.ListOperatorSnssai(ctx)
	if err != nil {
		ueConn.Log(ctx).Error("List Operator SNSSAI Error", zap.Error(err))
		sendPathSwitchRequestFailure(ctx, ran, msg, ngap.CauseRadioNetworkUnspecified)

		return
	}

	present, undecodable := pathSwitchSessions(ctx, ueConn, msg.PDUSessionResourceToBeSwitchedDLList)

	result := amfInstance.ReconcileSessionsToRAN(ctx, amfUe, ueConn, amf.RANSessions{
		Present:  present,
		Rejected: pathSwitchFailedSessions(msg.PDUSessionResourceFailedToSetup),
	}, amfInstance.Session.UpdateSmContextXnHandoverPathSwitchReq)

	var (
		switched ngap.PDUSessionResourceSwitchedList
		released ngap.PDUSessionResourceReleasedListPSAck
	)

	for _, a := range result.Applied {
		switched = append(switched, ngap.PDUSessionResourceSwitchedItem{
			PDUSessionID: ngap.PDUSessionID(a.PduSessionID),
			Transfer:     ngap.TransferContainer(a.Transfer),
		})
	}

	for _, id := range result.Failed {
		appendPathSwitchReleasedItem(ctx, ueConn, &released, id, pathSwitchFailureCause(amfUe, id))
	}

	for _, id := range undecodable {
		appendPathSwitchReleasedItem(ctx, ueConn, &released, id, ngap.CauseRadioNetworkUnknownPDUSessionID)
	}

	if len(switched) == 0 {
		sendPathSwitchRequestFailure(ctx, ran, msg, ngap.CauseRadioNetworkUnspecified)
		return
	}

	if !amfInstance.CommitPathSwitch(ctx, amfUe, ueConn, ran, models.RanUeNgapID(msg.RANUENGAPID), nh, ncc) {
		ueConn.Log(ctx).Warn("Path Switch Request: UE released during the user-plane switch")
		sendPathSwitchRequestFailure(ctx, ran, msg, ngap.CauseRadioNetworkUnspecified)

		return
	}

	if msg.UserLocationInformation != nil {
		ueConn.UpdateLocation(ctx, *msg.UserLocationInformation)
	}

	ueConn.SendPathSwitchRequestAcknowledge(ctx, amfUe.UESecCap(), ncc, nh[:], switched, released, snssaiList)
}

func pathSwitchFailureCause(amfUe *amf.UeContext, pduSessionID uint8) int {
	if _, known := amfUe.SmContextFindByPDUSessionID(pduSessionID); !known {
		return ngap.CauseRadioNetworkUnknownPDUSessionID
	}

	return ngap.CauseRadioNetworkUnspecified
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
		ueConn.Log(ctx).Warn(
			"received UE security capabilities in PathSwitchRequest but AMF has no stored capabilities for this UE",
		)

		return
	case amf.VerifyMismatch:
		ueConn.Log(ctx).Warn(
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
