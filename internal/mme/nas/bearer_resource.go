// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// handleBearerResourceAllocationRequest refuses a UE-requested dedicated-bearer
// allocation (TS 24.301 §6.5.3): the bearer QoS is network-determined and not
// modifiable on UE request, so the request is rejected.
func handleBearerResourceAllocationRequest(ctx context.Context, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	req, err := eps.ParseBearerResourceAllocationRequest(plain)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to decode Bearer Resource Allocation Request", zap.Error(err))
		return nasreply.Handled()
	}

	pti := req.ProcedureTransactionIdentity

	cause := esmRequestHeaderCause(pti, req.EPSBearerIdentity)
	if cause == 0 {
		cause = esmCauseRequestRejectedUnspecified
	}

	logger.From(ctx, logger.MmeLog).Info("bearer resource allocation rejected",
		zap.String("imsi", ue.IMSI()), zap.Uint8("pti", pti), zap.Uint8("esm-cause", cause))
	rejectBearerResourceAllocation(ctx, ue, pti, cause)

	return nasreply.Handled()
}

// handleBearerResourceModificationRequest refuses a UE-requested dedicated-bearer
// modification (TS 24.301 §6.5.4): the bearer QoS is network-determined and not
// modifiable on UE request, so the request is rejected.
func handleBearerResourceModificationRequest(ctx context.Context, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	req, err := eps.ParseBearerResourceModificationRequest(plain)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to decode Bearer Resource Modification Request", zap.Error(err))
		return nasreply.Handled()
	}

	pti := req.ProcedureTransactionIdentity

	cause := esmRequestHeaderCause(pti, req.EPSBearerIdentity)
	if cause == 0 {
		cause = esmCauseRequestRejectedUnspecified
	}

	logger.From(ctx, logger.MmeLog).Info("bearer resource modification rejected",
		zap.String("imsi", ue.IMSI()), zap.Uint8("pti", pti), zap.Uint8("esm-cause", cause))
	rejectBearerResourceModification(ctx, ue, pti, cause)

	return nasreply.Handled()
}

// rejectBearerResourceAllocation refuses a BEARER RESOURCE ALLOCATION REQUEST with
// an ESM cause (TS 24.301 §6.5.3.4).
func rejectBearerResourceAllocation(ctx context.Context, ue *mme.UeContext, pti, cause uint8) {
	ue.Conn().SendDownlinkProtected(ctx, &eps.BearerResourceAllocationReject{
		ProcedureTransactionIdentity: pti,
		ESMCause:                     cause,
	})
}

// rejectBearerResourceModification refuses a BEARER RESOURCE MODIFICATION REQUEST
// with an ESM cause (TS 24.301 §6.5.4.4).
func rejectBearerResourceModification(ctx context.Context, ue *mme.UeContext, pti, cause uint8) {
	ue.Conn().SendDownlinkProtected(ctx, &eps.BearerResourceModificationReject{
		ProcedureTransactionIdentity: pti,
		ESMCause:                     cause,
	})
}
