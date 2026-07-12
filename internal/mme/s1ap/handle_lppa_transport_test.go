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

// TestHandleUplinkLPPaTransportUnknownUE routes a transport whose MME-UE-S1AP-ID
// resolves no UE; resolveUE answers with an Error Indication and nothing panics.
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

// TestHandleUplinkLPPaTransportMalformed feeds garbage; handleParseError must
// log and return without panicking.
func TestHandleUplinkLPPaTransportMalformed(t *testing.T) {
	m := newTestMME(t)

	handleUplinkLPPaTransport(m, context.Background(), mme.NewRadioForTest(&captureConn{}), []byte{0xff, 0xff, 0xff})
}
