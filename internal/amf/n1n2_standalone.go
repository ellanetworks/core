// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
)

// n1PayloadContainers maps an N1 message class to the NAS payload container that carries
// it (TS 24.501 §9.11.3.40). The SM class is absent: it is PDU-session scoped and
// delivered with the session's resources, not standalone.
var n1PayloadContainers = map[models.N1MessageClass]fgs.PayloadContainerType{
	models.N1ClassLPP: fgs.PayloadContainerTypeLPP,
}

// DeliverStandaloneN1N2 delivers a request that is not PDU-session scoped: the N1 message
// to the UE in a DL NAS Transport, the N2 information to the serving RAN node by the NGAP
// procedure its class selects.
func DeliverStandaloneN1N2(ctx context.Context, ue *UeContext, conn *UeConn, req *models.N1N2MessageTransferRequest) error {
	if ue == nil || conn == nil || req == nil {
		return fmt.Errorf("nil ue, connection or request")
	}

	if req.BinaryDataN1Message != nil {
		container, ok := n1PayloadContainers[req.N1Class]
		if !ok {
			return fmt.Errorf("no NAS payload container for N1 class %q", req.N1Class)
		}

		// The Additional information IE carries the class's correlation identifier, which
		// the UE echoes back so the uplink routes to the right consumer.
		nasPdu, err := BuildDLNASTransport(container, req.BinaryDataN1Message, nil, nil, req.LCSCorrelationID)
		if err != nil {
			return fmt.Errorf("build DL NAS Transport (%s): %w", req.N1Class, err)
		}

		if err := ue.SendDownlinkNAS(nasPdu, uint8(fgs.SHTIntegrityProtectedCiphered), func(wire []byte) error {
			if err := conn.SendDownlinkNASTransport(ctx, wire); err != nil {
				return fmt.Errorf("send DL NAS Transport (%s): %w", req.N1Class, err)
			}

			logger.From(ctx, logger.AmfLog).Info("sent downlink nas transport to UE", logger.SUPI(ue.Supi().String()))

			return nil
		}); err != nil {
			return err
		}
	}

	if req.BinaryDataN2Information != nil {
		switch req.N2Class {
		case models.N2ClassNRPPa:
			if err := conn.SendDownlinkNRPPaTransport(ctx, req.RoutingID, req.BinaryDataN2Information); err != nil {
				return fmt.Errorf("send downlink NRPPa transport: %w", err)
			}
		default:
			return fmt.Errorf("no NGAP transport for N2 class %q", req.N2Class)
		}
	}

	return nil
}
