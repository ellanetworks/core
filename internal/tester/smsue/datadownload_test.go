// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsue

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/sms"
)

// fakeUICC answers every ENVELOPE with response and sw, and keeps the ENVELOPEs it gets.
type fakeUICC struct {
	response []byte
	sw       uint16
	got      [][]byte
}

func (u *fakeUICC) Envelope(data []byte) ([]byte, uint16) {
	u.got = append(u.got, data)
	return u.response, u.sw
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

// deliverRP has the stack take an MT RP-DATA of tpdu from the service centre +15550000000, and returns the RP message
// that the stack answers it with.
func deliverRP(t *testing.T, s *Stack, sent *[][]byte, tpdu []byte) sms.RPMessage {
	t.Helper()

	rp, err := (&sms.RPData{
		Direction: nas.DirectionDownlink, Reference: 7, Originator: sms.E164Address("15550000000"), UserData: tpdu,
	}).AppendBinary(nil)
	if err != nil {
		t.Fatal(err)
	}

	cp, err := (&sms.CPData{TransactionIdentifier: sms.TransactionIdentifier{Value: 0}, UserData: rp}).AppendBinary(nil)
	if err != nil {
		t.Fatal(err)
	}

	*sent = nil

	if err := s.Deliver(cp); err != nil {
		t.Fatal(err)
	}

	// A CP-ACK, then the CP-DATA of the RP answer.
	if len(*sent) != 2 {
		t.Fatalf("sent %d CP messages, want 2", len(*sent))
	}

	m, err := sms.ParseCP((*sent)[1])
	if err != nil {
		t.Fatal(err)
	}

	data, ok := m.(*sms.CPData)
	if !ok {
		t.Fatalf("answer %T, want a CP-DATA", m)
	}

	answer, err := sms.ParseRP(data.UserData, nas.DirectionUplink)
	if err != nil {
		t.Fatal(err)
	}

	return answer
}

// dataDownloadTPDU is an SMS-DELIVER from +15550001000 of TP-PID '7F', TP-DCS 'F6' and UDHI, whose user data is a
// command packet header.
const dataDownloadTPDU = "440b915155001000f07ff6" + "62909251341100" + "06" + "027000000102"

// TestDataDownload checks that the ME passes a (U)SIM data download to the UICC in an ENVELOPE (SMS-PP DOWNLOAD), and
// answers the network as the UICC's status word has it (TS 31.111 clause 7.1.1).
func TestDataDownload(t *testing.T) {
	por := mustHex(t, "027100000e0ab000110000000000000001612f")

	cases := []struct {
		name string
		sw   uint16
		// report is the RP-User-Data of the RP-ACK, or of the RP-ERROR when cause is set.
		report string
		cause  sms.RPCause
	}{
		// TP-MTI '00' without UDHI, TP-PI: TP-PID, TP-DCS and TP-UDL, those of the SMS-DELIVER, then the response.
		{name: "response data", sw: 0x9000, report: "00077ff613" + hex.EncodeToString(por)},
		{name: "warning", sw: 0x6282, report: "00077ff613" + hex.EncodeToString(por)},
		// TP-MTI '00', TP-FCS "(U)SIM Application Toolkit Busy", no optional parameter.
		{name: "toolkit busy", sw: 0x9300, report: "00d400", cause: sms.RPCauseProtocolErrorUnspecified},
		// TP-FCS "(U)SIM data download error".
		{name: "error", sw: 0x6a80, report: "00d500", cause: sms.RPCauseProtocolErrorUnspecified},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sent [][]byte

			s := New(func(cp []byte) error { sent = append(sent, cp); return nil })
			uicc := &fakeUICC{response: por, sw: tc.sw}
			s.SetUICC(uicc)

			answer := deliverRP(t, s, &sent, mustHex(t, dataDownloadTPDU))

			// D1, then the device identities from the network to the UICC, the service centre's address and the
			// SMS-DELIVER (TS 31.111 clause 7.1.1.2).
			wantEnvelope := "d121" + "82028381" + "0607915155000000f0" + "8b16" + dataDownloadTPDU
			if len(uicc.got) != 1 || hex.EncodeToString(uicc.got[0]) != wantEnvelope {
				t.Fatalf("ENVELOPE %x, want %s", uicc.got, wantEnvelope)
			}

			var userData []byte

			switch m := answer.(type) {
			case *sms.RPAck:
				if tc.cause != 0 || m.Reference != 7 {
					t.Fatalf("RP-ACK %+v", m)
				}

				userData = m.UserData
			case *sms.RPError:
				if m.Cause != tc.cause || m.Reference != 7 {
					t.Fatalf("RP-ERROR %+v", m)
				}

				userData = m.UserData
			default:
				t.Fatalf("answer %T", answer)
			}

			if hex.EncodeToString(userData) != tc.report {
				t.Fatalf("SMS-DELIVER-REPORT %x, want %s", userData, tc.report)
			}

			select {
			case m := <-s.inbox:
				t.Fatalf("the data download reached the inbox: %+v", m)
			default:
			}
		})
	}
}

// TestDataDownloadWithoutUICC checks that, without a UICC, and for other messages, the ME receives a short message as
// it otherwise does.
func TestDataDownloadWithoutUICC(t *testing.T) {
	var sent [][]byte

	s := New(func(cp []byte) error { sent = append(sent, cp); return nil })

	answer := deliverRP(t, s, &sent, mustHex(t, dataDownloadTPDU))
	if ack, ok := answer.(*sms.RPAck); !ok || ack.UserData != nil {
		t.Fatalf("answer %+v, want a plain RP-ACK", answer)
	}

	uicc := &fakeUICC{sw: 0x9000}
	s.SetUICC(uicc)

	// An 8-bit text of class 1, TP-PID '00'.
	answer = deliverRP(t, s, &sent, mustHex(t, "040b915155210300f1000462909251341100076865"+"6c6c6f2042"))
	if ack, ok := answer.(*sms.RPAck); !ok || ack.UserData != nil || len(uicc.got) != 0 {
		t.Fatalf("answer %+v, ENVELOPEs %d, want a plain RP-ACK and none", answer, len(uicc.got))
	}

	if m := <-s.inbox; m.Text != "hello B" || !bytes.Equal([]byte(m.From), []byte("+15551230001")) {
		t.Fatalf("inbox %+v", m)
	}
}
