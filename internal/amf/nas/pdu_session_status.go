// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func syncPDUSessionStatus(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, req *fgs.RegistrationRequest) *[16]bool {
	if req == nil || req.PDUSessionStatus == nil {
		return nil
	}

	reported := req.PDUSessionStatus.PSI
	held := new([16]bool)

	for psi := 1; psi <= 15; psi++ {
		smContext, ok := ue.SmContextFindByPDUSessionID(uint8(psi))
		if !ok {
			continue
		}

		if reported[psi] {
			held[psi] = true
			continue
		}

		if err := amfInstance.Session.ReleaseSmContext(ctx, smContext.Ref); err != nil {
			logger.From(ctx, logger.AmfLog).Error("release PDU session the UE reports inactive",
				zap.Error(err), zap.Int("pdu_session_id", psi))
		}

		ue.DeleteSmContext(uint8(psi))
	}

	return held
}
