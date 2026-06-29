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
// Layer Address (TS 36.413): IPv4 (4 octets), IPv6 (16), or dual-stack (20). When
// the eNB advertises both families the IPv6 endpoint is used. It reports false
// when no address is present.
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
// (TS 36.413): the eNB S1-U F-TEID it returns is handed to the anchor
// as the session's downlink endpoint.
func handleInitialContextSetupResponse(m *mme.MME, ctx context.Context, conn mme.NasWriter, value []byte) {
	msg, err := s1ap.ParseInitialContextSetupResponse(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Initial Context Setup Response", zap.Error(err))
		return
	}

	ue, ok := resolveUE(m, conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	if len(msg.ERABSetup) == 0 {
		logger.MmeLog.Warn("Initial Context Setup Response without an E-RAB",
			zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)))

		return
	}

	// The eNB returns its S1-U F-TEID (the downlink endpoint); hand it to the
	// anchor so the UPF encapsulates downlink traffic toward the eNB.
	erab := msg.ERABSetup[0]

	enbAddr, ok := enbTransportAddress(erab.TransportLayerAddress)
	if !ok {
		logger.MmeLog.Warn("Initial Context Setup Response with an invalid eNB transport address",
			zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)))

		return
	}

	p := m.LookupPDN(ue, uint8(erab.ERABID))
	if p == nil {
		logger.MmeLog.Warn("Initial Context Setup Response for an unknown E-RAB",
			zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)), zap.Int("erab-id", int(erab.ERABID)))

		return
	}

	p.EnbFTEID = models.FTEID{TEID: uint32(erab.GTPTEID), Addr: enbAddr}

	if ue.S1 != nil {
		ue.S1.BearersUp = true
	}

	if err := m.Session.ModifyEPSSession(ctx, ue.IMSI(), p.Ebi, p.EnbFTEID); err != nil {
		logger.MmeLog.Error("failed to set the eNB F-TEID on the EPS session",
			zap.String("imsi", ue.IMSI()), zap.Error(err))

		return
	}

	logger.MmeLog.Info("Initial Context Setup Response",
		zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)),
		zap.String("enb-s1u", p.EnbFTEID.Addr.String()),
	)

	// Deliver any pending data-network change to a UE that just re-established
	// its bearer from ECM-IDLE (Service Request): the radio bearer is now up, so
	// a modify/reactivate is deliverable. During attach this is a no-op — the UE
	// is not yet EMM-REGISTERED — so ReconcileUE returns early.
	m.ReconcileUE(ctx, ue)
}
