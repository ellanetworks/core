// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// handleBearerResourceAllocationRequest always rejects: the bearer QoS is
// network-determined, not UE-modifiable (TS 24.301 §6.5.3).
func handleBearerResourceAllocationRequest(ctx context.Context, ue *mme.UeContext, ueConn *mme.UeConn, req *eps.BearerResourceAllocationRequest) nasreply.Disposition {
	pti := req.PTI

	cause := esmRequestHeaderCause(uint8(pti), uint8(req.EPSBearerIdentity))
	if cause == 0 {
		cause = eps.ESMCauseRequestRejectedUnspecified
	}

	logger.From(ctx, logger.MmeLog).Info("bearer resource allocation rejected", zap.String("imsi", ue.IMSI()), zap.Uint8("pti", uint8(pti)), zap.Stringer("esm-cause", cause))
	rejectBearerResourceAllocation(ctx, ueConn, uint8(pti), cause)

	return nasreply.Handled()
}

const tftOperationNoOperation uint8 = 0x06

func requestsBearerResources(req *eps.BearerResourceModificationRequest) bool {
	if req.RequiredTrafficFlowQoS != nil {
		return true
	}

	if len(req.TrafficFlowAggregate) == 0 {
		return false
	}

	return req.TrafficFlowAggregate[0]>>5 != tftOperationNoOperation
}

func handleBearerResourceModificationRequest(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, req *eps.BearerResourceModificationRequest) nasreply.Disposition {
	pti := req.PTI

	if cause := esmRequestHeaderCause(uint8(pti), uint8(req.EPSBearerIdentity)); cause != 0 {
		logger.From(ctx, logger.MmeLog).Info("bearer resource modification rejected",
			zap.String("imsi", ue.IMSI()), zap.Uint8("pti", uint8(pti)), zap.Stringer("esm-cause", cause))
		rejectBearerResourceModification(ctx, ueConn, uint8(pti), cause)

		return nasreply.Handled()
	}

	if requestsBearerResources(req) {
		cause := eps.ESMCauseEPSQoSNotAccepted

		logger.From(ctx, logger.MmeLog).Info("bearer resource modification rejected",
			zap.String("imsi", ue.IMSI()), zap.Uint8("pti", uint8(pti)), zap.Stringer("esm-cause", cause))
		rejectBearerResourceModification(ctx, ueConn, uint8(pti), cause)

		return nasreply.Handled()
	}

	if !acceptUnchangedBearerModification(ctx, m, ue, ueConn, uint8(req.EPSBearerIdentityForPacketFilter), uint8(pti)) {
		rejectBearerResourceModification(ctx, ueConn, uint8(pti), eps.ESMCauseInvalidEPSBearerIdentity)
	}

	return nasreply.Handled()
}

func acceptUnchangedBearerModification(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, ebi, pti uint8) bool {
	p := m.LookupPDN(ue, ebi)
	if p == nil {
		return false
	}

	if !ue.BeginUnchangedBearerModification(p) {
		return false
	}

	plain, err := (&eps.ModifyEPSBearerContextRequest{
		EPSBearerIdentity: eps.EPSBearerIdentity(ebi),
		PTI:               nas.ProcedureTransactionIdentity(pti),
	}).MarshalBinary()
	if err != nil {
		ue.ClearPendingModify(p)

		logger.From(ctx, logger.MmeLog).Error("failed to build Modify EPS Bearer Context Request",
			zap.String("imsi", ue.IMSI()), zap.Error(err))

		return false
	}

	write := func(wire []byte) error {
		ueConn.SendDownlinkNASTransport(ctx, wire)

		return nil
	}

	if err := ueConn.SendProtected(plain, eps.SHTIntegrityProtectedCiphered, write); err != nil {
		ue.ClearPendingModify(p)

		mme.ReportProtectFailure(ctx, ueConn, "Modify EPS Bearer Context Request", err)

		return false
	}

	m.ArmESMGuardAbortOnly(ue, p, "Modify EPS Bearer Context Request", plain, eps.SHTIntegrityProtectedCiphered, func() {
		ue.ClearPendingModify(p)
	})

	logger.From(ctx, logger.MmeLog).Info("accepted a UE requested bearer resource modification that asked for no bearer resources",
		zap.String("imsi", ue.IMSI()), zap.Uint8("pti", pti), zap.String("apn", p.Apn))

	return true
}

func rejectBearerResourceAllocation(ctx context.Context, ueConn *mme.UeConn, pti uint8, cause eps.ESMCause) {
	ueConn.SendDownlinkProtected(ctx, &eps.BearerResourceAllocationReject{
		PTI:   nas.ProcedureTransactionIdentity(pti),
		Cause: cause,
	})
}

func rejectBearerResourceModification(ctx context.Context, ueConn *mme.UeConn, pti uint8, cause eps.ESMCause) {
	ueConn.SendDownlinkProtected(ctx, &eps.BearerResourceModificationReject{
		PTI:   nas.ProcedureTransactionIdentity(pti),
		Cause: cause,
	})
}
