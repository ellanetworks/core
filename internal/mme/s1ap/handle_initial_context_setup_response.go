// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"net/netip"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// enbTransportAddress resolves the eNB S1-U endpoint from an E-RAB Transport
// Layer Address (TS 36.413): IPv4 (4 octets), IPv6 (16), or dual-stack (20),
// preferring IPv6. It reports false when no address is present.
func enbTransportAddress(tla s1ap.TransportLayerAddress) (netip.Addr, bool) {
	v4, v6 := tla.IPs()

	switch {
	case v6.IsValid():
		return v6.Unmap(), true
	case v4.IsValid():
		return v4.Unmap(), true
	default:
		return netip.Addr{}, false
	}
}

// handleInitialContextSetupResponse records the eNB's bearer-setup result
// (TS 36.413): each confirmed eNB S1-U F-TEID becomes the session's downlink
// endpoint at the anchor; any bearer the eNB could not set up is torn down.
func handleInitialContextSetupResponse(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseInitialContextSetupResponse(value)
	if err != nil {
		handleParseError(m, radio.Conn, s1ap.ProcInitialContextSetup, err)
		return
	}

	ue, ueConn, ok := resolveUEIDs(m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	reportDiagnostics(m, ctx, radio.Conn, s1ap.ProcInitialContextSetup, s1ap.TriggeringSuccessfulOutcome, ueAssociated(ueConn.MMEUES1APID, ueConn.ENBUES1APID), msg.Diagnostics())

	mmeUEID := *msg.MMEUES1APID

	ue.TouchLastSeen()

	// Not authoritative: an INITIAL CONTEXT SETUP RESPONSE reports the E-RABs this
	// procedure set up and says nothing about any other bearer (TS 36.413 §8.3.1.2).
	result := m.ReconcileBearersToRAN(ctx, ue, mme.RANBearers{
		Present:  setupBearers(ctx, mmeUEID, ctxtSetupBearers(msg.ERABSetup)),
		Rejected: failedERABIDs(msg.ERABFailedToSetup),
	})

	setup := len(result.Applied)

	logger.From(ctx, logger.MmeLog).Info("Initial Context Setup Response",
		zap.Uint32("mme-ue-id", uint32(mmeUEID)),
		zap.Int("e-rabs-setup", setup),
		zap.Int("e-rabs-released", len(result.Released)))

	if setup == 0 {
		return
	}

	if ueConn != nil {
		ueConn.ICS = mme.ICSCompleted
	}

	// With the radio bearers up, a pending data-network change becomes deliverable.
	// ReconcileUE returns early for a UE not yet EMM-REGISTERED (attach).
	m.ReconcileUE(ctx, ue)
}

// setupBearer is the part of an E-RAB setup item the core needs. S1AP spells the item
// differently per message (ERABSetupItemCtxtSURes, ERABSetupItemBearerSURes) with the
// same three fields, so each caller projects onto this.
type setupBearer struct {
	ERABID                s1ap.ERABID
	TransportLayerAddress s1ap.TransportLayerAddress
	GTPTEID               s1ap.GTPTEID
}

// setupBearers decodes an E-RAB setup list into the RAN bearer set the reconciliation
// primitive consumes. An item whose transport address will not decode names no usable
// downlink and is dropped; it is absent from the applied set the caller reports.
func setupBearers(ctx context.Context, mmeUEID s1ap.MMEUES1APID, items []setupBearer) []mme.RANBearer {
	present := make([]mme.RANBearer, 0, len(items))

	for _, erab := range items {
		addr, ok := enbTransportAddress(erab.TransportLayerAddress)
		if !ok {
			logger.From(ctx, logger.MmeLog).Warn("E-RAB setup item with an invalid eNB transport address",
				zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.Uint8("e-rab-id", uint8(erab.ERABID)))

			continue
		}

		present = append(present, mme.RANBearer{
			Ebi:      uint8(erab.ERABID),
			EnbFTEID: models.FTEID{TEID: uint32(erab.GTPTEID), Addr: addr},
		})
	}

	return present
}

// failedERABIDs is the EPS bearer identities of an E-RAB failed-to-setup list.
func failedERABIDs(items []s1ap.ERABItem) []uint8 {
	if len(items) == 0 {
		return nil
	}

	out := make([]uint8, 0, len(items))
	for _, erab := range items {
		out = append(out, uint8(erab.ERABID))
	}

	return out
}

// ctxtSetupBearers projects an INITIAL CONTEXT SETUP RESPONSE setup list.
func ctxtSetupBearers(items []s1ap.ERABSetupItemCtxtSURes) []setupBearer {
	out := make([]setupBearer, 0, len(items))
	for _, e := range items {
		out = append(out, setupBearer{ERABID: e.ERABID, TransportLayerAddress: e.TransportLayerAddress, GTPTEID: e.GTPTEID})
	}

	return out
}
