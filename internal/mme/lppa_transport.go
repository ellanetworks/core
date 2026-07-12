// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/s1ap"
)

// SendDownlinkLPPaTransport builds a DOWNLINK UE ASSOCIATED LPPA TRANSPORT
// carrying the LMF's LPPa PDU and sends it to this UE's eNB (TS 36.413 §8.14).
// Delivery is best-effort, logged at the S1AP send chokepoint like every MME
// downlink; the returned error reports a build failure only.
func (c *UeConn) SendDownlinkLPPaTransport(ctx context.Context, routingID uint8, pdu []byte) error {
	if c == nil {
		return fmt.Errorf("nil connection")
	}

	msg := &s1ap.DownlinkUEAssociatedLPPaTransport{
		MMEUES1APID: c.MMEUES1APID,
		ENBUES1APID: c.ENBUES1APID,
		RoutingID:   s1ap.RoutingID(routingID),
		LPPaPDU:     pdu,
	}

	b, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("build downlink LPPa transport: %w", err)
	}

	c.SendS1AP(ctx, S1APProcedureDownlinkUEAssociatedLPPaTransport, b)

	return nil
}
