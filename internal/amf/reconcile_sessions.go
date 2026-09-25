// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type RANSession struct {
	PduSessionID uint8
	Transfer     []byte
}

type RANSessions struct {
	Present       []RANSession
	Rejected      []uint8
	Authoritative bool
}

type AppliedSession struct {
	PduSessionID uint8
	Transfer     []byte
}

type RANSessionResult struct {
	Applied     []AppliedSession
	Failed      []uint8
	Deactivated []uint8
}

// ReconcileSessionsToRAN converges the PDU sessions of ue onto ueConn, the
// UE-associated logical NG-connection whose AN resources the caller just negotiated,
// and records the resulting user-plane state on it.
func (a *AMF) ReconcileSessionsToRAN(
	ctx context.Context,
	ue *UeContext,
	ueConn *UeConn,
	want RANSessions,
	apply func(ctx context.Context, ref string, transfer []byte) ([]byte, error),
) RANSessionResult {
	var result RANSessionResult

	if ue == nil || ueConn == nil {
		return result
	}

	named := make(map[uint8]struct{}, len(want.Present)+len(want.Rejected))

	for _, s := range want.Present {
		named[s.PduSessionID] = struct{}{}

		smContext, ok := ue.SmContextFindByPDUSessionID(s.PduSessionID)
		if !ok {
			logger.From(ctx, logger.AmfLog).Warn("RAN reports a PDU session the core does not know; not switched",
				logger.PDUSessionID(s.PduSessionID))

			result.Failed = append(result.Failed, s.PduSessionID)

			continue
		}

		n2Rsp, err := apply(ctx, smContext.Ref, s.Transfer)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Error("failed to converge a PDU session onto the RAN endpoint",
				logger.SUPI(ue.Supi().String()), logger.SMContextRef(smContext.Ref),
				logger.PDUSessionID(s.PduSessionID), zap.Error(err))

			result.Failed = append(result.Failed, s.PduSessionID)

			a.deactivateSession(ctx, ueConn, smContext.Ref, s.PduSessionID)
			result.Deactivated = append(result.Deactivated, s.PduSessionID)

			continue
		}

		ueConn.SetN2SessionActive(s.PduSessionID)

		result.Applied = append(result.Applied, AppliedSession{PduSessionID: s.PduSessionID, Transfer: n2Rsp})
	}

	for _, id := range want.Rejected {
		named[id] = struct{}{}

		smContext, ok := ue.SmContextFindByPDUSessionID(id)
		if !ok {
			continue
		}

		a.deactivateSession(ctx, ueConn, smContext.Ref, id)
		result.Deactivated = append(result.Deactivated, id)
	}

	if want.Authoritative {
		for _, sr := range ue.SmContextRefs() {
			if sr.Ref == "" {
				continue
			}

			if _, ok := named[sr.PduSessionID]; ok {
				continue
			}

			if ueConn.N2SessionInactive(sr.PduSessionID) {
				continue
			}

			logger.From(ctx, logger.AmfLog).Info("deactivating a PDU session the RAN did not report",
				logger.SUPI(ue.Supi().String()), logger.PDUSessionID(sr.PduSessionID))

			a.deactivateSession(ctx, ueConn, sr.Ref, sr.PduSessionID)
			result.Deactivated = append(result.Deactivated, sr.PduSessionID)
		}
	}

	return result
}

func (a *AMF) deactivateSession(ctx context.Context, ueConn *UeConn, ref string, pduSessionID uint8) {
	if err := a.Session.DeactivateSmContext(ctx, ref); err != nil {
		logger.From(ctx, logger.AmfLog).Error("failed to deactivate a PDU session",
			logger.SMContextRef(ref), logger.PDUSessionID(pduSessionID), zap.Error(err))

		return
	}

	ueConn.SetN2SessionInactive(pduSessionID)
}

// ReconcileSessionsForUE re-evaluates every PDU session of a UE against the
// current DB policy and applies any change (UPF, gNB, and UE) via the SMF.
func (amf *AMF) ReconcileSessionsForUE(ctx context.Context, ue *UeContext) {
	if ue == nil {
		return
	}

	ue.mu.Lock()
	smContextRefs := make([]string, 0, len(ue.SmContextList))

	for _, smCtx := range ue.SmContextList {
		smContextRefs = append(smContextRefs, smCtx.Ref)
	}

	ue.mu.Unlock()

	for _, ref := range smContextRefs {
		if ref == "" {
			continue
		}

		if err := amf.Session.ReconcileSession(ctx, ref); err != nil {
			logger.AmfLog.Warn("session reconcile failed",
				logger.SMContextRef(ref),
				zap.Error(err))
		}
	}
}

func (amf *AMF) RefreshUEAMBRs(ctx context.Context) {
	amf.mu.RLock()
	ues := make([]*UeContext, 0, len(amf.UEs))

	for _, ue := range amf.UEs {
		if ue.State() == Registered {
			ues = append(ues, ue)
		}
	}

	amf.mu.RUnlock()

	for _, ue := range ues {
		profile, err := amf.SubscriberProfile(ctx, ue.Supi())
		if err != nil || profile.Ambr == nil {
			logger.AmfLog.Warn("failed to read the subscribed UE-AMBR", logger.SUPI(ue.Supi().String()), zap.Error(err))
			continue
		}

		if current := ue.Ambr(); current != nil && *current == *profile.Ambr {
			continue
		}

		ue.SetAmbr(profile.Ambr)

		ueConn := ue.Conn()
		if ueConn == nil || ueConn.ICS() != ICSCompleted {
			continue
		}

		if err := ueConn.SendUEContextModification(ctx, *profile.Ambr); err != nil {
			logger.AmfLog.Warn("failed to signal the UE-AMBR", logger.SUPI(ue.Supi().String()), zap.Error(err))
		}
	}
}
