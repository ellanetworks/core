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
	v4, v6, err := models.DecodeTransportLayerAddress([]byte(tla))
	if err != nil {
		return netip.Addr{}, false
	}

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
		logger.From(ctx, logger.MmeLog).Warn("failed to decode Initial Context Setup Response", zap.Error(err))
		return
	}

	ue, ok := resolveUE(m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	ue.TouchLastSeen()

	// Tear down bearers the eNB failed to set up (TS 36.413 §8.3.1.2).
	for _, erab := range msg.ERABFailedToSetup {
		if p := m.LookupPDN(ue, uint8(erab.ERABID)); p != nil {
			logger.From(ctx, logger.MmeLog).Warn("eNB failed to set up an E-RAB in Initial Context Setup; releasing the PDN connection",
				zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)), zap.Uint8("e-rab-id", uint8(erab.ERABID)))
			m.ReleasePDN(ctx, ue, p)
		}
	}

	if len(msg.ERABSetup) == 0 {
		logger.From(ctx, logger.MmeLog).Warn("Initial Context Setup Response without an E-RAB",
			zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)))

		return
	}

	// Record the eNB S1-U F-TEID for every confirmed E-RAB (TS 36.413). A bad or
	// unknown E-RAB is skipped, not fatal to the rest.
	setup := 0

	for _, erab := range msg.ERABSetup {
		enbAddr, ok := enbTransportAddress(erab.TransportLayerAddress)
		if !ok {
			logger.From(ctx, logger.MmeLog).Warn("Initial Context Setup Response with an invalid eNB transport address",
				zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)), zap.Int("erab-id", int(erab.ERABID)))

			continue
		}

		p := m.LookupPDN(ue, uint8(erab.ERABID))
		if p == nil {
			logger.From(ctx, logger.MmeLog).Warn("Initial Context Setup Response for an unknown E-RAB",
				zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)), zap.Int("erab-id", int(erab.ERABID)))

			continue
		}

		p.EnbFTEID = models.FTEID{TEID: uint32(erab.GTPTEID), Addr: enbAddr}

		if err := m.Session.ModifyEPSSession(ctx, ue.IMSI(), p.Ebi, p.EnbFTEID); err != nil {
			logger.From(ctx, logger.MmeLog).Error("failed to set the eNB F-TEID on the EPS session",
				zap.String("imsi", ue.IMSI()), zap.Int("erab-id", int(erab.ERABID)), zap.Error(err))

			continue
		}

		setup++

		logger.From(ctx, logger.MmeLog).Info("Initial Context Setup Response",
			zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)),
			zap.Int("erab-id", int(erab.ERABID)),
			zap.String("enb-s1u", p.EnbFTEID.Addr.String()),
		)
	}

	if setup == 0 {
		return
	}

	if ue.Conn() != nil {
		ue.Conn().ICS = mme.ICSCompleted
	}

	// With the radio bearers up, a pending data-network change becomes deliverable.
	// ReconcileUE returns early for a UE not yet EMM-REGISTERED (attach).
	m.ReconcileUE(ctx, ue)
}
