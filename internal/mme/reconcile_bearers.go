// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"cmp"
	"context"
	"slices"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

type RANBearer struct {
	Ebi      uint8
	EnbFTEID models.FTEID
}

type RANBearers struct {
	Present        []RANBearer
	Rejected       []uint8
	Authoritative  bool
	ReleaseUnknown bool
	ReleaseFailed  bool
	AfterCommit    bool
}

type RANBearerResult struct {
	Applied  []uint8
	Failed   []uint8
	Released []uint8

	followUps []func(context.Context)
}

func (r *RANBearerResult) Complete(ctx context.Context) {
	followUps := r.followUps
	r.followUps = nil

	for _, f := range followUps {
		f(ctx)
	}
}

func (r *RANBearerResult) AppliedDefaultBearer(m *MME, ue *UeContext) bool {
	return slices.ContainsFunc(r.Applied, func(ebi uint8) bool { return m.LookupPDN(ue, ebi) != nil })
}

func (m *MME) ReconcileBearersToRAN(ctx context.Context, ue *UeContext, want RANBearers) RANBearerResult {
	var result RANBearerResult

	if ue == nil {
		return result
	}

	later := func(f func(context.Context)) { result.followUps = append(result.followUps, f) }

	named := make(map[uint8]struct{}, len(want.Present)+len(want.Rejected))
	failedPDNs := make(map[uint8]bool)

	present := slices.Clone(want.Present)
	slices.SortStableFunc(present, func(a, b RANBearer) int {
		return cmp.Compare(boolRank(m.LookupPDN(ue, a.Ebi) == nil), boolRank(m.LookupPDN(ue, b.Ebi) == nil))
	})

	for _, b := range present {
		named[b.Ebi] = struct{}{}

		p := m.LookupPDN(ue, b.Ebi)
		if p == nil {
			if dp, d := m.LookupDedicated(ue, b.Ebi); d != nil {
				switch {
				case failedPDNs[dp.Ebi]:
					result.Failed = append(result.Failed, b.Ebi)
				case m.dedicatedRadioUp(ctx, ue, dp, d, b.EnbFTEID, want.ReleaseFailed):
					result.Applied = append(result.Applied, b.Ebi)
				default:
					result.Failed = append(result.Failed, b.Ebi)
				}

				continue
			}

			logger.From(ctx, logger.MmeLog).Warn("RAN reports an E-RAB the core does not know; not switched",
				logger.SUPI(ue.Supi().String()), logger.ERABID(b.Ebi))

			if want.ReleaseUnknown {
				ebi := b.Ebi

				later(func(ctx context.Context) {
					if ueConn := ue.Conn(); ueConn != nil {
						m.sendDedicatedERABRelease(ctx, ueConn, ebi, nil)
					}
				})
			}

			result.Failed = append(result.Failed, b.Ebi)

			continue
		}

		if err := m.Session.ModifyEPSSession(ctx, p.SessionRef, b.Ebi, b.EnbFTEID); err != nil {
			logger.From(ctx, logger.MmeLog).Error("failed to switch an EPS session downlink to the RAN endpoint",
				logger.SUPI(ue.Supi().String()), logger.ERABID(b.Ebi), zap.Error(err))

			failedPDNs[p.Ebi] = true

			later(func(ctx context.Context) { m.DisconnectLostPDN(ctx, ue, p) })

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
			later(func(ctx context.Context) { m.DisconnectLostPDN(ctx, ue, p) })

			result.Released = append(result.Released, ebi)
		} else if _, d := m.LookupDedicated(ue, ebi); d != nil {
			later(func(ctx context.Context) { m.FailDedicatedBearer(ctx, ue, ebi, "the eNB did not set up the E-RAB") })

			result.Released = append(result.Released, ebi)
		}
	}

	if want.Authoritative {
		for _, p := range m.SnapshotPDNs(ue) {
			if _, ok := named[p.Ebi]; ok {
				continue
			}

			logger.From(ctx, logger.MmeLog).Info("the RAN did not report a default bearer's E-RAB; disconnecting its PDN connection",
				logger.SUPI(ue.Supi().String()), logger.ERABID(p.Ebi))

			later(func(ctx context.Context) { m.DisconnectLostPDN(ctx, ue, p) })

			result.Released = append(result.Released, p.Ebi)
		}

		for _, d := range m.SnapshotDedicated(ue) {
			if _, ok := named[d.Ebi]; ok {
				continue
			}

			ebi := d.Ebi

			later(func(ctx context.Context) { m.FailDedicatedBearer(ctx, ue, ebi, "the RAN did not report the E-RAB") })

			result.Released = append(result.Released, ebi)
		}
	}

	if !want.AfterCommit {
		result.Complete(ctx)
	}

	return result
}

func boolRank(b bool) int {
	if b {
		return 1
	}

	return 0
}

func (m *MME) DisconnectLostPDN(ctx context.Context, ue *UeContext, p *PdnConnection) {
	if m.LookupPDN(ue, p.Ebi) != p {
		return
	}

	if !ue.hasOtherPDN(p) {
		m.DeactivateBearerLocally(ctx, ue, p)
		return
	}

	m.DisconnectBearer(ctx, ue, p, eps.ESMCauseRegularDeactivation, 0)

	if m.LookupPDN(ue, p.Ebi) == p && !ue.BearerDeactivating(p) {
		m.ReleasePDN(ctx, ue, p)
	}
}

func (ue *UeContext) hasOtherPDN(p *PdnConnection) bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	for ebi := range ue.Pdns {
		if ebi != p.Ebi {
			return true
		}
	}

	return false
}
