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
func handleBearerResourceAllocationRequest(ctx context.Context, ue *mme.UeContext, req *eps.BearerResourceAllocationRequest) nasreply.Disposition {
	pti := req.PTI

	cause := esmRequestHeaderCause(uint8(pti), uint8(req.EPSBearerIdentity))
	if cause == 0 {
		cause = eps.ESMCauseRequestRejectedUnspecified
	}

	logger.From(ctx, logger.MmeLog).Info("bearer resource allocation rejected", zap.String("imsi", ue.IMSI()), zap.Uint8("pti", uint8(pti)), zap.Stringer("esm-cause", cause))
	rejectBearerResourceAllocation(ctx, ue, uint8(pti), cause)

	return nasreply.Handled()
}

// handleBearerResourceModificationRequest always rejects: the bearer QoS is
// network-determined, not UE-modifiable (TS 24.301 §6.5.4).
func handleBearerResourceModificationRequest(ctx context.Context, ue *mme.UeContext, req *eps.BearerResourceModificationRequest) nasreply.Disposition {
	pti := req.PTI

	cause := esmRequestHeaderCause(uint8(pti), uint8(req.EPSBearerIdentity))
	if cause == 0 {
		cause = eps.ESMCauseRequestRejectedUnspecified
	}

	logger.From(ctx, logger.MmeLog).Info("bearer resource modification rejected",
		zap.String("imsi", ue.IMSI()), zap.Uint8("pti", uint8(pti)), zap.Stringer("esm-cause", cause))
	rejectBearerResourceModification(ctx, ue, uint8(pti), cause)

	return nasreply.Handled()
}

func rejectBearerResourceAllocation(ctx context.Context, ue *mme.UeContext, pti uint8, cause eps.ESMCause) {
	ue.Conn().SendDownlinkProtected(ctx, &eps.BearerResourceAllocationReject{
		PTI:   nas.ProcedureTransactionIdentity(pti),
		Cause: cause,
	})
}

func rejectBearerResourceModification(ctx context.Context, ue *mme.UeContext, pti uint8, cause eps.ESMCause) {
	ue.Conn().SendDownlinkProtected(ctx, &eps.BearerResourceModificationReject{
		PTI:   nas.ProcedureTransactionIdentity(pti),
		Cause: cause,
	})
}
