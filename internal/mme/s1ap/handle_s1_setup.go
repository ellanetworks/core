// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"fmt"
	"slices"

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

// causeUnknownPLMN is S1AP Cause Misc "unknown-PLMN" (TS 36.413), returned in S1
// Setup Failure when the eNB broadcasts no PLMN this MME serves.
var causeUnknownPLMN = s1ap.Cause{Group: s1ap.CauseGroupMisc, Value: s1ap.CauseMiscUnknownPLMN}

// causeNoServedTAC is S1AP Cause Misc "unspecified", returned when the eNB
// broadcasts a served PLMN but no TAC this MME serves. TS 36.413 §8.7.3.4
// mandates rejection only on Unknown PLMN; rejecting an unserved TAC matches the
// AMF's NG Setup / RAN Configuration Update handling.
var causeNoServedTAC = s1ap.Cause{Group: s1ap.CauseGroupMisc, Value: s1ap.CauseMiscUnspecified}

// handleS1Setup answers an eNB's S1 Setup Request with an S1 Setup Response when
// the eNB broadcasts a TAI this MME serves, otherwise an S1 Setup Failure
// (TS 36.413).
func handleS1Setup(m *mme.MME, ctx context.Context, conn *sctp.SCTPConn, value []byte) {
	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		logger.From(ctx, m.RadioLog(conn)).Error("failed to get operator PLMN for S1 Setup", zap.Error(err))
		return
	}

	tacs, err := m.OperatorTACs(ctx)
	if err != nil {
		logger.From(ctx, m.RadioLog(conn)).Error("failed to get operator TACs for S1 Setup", zap.Error(err))
		return
	}

	mmeGroupID, mmeCode := m.MmeIdentity()

	req, outBytes, accepted, reason, err := s1SetupOutcomeFor(value, plmn, tacs, mmeGroupID, mmeCode)
	if err != nil {
		logger.From(ctx, m.RadioLog(conn)).Error("failed to handle S1 Setup Request", zap.Error(err))
		return
	}

	logger.From(ctx, m.RadioLog(conn)).Info("S1 Setup Request",
		zap.String("enb-name", req.ENBName),
		zap.Uint32("enb-id", req.GlobalENBID.ENBID.Value),
	)

	if !accepted {
		if _, err := conn.WriteMsg(outBytes, &sctp.SndRcvInfo{PPID: mme.S1apWirePPID, Stream: mme.S1apStreamNonUE}); err != nil {
			logger.From(ctx, m.RadioLog(conn)).Error("failed to send S1 Setup Failure", zap.Error(err))
			return
		}

		m.LogNetworkEvent(ctx, conn, mme.S1APProcedureS1SetupFailure, logger.DirectionOutbound, outBytes)

		logger.From(ctx, m.RadioLog(conn)).Warn("S1 Setup rejected",
			zap.String("enb-name", req.ENBName),
			zap.String("reason", reason),
			zap.String("served-plmn", plmn.Mcc+"/"+plmn.Mnc))

		return
	}

	if _, err := conn.WriteMsg(outBytes, &sctp.SndRcvInfo{PPID: mme.S1apWirePPID, Stream: mme.S1apStreamNonUE}); err != nil {
		logger.From(ctx, m.RadioLog(conn)).Error("failed to send S1 Setup Response", zap.Error(err))
		return
	}

	m.LogNetworkEvent(ctx, conn, mme.S1APProcedureS1SetupResponse, logger.DirectionOutbound, outBytes)

	// Allow the eNB's UE-associated signalling through the dispatcher's setup-first
	// gate (TS 36.413).
	m.MarkRadioSetupComplete(conn)

	logger.From(ctx, m.RadioLog(conn)).Info("S1 Setup Response sent", zap.String("enb-name", req.ENBName))
}

// s1SetupOutcomeFor returns an S1 Setup Response when the eNB broadcasts a served
// TAI, otherwise an S1 Setup Failure (TS 36.413). reason is a human-readable
// rejection summary, empty when accepted.
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
// not, the cause to reject with: "Unknown PLMN" when no broadcast PLMN matches,
// otherwise "unspecified" when a served PLMN is broadcast but no served TAC
// (TS 36.413). ok is true (cause unset) when a served TAI exists.
func servedTAICause(tas s1ap.SupportedTAs, plmn s1ap.PLMNIdentity, tacs []uint16) (cause s1ap.Cause, ok bool) {
	if !enbBroadcastsPLMN(tas, plmn) {
		return causeUnknownPLMN, false
	}

	if !enbBroadcastsServedTAI(tas, plmn, tacs) {
		return causeNoServedTAC, false
	}

	return s1ap.Cause{}, true
}

func enbBroadcastsPLMN(tas s1ap.SupportedTAs, plmn s1ap.PLMNIdentity) bool {
	for _, ta := range tas {
		if slices.Contains(ta.BroadcastPLMNs, plmn) {
			return true
		}
	}

	return false
}

// enbBroadcastsServedTAI reports whether the eNB broadcasts a TAI this MME serves:
// a supported TA whose TAC is one of tacs and that also broadcasts plmn
// (TS 36.413).
func enbBroadcastsServedTAI(tas s1ap.SupportedTAs, plmn s1ap.PLMNIdentity, tacs []uint16) bool {
	for _, ta := range tas {
		if !slices.Contains(tacs, uint16(ta.TAC)) {
			continue
		}

		if slices.Contains(ta.BroadcastPLMNs, plmn) {
			return true
		}
	}

	return false
}

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
