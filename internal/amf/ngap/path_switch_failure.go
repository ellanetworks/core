// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func pathSwitchSessions(ctx context.Context, ueConn *amf.UeConn, items ngap.PDUSessionResourceToBeSwitchedDLList) (present []amf.RANSession, undecodable []uint8) {
	present = make([]amf.RANSession, 0, len(items))

	for _, item := range items {
		pduSessionID, ok := validPDUSessionID(int64(item.PDUSessionID))
		if !ok {
			logger.WithTrace(ctx, ueConn.Log()).Error("invalid PDU session ID from gNB, not switched", zap.Int64("pduSessionID", int64(item.PDUSessionID)))

			undecodable = append(undecodable, uint8(item.PDUSessionID))

			continue
		}

		present = append(present, amf.RANSession{PduSessionID: pduSessionID, Transfer: item.Transfer})
	}

	return present, undecodable
}

func pathSwitchFailedSessions(items ngap.PDUSessionResourceFailedToSetupListPSReq) []uint8 {
	if len(items) == 0 {
		return nil
	}

	out := make([]uint8, 0, len(items))

	for _, item := range items {
		pduSessionID, ok := validPDUSessionID(int64(item.PDUSessionID))
		if !ok {
			continue
		}

		out = append(out, pduSessionID)
	}

	return out
}

// sendPathSwitchRequestFailure refuses a path switch. TS 38.413 §9.2.3.10 has
// no Cause IE for the message as a whole — unlike TS 36.413 §9.1.5.10 — so the
// reason is reported per session, once for every session the request named.
func sendPathSwitchRequestFailure(ctx context.Context, ran *amf.Radio, msg *ngap.PathSwitchRequest, causeValue int) {
	released, err := pathSwitchReleasedList(msg.PDUSessionResourceToBeSwitchedDLList, causeValue)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error building path switch released list", zap.Error(err))
	}

	amfID, ranID := msg.SourceAMFUENGAPID, msg.RANUENGAPID

	pkt, err := (&ngap.PathSwitchRequestFailure{
		AMFUENGAPID:                &amfID,
		RANUENGAPID:                &ranID,
		PDUSessionResourceReleased: released,
	}).Marshal()
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error building path switch request failure", zap.Error(err))
		return
	}

	ran.SendToRadio(ctx, amf.NGAPProcedurePathSwitchRequestFailure, pkt)
}

// pathSwitchReleasedList builds the released list with one item per distinct
// requested PDU session (TS 38.413).
func pathSwitchReleasedList(items ngap.PDUSessionResourceToBeSwitchedDLList, causeValue int) (ngap.PDUSessionResourceReleasedListPSFail, error) {
	transfer, err := buildPathSwitchRequestUnsuccessfulTransfer(causeValue)
	if err != nil {
		return nil, err
	}

	list := make(ngap.PDUSessionResourceReleasedListPSFail, 0, len(items))
	seen := make(map[ngap.PDUSessionID]struct{}, len(items))

	for _, item := range items {
		if _, dup := seen[item.PDUSessionID]; dup {
			continue
		}

		seen[item.PDUSessionID] = struct{}{}

		list = append(list, ngap.PDUSessionResourceReleasedItemPSFail{
			PDUSessionID: item.PDUSessionID,
			Transfer:     transfer,
		})
	}

	return list, nil
}

func buildPathSwitchRequestUnsuccessfulTransfer(causeValue int) (ngap.TransferContainer, error) {
	b, err := (&ngap.PathSwitchRequestUnsuccessfulTransfer{
		Cause: ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: causeValue},
	}).Marshal()
	if err != nil {
		return nil, fmt.Errorf("encode path switch request unsuccessful transfer: %w", err)
	}

	return b, nil
}

// sendPathSwitchProtocolFailure reports a PATH SWITCH REQUEST the AMF rejected
// on criticality grounds, using the procedure's own unsuccessful outcome as
// §10.3.4.2 requires. It reports false where the rejected message did not yield
// the sessions the mandatory released list has to name, which §10.3.5 answers
// with the Error Indication procedure instead.
func sendPathSwitchProtocolFailure(ctx context.Context, ran *amf.Radio, amfID ngap.AMFUENGAPID, ranID ngap.RANUENGAPID, ase *ngap.AbstractSyntaxError) bool {
	sessions := ase.PathSwitchSessions()
	if len(sessions) == 0 {
		return false
	}

	released, err := pathSwitchReleasedList(sessions, ngap.CauseRadioNetworkUnspecified)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error building path switch released list", zap.Error(err))
		return false
	}

	diagnostics := ase.OutcomeDiagnostics()

	pkt, err := (&ngap.PathSwitchRequestFailure{
		AMFUENGAPID:                &amfID,
		RANUENGAPID:                &ranID,
		PDUSessionResourceReleased: released,
		CriticalityDiagnostics:     &diagnostics,
	}).Marshal()
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("failed to marshal Path Switch Request Failure", zap.Error(err))
		return false
	}

	ran.SendToRadio(ctx, amf.NGAPProcedurePathSwitchRequestFailure, pkt)

	logger.WithTrace(ctx, ran.Log).Warn("Path Switch rejected", zap.Error(ase))

	return true
}
