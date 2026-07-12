// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"bytes"
	"context"
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

func TestSendDownlinkLPPaTransport(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	c := m.NewUeConn(cc, 7)

	lppaPDU := []byte{0x00, 0x05, 0xab, 0xcd, 0xef}

	if err := c.SendDownlinkLPPaTransport(context.Background(), 3, lppaPDU); err != nil {
		t.Fatalf("send: %v", err)
	}

	if cc.count() != 1 {
		t.Fatalf("sent %d messages, want 1", cc.count())
	}

	pdu, err := s1ap.Unmarshal(cc.sent[0])
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcDownlinkUEAssociatedLPPaTransport {
		t.Fatalf("pdu = %T proc = %v, want DownlinkUEAssociatedLPPaTransport", pdu, im.ProcedureCode)
	}

	out, err := s1ap.ParseDownlinkUEAssociatedLPPaTransport(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.MMEUES1APID != c.MMEUES1APID || out.ENBUES1APID != 7 || out.RoutingID != 3 {
		t.Fatalf("ids: mme=%d enb=%d routing=%d", out.MMEUES1APID, out.ENBUES1APID, out.RoutingID)
	}

	if !bytes.Equal(out.LPPaPDU, lppaPDU) {
		t.Fatalf("lppa pdu = %x, want %x", out.LPPaPDU, lppaPDU)
	}
}

func TestSendDownlinkLPPaTransportNilConn(t *testing.T) {
	var c *UeConn

	if err := c.SendDownlinkLPPaTransport(context.Background(), 0, nil); err == nil {
		t.Fatal("expected error on nil connection")
	}
}
