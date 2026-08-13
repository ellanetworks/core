// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

func (m *MME) FetchEPSContext(ctx context.Context, oldGUTI eps.GUTI, tau []byte) (interworking.EPSContextResponse, error) {
	if m.FiveGS == nil {
		return interworking.EPSContextResponse{}, ErrNoFiveGSPeer
	}

	if len(tau) == 0 {
		return interworking.EPSContextResponse{}, fmt.Errorf("%w: no message to verify", interworking.ErrIntegrityCheckFailed)
	}

	return m.FiveGS.EPSContext(ctx, interworking.EPSContextRequest{
		Mapped5GGUTI: etsi.MapGUTIEPSTo5G(oldGUTI),
		EPSNAS:       tau,
	})
}

func (m *MME) AckEPSContext(ctx context.Context, supi etsi.SUPI, transferred []uint8) error {
	if m.FiveGS == nil {
		return ErrNoFiveGSPeer
	}

	return m.FiveGS.EPSContextAck(ctx, supi, transferred)
}

func (m *MME) AdoptIdlePDNs(ctx context.Context, ue *UeContext, conns []interworking.PDNConnection) []uint8 {
	transferred := make([]uint8, 0, len(conns))

	for _, c := range conns {
		qos, err := ResolveQoSByAPN(ctx, m, ue.IMSI(), c.APN)
		if err != nil {
			logger.From(ctx, logger.MmeLog).Warn("arriving PDU session has no QoS in the subscriber profile; leaving it behind",
				zap.String("imsi", ue.IMSI()), zap.String("apn", c.APN), zap.Error(err))

			continue
		}

		if !qos.Allow4G {
			logger.From(ctx, logger.MmeLog).Warn("arriving PDU session is not allowed on 4G; leaving it behind",
				zap.String("imsi", ue.IMSI()), zap.String("apn", c.APN))

			continue
		}

		snssai := c.Snssai

		bearer, err := m.Session.TransferIdleToEPS(ctx, ue.Supi(), c.PDUSessionID, c.EPSBearerIdentity, c.APN, &snssai)
		if err != nil {
			logger.From(ctx, logger.MmeLog).Warn("a PDU session could not move onto EPS; leaving it behind",
				zap.Error(err), zap.Uint8("pdu-session-id", c.PDUSessionID), zap.String("apn", c.APN))

			continue
		}

		m.publishRelocatedPDN(ue, c.EPSBearerIdentity, qos, bearer)

		transferred = append(transferred, c.PDUSessionID)
	}

	return transferred
}

// The selection reads netCap so that it matches the one the security mode
// command would negotiate and replay (TS 33.401 §7.2.4.3.2, §7.2.4.4).
func (m *MME) NASAlgorithmsForMappedContext(ctx context.Context, netCap eps.UENetworkCapability, current interworking.EPSNASAlgorithms) (algorithms interworking.EPSNASAlgorithms, changed bool, err error) {
	intOrder, encOrder, err := m.SecurityAlgorithms(ctx)
	if err != nil {
		return interworking.EPSNASAlgorithms{}, false, fmt.Errorf("mme: resolve operator security policy: %w", err)
	}

	eea, eia, ok := eps.SelectNASAlgorithms(netCap, intOrder, encOrder)
	if !ok {
		return interworking.EPSNASAlgorithms{}, false, fmt.Errorf("mme: no NAS security algorithm common to the UE and the operator policy")
	}

	selected := interworking.EPSNASAlgorithms{Ciphering: eea, Integrity: eia}

	return selected, selected != current, nil
}
