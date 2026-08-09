// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// RANSession is one PDU session a gNB reports it holds, with the opaque N2 transfer
// the SMF consumes. Unlike EPS, where the MME carries the eNB S1-U endpoint itself,
// the downlink endpoint stays inside the transfer (TS 38.413 §9.3.4).
type RANSession struct {
	PduSessionID uint8
	Transfer     []byte
}

// RANSessions is a gNB's view of a UE's PDU session set, as carried by one NGAP
// message.
//
// Authoritative distinguishes the two kinds of message 3GPP defines, exactly as
// RANBearers does for EPS. A PATH SWITCH REQUEST or a completed handover names
// everything the gNB holds, so a session in the UE context that appears in neither
// list has been implicitly released by the gNB (TS 38.413 §8.4.4.2). A message that
// reports only its own procedure's sessions is not authoritative.
type RANSessions struct {
	Present       []RANSession
	Rejected      []RANSession
	Authoritative bool
}

// AppliedSession is a session whose downlink now points at the gNB, with the N2
// response transfer to echo back.
type AppliedSession struct {
	PduSessionID uint8
	Transfer     []byte
}

// RANSessionResult reports the outcome per PDU session. Failed is what the caller
// reports back to the gNB so it releases the matching radio resources.
type RANSessionResult struct {
	Applied  []AppliedSession
	Failed   []uint8
	Released []uint8
}

// ReconcileSessionsToRAN converges a UE's PDU sessions onto the set a gNB reports.
// It is the 5G counterpart of MME.ReconcileBearersToRAN and follows the same rules:
//
//   - a session the gNB holds is applied at the SMF, and only a successful apply
//     counts as switched;
//   - a session the gNB rejected, or that the SMF could not converge, has its core
//     context released rather than only being reported;
//   - on an authoritative message, a session absent from both lists is released too.
//
// apply performs the procedure-specific SMF call — path switch and handover
// completion drive different Nsmf operations — and returns the N2 response transfer,
// if any. The caller keeps what is procedure-specific: rejecting duplicate PDU
// session IDs, choosing the response message, and deciding what a total failure
// means.
//
// There is no default-session re-election step: EPS has a default bearer to repair,
// 5GS has no equivalent (TS 23.501 §5.6).
func (a *AMF) ReconcileSessionsToRAN(
	ctx context.Context,
	ue *UeContext,
	want RANSessions,
	apply func(ctx context.Context, ref string, transfer []byte) ([]byte, error),
) RANSessionResult {
	var result RANSessionResult

	if ue == nil {
		return result
	}

	named := make(map[uint8]struct{}, len(want.Present)+len(want.Rejected))

	for _, s := range want.Present {
		named[s.PduSessionID] = struct{}{}

		smContext, ok := ue.SmContextFindByPDUSessionID(s.PduSessionID)
		if !ok {
			logger.From(ctx, logger.AmfLog).Warn("RAN reports a PDU session the core does not know; not switched",
				zap.Uint8("pdu-session-id", s.PduSessionID))

			result.Failed = append(result.Failed, s.PduSessionID)

			continue
		}

		n2Rsp, err := apply(ctx, smContext.Ref, s.Transfer)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Error("failed to switch a PDU session to the RAN endpoint",
				zap.String("smContextRef", smContext.Ref), zap.Uint8("pdu-session-id", s.PduSessionID), zap.Error(err))

			// The session has no usable downlink: free its core context rather than
			// leaving it pointed at a gNB the UE has left.
			a.releaseSession(ctx, ue, smContext.Ref, s.PduSessionID)

			result.Failed = append(result.Failed, s.PduSessionID)
			result.Released = append(result.Released, s.PduSessionID)

			continue
		}

		result.Applied = append(result.Applied, AppliedSession{PduSessionID: s.PduSessionID, Transfer: n2Rsp})
	}

	for _, s := range want.Rejected {
		named[s.PduSessionID] = struct{}{}

		smContext, ok := ue.SmContextFindByPDUSessionID(s.PduSessionID)
		if !ok {
			continue
		}

		a.releaseSession(ctx, ue, smContext.Ref, s.PduSessionID)
		result.Released = append(result.Released, s.PduSessionID)
	}

	if want.Authoritative {
		for _, sr := range ue.SmContextRefs() {
			if sr.Ref == "" {
				continue
			}

			if _, ok := named[sr.PduSessionID]; ok {
				continue
			}

			logger.From(ctx, logger.AmfLog).Info("releasing a PDU session the RAN did not report; implicitly released",
				zap.Uint8("pdu-session-id", sr.PduSessionID))

			a.releaseSession(ctx, ue, sr.Ref, sr.PduSessionID)
			result.Released = append(result.Released, sr.PduSessionID)
		}
	}

	return result
}

// releaseSession frees a PDU session's core context and drops it from the UE.
func (a *AMF) releaseSession(ctx context.Context, ue *UeContext, ref string, pduSessionID uint8) {
	if err := a.Session.ReleaseSmContext(ctx, ref); err != nil {
		logger.From(ctx, logger.AmfLog).Error("failed to release a PDU session",
			zap.String("smContextRef", ref), zap.Uint8("pdu-session-id", pduSessionID), zap.Error(err))
	}

	ue.DeleteSmContext(pduSessionID)
}

// DuplicatePDUSessionID reports the first PDU session identity that appears more
// than once. Several NGAP procedures make a repeated ID an abnormal condition but
// answer it differently, so detection is shared and the response stays with the
// caller (TS 38.413 §8.4.4.4). Mirrors mme.DuplicateEBI.
func DuplicatePDUSessionID(ids []uint8) (uint8, bool) {
	seen := make(map[uint8]struct{}, len(ids))

	for _, id := range ids {
		if _, ok := seen[id]; ok {
			return id, true
		}

		seen[id] = struct{}{}
	}

	return 0, false
}
