// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

const (
	relativeMMECapacity uint8 = 255
	mmeName                   = "ella"
)

// causeUnknownPLMN is S1AP Cause Misc "unknown-PLMN" (TS 36.413, the
// sixth Misc root value), returned in S1 Setup Failure when the eNB broadcasts no
// PLMN this MME serves.
var causeUnknownPLMN = s1ap.Cause{Group: s1ap.CauseGroupMisc, Value: s1ap.CauseMiscUnknownPLMN}

// causeNoServedTAC is S1AP Cause Misc "unspecified", returned in an S1 Setup or
// ENB Configuration Update Failure when the eNB broadcasts a served PLMN but no
// TAC this MME serves. TS 36.413 §8.7.3.4 mandates rejection only on Unknown
// PLMN; rejecting an unserved TAC matches the AMF, which fails NG Setup and RAN
// Configuration Update on the same condition.
var causeNoServedTAC = s1ap.Cause{Group: s1ap.CauseGroupMisc, Value: s1ap.CauseMiscUnspecified}

// handleS1Setup answers an eNB's S1 Setup Request: an S1 Setup Response when the
// eNB broadcasts a TAI (PLMN + TAC) this MME serves, otherwise an S1 Setup
// Failure with cause "Unknown PLMN" or, for an unserved TAC, "unspecified"
// (TS 36.413).
func handleS1Setup(m *mme.MME, ctx context.Context, conn *sctp.SCTPConn, value []byte) {
	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		logger.MmeLog.Error("failed to get operator PLMN for S1 Setup", zap.Error(err))
		return
	}

	tacs, err := m.OperatorTACs(ctx)
	if err != nil {
		logger.MmeLog.Error("failed to get operator TACs for S1 Setup", zap.Error(err))
		return
	}

	mmeGroupID, mmeCode := m.MmeIdentity()

	req, outBytes, accepted, reason, err := s1SetupOutcomeFor(value, plmn, tacs, mmeGroupID, mmeCode)
	if err != nil {
		logger.MmeLog.Error("failed to handle S1 Setup Request", zap.Error(err))
		return
	}

	logger.MmeLog.Info("S1 Setup Request",
		zap.String("enb-name", req.ENBName),
		zap.Uint32("enb-id", req.GlobalENBID.ENBID.Value),
	)

	if !accepted {
		if _, err := conn.WriteMsg(outBytes, &sctp.SndRcvInfo{PPID: mme.S1apWirePPID, Stream: mme.S1apStreamNonUE}); err != nil {
			logger.MmeLog.Error("failed to send S1 Setup Failure", zap.Error(err))
			return
		}

		m.LogNetworkEvent(ctx, conn, mme.S1APProcedureS1SetupFailure, logger.DirectionOutbound, outBytes)

		logger.MmeLog.Warn("S1 Setup rejected",
			zap.String("enb-name", req.ENBName),
			zap.String("reason", reason),
			zap.String("served-plmn", plmn.Mcc+"/"+plmn.Mnc))

		return
	}

	if _, err := conn.WriteMsg(outBytes, &sctp.SndRcvInfo{PPID: mme.S1apWirePPID, Stream: mme.S1apStreamNonUE}); err != nil {
		logger.MmeLog.Error("failed to send S1 Setup Response", zap.Error(err))
		return
	}

	m.LogNetworkEvent(ctx, conn, mme.S1APProcedureS1SetupResponse, logger.DirectionOutbound, outBytes)

	// S1 Setup has completed: allow the eNB's UE-associated signalling through the
	// dispatcher's setup-first gate (TS 36.413).
	m.MarkENBSetupComplete(conn)

	logger.MmeLog.Info("S1 Setup Response sent", zap.String("enb-name", req.ENBName))
}

// handleENBConfigurationUpdate answers an eNB's ENB CONFIGURATION UPDATE: it
// validates that any updated supported TAs still broadcast a PLMN this MME serves
// (otherwise an ENB CONFIGURATION UPDATE FAILURE with cause "Unknown PLMN"),
// stores an updated eNB name, and acknowledges (TS 36.413 §8.7.4). The eNB blocks
// on this response, so an unhandled update would stall its reconfiguration.
func handleENBConfigurationUpdate(m *mme.MME, ctx context.Context, conn *sctp.SCTPConn, value []byte) {
	req, err := s1ap.ParseENBConfigurationUpdate(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode ENB Configuration Update", zap.Error(err))
		return
	}

	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		logger.MmeLog.Error("failed to get operator PLMN for ENB Configuration Update", zap.Error(err))
		return
	}

	tacs, err := m.OperatorTACs(ctx)
	if err != nil {
		logger.MmeLog.Error("failed to get operator TACs for ENB Configuration Update", zap.Error(err))
		return
	}

	out, accepted, err := enbConfigUpdateOutcomeFor(req, plmn, tacs)
	if err != nil {
		logger.MmeLog.Error("failed to handle ENB Configuration Update", zap.Error(err))
		return
	}

	msgType := mme.S1APProcedureENBConfigUpdateAck
	if !accepted {
		msgType = mme.S1APProcedureENBConfigUpdateFailure
	}

	if _, err := conn.WriteMsg(out, &sctp.SndRcvInfo{PPID: mme.S1apWirePPID, Stream: mme.S1apStreamNonUE}); err != nil {
		logger.MmeLog.Error("failed to send ENB Configuration Update response", zap.Error(err))
		return
	}

	m.LogNetworkEvent(ctx, conn, msgType, logger.DirectionOutbound, out)

	if !accepted {
		logger.MmeLog.Warn("ENB Configuration Update rejected: eNB broadcasts no TAI (PLMN + TAC) served by this MME")
		return
	}

	if req.ENBName != "" {
		m.UpdateENBName(conn, req.ENBName)
	}

	if len(req.SupportedTAs) > 0 {
		m.UpdateENBSupportedTAs(conn, mme.EnbSupportedTAIs(req.SupportedTAs))
	}

	logger.MmeLog.Info("ENB Configuration Update acknowledged", zap.String("enb-name", req.ENBName))
}

// enbConfigUpdateOutcomeFor produces the S1AP response to an ENB CONFIGURATION
// UPDATE: an Acknowledge when any updated supported TAs still broadcast a TAI
// (PLMN + TAC) this MME serves, otherwise an ENB CONFIGURATION UPDATE FAILURE
// with cause "Unknown PLMN" or, for an unserved TAC, "unspecified"
// (TS 36.413 §8.7.4). accepted reports which was produced. An update with no
// supported TAs (a name- or DRX-only change) is always accepted.
func enbConfigUpdateOutcomeFor(req *s1ap.ENBConfigurationUpdate, plmn models.PlmnID, tacs []uint16) (out []byte, accepted bool, err error) {
	if len(req.SupportedTAs) > 0 {
		served, err := mme.EncodePLMN(plmn)
		if err != nil {
			return nil, false, fmt.Errorf("mme: encode served PLMN: %w", err)
		}

		cause, ok := servedTAICause(req.SupportedTAs, served, tacs)
		if !ok {
			out, err = (&s1ap.ENBConfigurationUpdateFailure{Cause: cause}).Marshal()
			if err != nil {
				return nil, false, fmt.Errorf("mme: marshal ENB Configuration Update Failure: %w", err)
			}

			return out, false, nil
		}
	}

	out, err = (&s1ap.ENBConfigurationUpdateAcknowledge{}).Marshal()
	if err != nil {
		return nil, false, fmt.Errorf("mme: marshal ENB Configuration Update Acknowledge: %w", err)
	}

	return out, true, nil
}

// s1SetupOutcomeFor decodes an S1 Setup Request and produces the S1AP message to
// send back: an S1 Setup Response when the eNB broadcasts a TAI (PLMN + TAC) this
// MME serves, otherwise an S1 Setup Failure with cause "Unknown PLMN" or, for an
// unserved TAC, "unspecified" (TS 36.413). accepted reports which outcome was
// produced; reason is a human-readable rejection summary, empty when accepted.
func s1SetupOutcomeFor(reqValue []byte, plmn models.PlmnID, tacs []uint16, mmeGroupID uint16, mmeCode uint8) (req *s1ap.S1SetupRequest, out []byte, accepted bool, reason string, err error) {
	req, err = s1ap.ParseS1SetupRequest(reqValue)
	if err != nil {
		return nil, nil, false, "", fmt.Errorf("mme: parse S1 Setup Request: %w", err)
	}

	served, err := mme.EncodePLMN(plmn)
	if err != nil {
		return req, nil, false, "", fmt.Errorf("mme: encode served PLMN: %w", err)
	}

	if cause, ok := servedTAICause(req.SupportedTAs, served, tacs); !ok {
		out, err = (&s1ap.S1SetupFailure{Cause: cause}).Marshal()
		if err != nil {
			return req, nil, false, "", fmt.Errorf("mme: marshal S1 Setup Failure: %w", err)
		}

		reason = "eNB broadcasts no PLMN served by this MME (Unknown PLMN)"
		if cause == causeNoServedTAC {
			reason = "eNB broadcasts a served PLMN but no TAC served by this MME"
		}

		return req, out, false, reason, nil
	}

	resp, err := buildS1SetupResponse(plmn, mmeGroupID, mmeCode)
	if err != nil {
		return req, nil, false, "", err
	}

	out, err = resp.Marshal()
	if err != nil {
		return req, nil, false, "", fmt.Errorf("mme: marshal S1 Setup Response: %w", err)
	}

	return req, out, true, "", nil
}

// servedTAICause reports whether the eNB broadcasts a TAI this MME serves and, if
// not, the S1AP cause to reject with: "Unknown PLMN" when no broadcast PLMN
// matches, otherwise "unspecified" when a served PLMN is broadcast but with no
// served TAC (TS 36.413). ok is true (and cause unset) when a served TAI exists.
func servedTAICause(tas s1ap.SupportedTAs, plmn s1ap.PLMNIdentity, tacs []uint16) (cause s1ap.Cause, ok bool) {
	if !enbBroadcastsPLMN(tas, plmn) {
		return causeUnknownPLMN, false
	}

	if !enbBroadcastsServedTAI(tas, plmn, tacs) {
		return causeNoServedTAC, false
	}

	return s1ap.Cause{}, true
}

// enbBroadcastsPLMN reports whether any PLMN the eNB broadcasts across its
// supported TAs equals plmn (TS 36.413).
func enbBroadcastsPLMN(tas s1ap.SupportedTAs, plmn s1ap.PLMNIdentity) bool {
	for _, ta := range tas {
		for _, b := range ta.BroadcastPLMNs {
			if b == plmn {
				return true
			}
		}
	}

	return false
}

// enbBroadcastsServedTAI reports whether the eNB broadcasts a TAI this MME serves:
// a supported TA whose TAC is one of tacs and that also broadcasts plmn
// (TS 36.413).
func enbBroadcastsServedTAI(tas s1ap.SupportedTAs, plmn s1ap.PLMNIdentity, tacs []uint16) bool {
	for _, ta := range tas {
		served := false

		for _, t := range tacs {
			if t == uint16(ta.TAC) {
				served = true

				break
			}
		}

		if !served {
			continue
		}

		for _, b := range ta.BroadcastPLMNs {
			if b == plmn {
				return true
			}
		}
	}

	return false
}

// servedGUMMEIs builds the Served GUMMEIs advertised in the S1 Setup Response:
// the operator PLMN combined with the configured MME group ID and code.
func servedGUMMEIs(plmn models.PlmnID, mmeGroupID uint16, mmeCode uint8) (s1ap.ServedGUMMEIs, error) {
	p, err := mme.EncodePLMN(plmn)
	if err != nil {
		return nil, err
	}

	return s1ap.ServedGUMMEIs{{
		ServedPLMNs:    []s1ap.PLMNIdentity{p},
		ServedGroupIDs: []s1ap.MMEGroupID{{byte(mmeGroupID >> 8), byte(mmeGroupID)}},
		ServedMMECs:    []s1ap.MMECode{s1ap.MMECode(mmeCode)},
	}}, nil
}

// buildS1SetupResponse assembles this MME's S1 Setup Response identity for the
// given operator PLMN and configured MME identity.
func buildS1SetupResponse(plmn models.PlmnID, mmeGroupID uint16, mmeCode uint8) (*s1ap.S1SetupResponse, error) {
	gummeis, err := servedGUMMEIs(plmn, mmeGroupID, mmeCode)
	if err != nil {
		return nil, err
	}

	return &s1ap.S1SetupResponse{
		MMEName:             mmeName,
		ServedGUMMEIs:       gummeis,
		RelativeMMECapacity: relativeMMECapacity,
	}, nil
}
