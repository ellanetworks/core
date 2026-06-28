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
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// TS 23.502 4.9.1
func HandlePathSwitchRequest(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.PathSwitchRequest) {
	// TS 38.413 §8.4.4.4: a to-be-switched downlink list that repeats a PDU
	// Session ID is an abnormal condition the AMF rejects with a Path Switch
	// Request Failure.
	if id, dup := duplicatePDUSessionID(msg.PDUSessionResourceItems); dup {
		logger.WithTrace(ctx, ran.Log).Error("duplicate PDU Session ID in PathSwitchRequest to-be-switched list", zap.Int64("pduSessionID", id))
		sendPathSwitchRequestFailure(ctx, ran, msg, ngapType.CauseRadioNetworkPresentMultiplePDUSessionIDInstances)

		return
	}

	ranUe := amfInstance.FindRanUeByAmfUeNgapID(msg.SourceAMFUENGAPID)
	if ranUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("Cannot find UE from sourceAMfUeNgapID", zap.Int64("sourceAMFUENGAPID", msg.SourceAMFUENGAPID))
		sendPathSwitchRequestFailure(ctx, ran, msg, ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID)

		return
	}

	ranUe.TouchLastSeen()
	logger.WithTrace(ctx, ranUe.Log).Debug("Handle Path Switch Request", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))

	amfUe := ranUe.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, ranUe.Log).Error("UeContext is nil")
		sendPathSwitchRequestFailure(ctx, ran, msg, ngapType.CauseRadioNetworkPresentUnspecified)

		return
	}

	if !amfUe.SecurityContextIsValid() {
		logger.WithTrace(ctx, ranUe.Log).Error("No Security Context", logger.SUPI(amfUe.Supi.String()))
		sendPathSwitchRequestFailure(ctx, ran, msg, ngapType.CauseRadioNetworkPresentUnspecified)

		return
	}

	verifyUESecurityCapabilitiesOnPathSwitch(ctx, ranUe, amfUe, msg.UESecurityCapabilities)

	ranUe.RanUeNgapID = msg.RANUENGAPID

	ranUe.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation.Raw())

	var (
		pduSessionResourceSwitchedList      ngapType.PDUSessionResourceSwitchedList
		pduSessionResourceReleasedListPSAck ngapType.PDUSessionResourceReleasedListPSAck
	)

	for _, item := range msg.PDUSessionResourceItems {
		pduSessionID, ok := validPDUSessionID(item.PDUSessionID.Value)
		if !ok {
			logger.WithTrace(ctx, ranUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
			continue
		}

		transfer := item.PathSwitchRequestTransfer

		smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !ok {
			logger.WithTrace(ctx, ranUe.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
			continue
		}

		n2Rsp, err := amfInstance.Smf.UpdateSmContextXnHandoverPathSwitchReq(ctx, smContext.Ref, transfer)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error("SendUpdateSmContextXnHandover[PathSwitchRequestTransfer] Error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
			continue
		}

		pduSessionResourceSwitchedItem := ngapType.PDUSessionResourceSwitchedItem{}
		pduSessionResourceSwitchedItem.PDUSessionID.Value = int64(pduSessionID)
		pduSessionResourceSwitchedItem.PathSwitchRequestAcknowledgeTransfer = n2Rsp
		pduSessionResourceSwitchedList.List = append(pduSessionResourceSwitchedList.List, pduSessionResourceSwitchedItem)
	}

	for _, item := range msg.FailedToSetupItems {
		pduSessionID, ok := validPDUSessionID(item.PDUSessionID.Value)
		if !ok {
			logger.WithTrace(ctx, ranUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
			continue
		}

		transfer := item.PathSwitchRequestSetupFailedTransfer

		smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !ok {
			logger.WithTrace(ctx, ranUe.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
			continue
		}

		err := amfInstance.Smf.UpdateSmContextHandoverFailed(ctx, smContext.Ref, transfer)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error("SendUpdateSmContextXnHandoverFailed[PathSwitchRequestSetupFailedTransfer] Error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
		}
	}

	// TS 23.502 4.9.1.2.2 step 7: send ack to Target NG-RAN. If none of the requested PDU Sessions have been switched
	// successfully, the AMF shall send an N2 Path Switch Request Failure message to the Target NG-RAN
	if len(pduSessionResourceSwitchedList.List) > 0 {
		// TS 33.501 §6.9.2.3.2: compute fresh {NH, NCC} for the Ack
		err := amfUe.UpdateNH()
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error("error updating NH", zap.Error(err))
			return
		}

		err = ranUe.SwitchToRan(ran, msg.RANUENGAPID)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error(err.Error())
			return
		}

		snssaiList, err := amfInstance.ListOperatorSnssai(ctx)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error("List Operator SNSSAI Error", zap.Error(err))
			return
		}

		err = ranUe.Radio().NGAPSender.SendPathSwitchRequestAcknowledge(
			ctx,
			ranUe.AmfUeNgapID,
			ranUe.RanUeNgapID,
			amfUe.Current().UESecurityCapability,
			amfUe.Current().NCC,
			amfUe.Current().NH,
			pduSessionResourceSwitchedList,
			pduSessionResourceReleasedListPSAck,
			snssaiList,
		)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error("error sending path switch request acknowledge", zap.Error(err))
			return
		}
	} else {
		// TS 38.413 §8.4.4.3: no PDU session switched, so the whole path switch
		// fails and every requested session is released.
		sendPathSwitchRequestFailure(ctx, ran, msg, ngapType.CauseRadioNetworkPresentUnspecified)
	}
}

// verifyUESecurityCapabilitiesOnPathSwitch compares the UE 5G security
// capabilities reported by the target gNB against the AMF's stored
// values via the VerifyUESecurityCapability accessor and logs any
// mismatch (TS 33.501 §6.7.3.1). It never mutates amfUe — stored values
// are preserved by construction because the handler has no AuthProof.
func verifyUESecurityCapabilitiesOnPathSwitch(
	ctx context.Context,
	ranUe *amf.RanUe,
	amfUe *amf.UeContext,
	received *ngapType.UESecurityCapabilities,
) {
	if received == nil {
		return
	}

	if len(received.NRencryptionAlgorithms.Value.Bytes) == 0 ||
		len(received.NRintegrityProtectionAlgorithms.Value.Bytes) == 0 {
		logger.WithTrace(ctx, ranUe.Log).Warn(
			"UE security capabilities from target gNB have empty NR algorithm bitstrings; ignoring and using locally stored values",
		)

		return
	}

	reported := ngapToNasUESecurityCapability(received)

	switch amfUe.VerifyUESecurityCapability(reported) {
	case amf.VerifyMatch:
		return
	case amf.VerifyNoStoredValue:
		logger.WithTrace(ctx, ranUe.Log).Warn(
			"received UE security capabilities in PathSwitchRequest but AMF has no stored capabilities for this UE",
		)

		return
	case amf.VerifyMismatch:
		logger.WithTrace(ctx, ranUe.Log).Warn(
			"UE 5G security capabilities reported by target gNB differ from locally stored values; ignoring received values (TS 33.501 §6.7.3.1)",
			zap.Binary("stored", amfUe.Current().UESecurityCapability.Buffer),
			zap.Binary("received", reported.Buffer),
		)
	}
}

// ngapToNasUESecurityCapability converts the NGAP UESecurityCapabilities
// IE to the NAS UESecurityCapability type the AMF stores internally,
// using the common "first byte only" encoding (EA/IA 1..3 + EIA0).
//
// E-UTRA (EEA/EIA) bits carried by the NGAP IE are intentionally
// dropped: this AMF does not negotiate E-UTRA algorithms with the UE,
// so the verify path compares only the 5G NR columns. This matches the
// legacy behaviour of the pre-refactor handler.
func ngapToNasUESecurityCapability(received *ngapType.UESecurityCapabilities) *nasType.UESecurityCapability {
	out := &nasType.UESecurityCapability{}
	out.SetLen(2)

	encByte := received.NRencryptionAlgorithms.Value.Bytes[0]
	intByte := received.NRintegrityProtectionAlgorithms.Value.Bytes[0]

	out.SetEA1_128_5G((encByte & 0x80) >> 7)
	out.SetEA2_128_5G((encByte & 0x40) >> 6)
	out.SetEA3_128_5G((encByte & 0x20) >> 5)
	out.SetIA1_128_5G((intByte & 0x80) >> 7)
	out.SetIA2_128_5G((intByte & 0x40) >> 6)
	out.SetIA3_128_5G((intByte & 0x20) >> 5)

	return out
}
