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

type RANSessions struct {
	Present             []RANSession
	Rejected            []uint8
	Authoritative       bool
	ReleaseOnApplyError bool
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
			logger.From(ctx, logger.AmfLog).Error("failed to converge a PDU session onto the RAN endpoint",
				logger.SUPI(ue.Supi().String()), zap.String("smContextRef", smContext.Ref),
				zap.Uint8("pdu-session-id", s.PduSessionID), zap.Error(err))

			result.Failed = append(result.Failed, s.PduSessionID)

			if want.ReleaseOnApplyError {
				a.releaseSession(ctx, ue, smContext.Ref, s.PduSessionID)
				result.Released = append(result.Released, s.PduSessionID)
			}

			continue
		}

		result.Applied = append(result.Applied, AppliedSession{PduSessionID: s.PduSessionID, Transfer: n2Rsp})
	}

	for _, id := range want.Rejected {
		named[id] = struct{}{}

		smContext, ok := ue.SmContextFindByPDUSessionID(id)
		if !ok {
			continue
		}

		a.releaseSession(ctx, ue, smContext.Ref, id)
		result.Released = append(result.Released, id)
	}

	if want.Authoritative {
		for _, sr := range ue.SmContextRefs() {
			if sr.Ref == "" {
				continue
			}

			if _, ok := named[sr.PduSessionID]; ok {
				continue
			}

			if sr.Inactive {
				continue
			}

			logger.From(ctx, logger.AmfLog).Info("releasing a PDU session the RAN did not report",
				logger.SUPI(ue.Supi().String()), zap.Uint8("pdu-session-id", sr.PduSessionID))

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
