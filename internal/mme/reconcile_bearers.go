// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

type RANBearer struct {
	Ebi      uint8
	EnbFTEID models.FTEID
}

type RANBearers struct {
	Present       []RANBearer
	Rejected      []uint8
	Authoritative bool
}

type RANBearerResult struct {
	Applied  []uint8
	Failed   []uint8
	Released []uint8
}

func (m *MME) ReconcileBearersToRAN(ctx context.Context, ue *UeContext, want RANBearers) RANBearerResult {
	var result RANBearerResult

	if ue == nil {
		return result
	}

	named := make(map[uint8]struct{}, len(want.Present)+len(want.Rejected))

	for _, b := range want.Present {
		named[b.Ebi] = struct{}{}

		p := m.LookupPDN(ue, b.Ebi)
		if p == nil {
			logger.From(ctx, logger.MmeLog).Warn("RAN reports an E-RAB the core does not know; not switched",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", b.Ebi))

			result.Failed = append(result.Failed, b.Ebi)

			continue
		}

		if err := m.Session.ModifyEPSSession(ctx, p.SessionRef, b.Ebi, b.EnbFTEID); err != nil {
			logger.From(ctx, logger.MmeLog).Error("failed to switch an EPS session downlink to the RAN endpoint",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", b.Ebi), zap.Error(err))

			m.ReleasePDN(ctx, ue, p)

			result.Failed = append(result.Failed, b.Ebi)
			result.Released = append(result.Released, b.Ebi)

			continue
		}

		m.SetPDNEnbFTEID(ue, p, b.EnbFTEID)

		result.Applied = append(result.Applied, b.Ebi)
	}

	for _, ebi := range want.Rejected {
		named[ebi] = struct{}{}

		if p := m.LookupPDN(ue, ebi); p != nil {
			m.ReleasePDN(ctx, ue, p)

			result.Released = append(result.Released, ebi)
		}
	}

	if want.Authoritative {
		for _, p := range m.SnapshotPDNs(ue) {
			if _, ok := named[p.Ebi]; ok {
				continue
			}

			logger.From(ctx, logger.MmeLog).Info("releasing an E-RAB the RAN did not report; implicitly released",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", p.Ebi))

			m.ReleasePDN(ctx, ue, p)
			result.Released = append(result.Released, p.Ebi)
		}
	}

	return result
}
