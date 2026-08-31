// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

func TestHandleUplinkLPPaTransport(t *testing.T) {
	m := newTestMME(t)
	conn := &captureConn{}
	ue := m.NewUe(conn, 7)
	m.RegisterUEForTest(ue, "001010000000001")

	lppaPDU := []byte{0x00, 0x05, 0xab, 0xcd}

	wire, err := (&s1ap.UplinkUEAssociatedLPPaTransport{
		MMEUES1APID: ue.Conn().MMEUES1APID,
		ENBUES1APID: 7,
		RoutingID:   0,
		LPPaPDU:     lppaPDU,
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handleUplinkLPPaTransport(m, context.Background(), mme.NewRadioForTest(conn), initiatingValue(t, wire))

	msgs := ue.GetLPPaMessages()
	if len(msgs) != 1 || !bytes.Equal(msgs[0].Payload, lppaPDU) {
		t.Fatalf("stored %d messages: %+v", len(msgs), msgs)
	}
}

func TestHandleUplinkLPPaTransportUnknownUE(t *testing.T) {
	m := newTestMME(t)
	conn := &captureConn{}

	wire, err := (&s1ap.UplinkUEAssociatedLPPaTransport{
		MMEUES1APID: 4242,
		ENBUES1APID: 1,
		RoutingID:   0,
		LPPaPDU:     []byte{0x01},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handleUplinkLPPaTransport(m, context.Background(), mme.NewRadioForTest(conn), initiatingValue(t, wire))

	if conn.count() == 0 {
		t.Fatal("expected an Error Indication for the unknown MME-UE-S1AP-ID")
	}
}

func TestHandleUplinkLPPaTransportMalformed(t *testing.T) {
	m := newTestMME(t)
	conn := &captureConn{}

	handleUplinkLPPaTransport(m, context.Background(), mme.NewRadioForTest(conn), []byte{0xff, 0xff, 0xff})

	if got := conn.count(); got != 1 {
		t.Fatalf("expected an Error Indication for the malformed transport, got %d S1AP messages", got)
	}

	parseOutboundErrorIndication(t, conn.sent[0])
}
