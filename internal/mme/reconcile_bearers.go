// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

// RANBearer is one E-RAB the eNB reports it holds, with the downlink endpoint the
// core must forward to.
type RANBearer struct {
	Ebi      uint8
	EnbFTEID models.FTEID
}

// RANBearers is an eNB's view of a UE's bearer set, as carried by one S1AP message.
//
// Authoritative distinguishes the two kinds of message 3GPP defines. A PATH SWITCH
// REQUEST or a completed handover names *everything* the eNB holds, so an E-RAB in
// the UE context that appears in neither list has been implicitly released by the
// eNB (TS 36.413 §8.4.4.2, TS 23.401 §5.5.1.1.2 step 2). An E-RAB SETUP RESPONSE
// names only the bearers its own procedure set up and says nothing about the rest.
// Treating the first kind as if it were the second silently strands bearers whose
// downlink still points at the source eNB.
type RANBearers struct {
	Present       []RANBearer
	Rejected      []uint8
	Authoritative bool
}

// RANBearerResult reports the outcome per E-RAB. Failed is what the caller reports
// back to the eNB — in the PATH SWITCH REQUEST ACKNOWLEDGE E-RAB To Be Released
// List, for instance — so it releases the matching data radio bearers.
type RANBearerResult struct {
	Applied  []uint8
	Failed   []uint8
	Released []uint8
}

// ReconcileBearersToRAN converges a UE's PDN connections onto the bearer set an eNB
// reports, and is the single place that does so: every S1AP message that carries a
// bearer set drives this, so the ordering, locking and cleanup rules below hold once
// rather than per handler.
//
//   - a bearer the eNB holds has its anchor downlink switched first and its stored
//     endpoint written second, under the UE lock, only on success — so the context
//     never claims a path the anchor does not have;
//   - a bearer the eNB rejected, or that the core could not converge, has its
//     core-network resources released (TS 23.401 §5.5.1.1.2 step 6) rather than only
//     being reported;
//   - on an authoritative message, a bearer absent from both lists is released too;
//   - the default bearer is re-elected over the survivors, so a UE never keeps
//     bearers with no default among them.
//
// The caller keeps what is procedure-specific: rejecting duplicate E-RAB IDs with
// the right cause, choosing the response message, and deciding what a total failure
// means.
func (m *MME) ReconcileBearersToRAN(ctx context.Context, ue *UeContext, want RANBearers) RANBearerResult {
	var result RANBearerResult

	if ue == nil {
		return result
	}

	// Everything the eNB named, so an authoritative message can tell which of the
	// UE's remaining bearers it left out.
	named := make(map[uint8]struct{}, len(want.Present)+len(want.Rejected))

	for _, b := range want.Present {
		named[b.Ebi] = struct{}{}

		p := m.LookupPDN(ue, b.Ebi)
		if p == nil {
			// The eNB holds a bearer the core has no context for; it must drop it.
			logger.From(ctx, logger.MmeLog).Warn("RAN reports an E-RAB the core does not know; not switched",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", b.Ebi))

			result.Failed = append(result.Failed, b.Ebi)

			continue
		}

		if err := m.Session.ModifyEPSSession(ctx, ue.IMSI(), b.Ebi, b.EnbFTEID); err != nil {
			logger.From(ctx, logger.MmeLog).Error("failed to switch an EPS session downlink to the RAN endpoint",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", b.Ebi), zap.Error(err))

			// The bearer has no usable downlink: free its core resources rather than
			// leaving a session pointed at an eNB the UE has left.
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

	m.ensureDefaultBearer(ue)

	return result
}

// ensureDefaultBearer re-elects the default EPS bearer when the previous one is
// gone, promoting the lowest surviving EBI. Without it a UE can hold bearers with
// DefaultEBI zeroed, which silently makes every bearer look releasable on its own
// and stops a last-bearer release from ever detaching the UE (TS 24.301).
func (m *MME) ensureDefaultBearer(ue *UeContext) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if _, ok := ue.Pdns[ue.DefaultEBI]; ok && ue.DefaultEBI != 0 {
		return
	}

	ue.DefaultEBI = 0

	for ebi := range ue.Pdns {
		if ue.DefaultEBI == 0 || ebi < ue.DefaultEBI {
			ue.DefaultEBI = ebi
		}
	}
}

// DuplicateEBI reports the first EPS bearer identity that appears more than once.
// Several S1AP procedures make a repeated E-RAB ID an abnormal condition but answer
// it differently, so detection is shared and the response stays with the caller
// (TS 36.413 §8.4.4.4, §8.2.4.4).
func DuplicateEBI(ebis []uint8) (uint8, bool) {
	seen := make(map[uint8]struct{}, len(ebis))

	for _, ebi := range ebis {
		if _, ok := seen[ebi]; ok {
			return ebi, true
		}

		seen[ebi] = struct{}{}
	}

	return 0, false
}
