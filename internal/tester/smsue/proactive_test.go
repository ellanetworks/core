// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsue

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/tester/cat"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/sms"
)

type proactiveUICC struct {
	command   []byte
	responses chan []byte
}

func (u *proactiveUICC) Envelope([]byte) ([]byte, uint16) {
	return nil, uint16(cat.SWProactive)<<8 | uint16(len(u.command))
}

func (u *proactiveUICC) Fetch() ([]byte, uint16) {
	return u.command, cat.SWOK
}

func (u *proactiveUICC) TerminalResponse(data []byte) uint16 {
	u.responses <- data
	return cat.SWOK
}

func sendShortMessage(t *testing.T, objects ...[]byte) []byte {
	t.Helper()

	value := cat.AppendTLV(nil, 0x81, []byte{0x07, cat.TypeSendShortMessage, 0x00})
	value = cat.AppendTLV(value, 0x82, []byte{cat.DeviceUICC, cat.DeviceNetwork})

	for _, o := range objects {
		value = append(value, o...)
	}

	return cat.AppendTLV(nil, cat.TagProactiveCommand, value)
}

func mtCP(t *testing.T, ref uint8, tpdu []byte) []byte {
	t.Helper()

	rp, err := (&sms.RPData{
		Direction: nas.DirectionDownlink, Reference: ref, Originator: sms.E164Address("15550000000"), UserData: tpdu,
	}).AppendBinary(nil)
	if err != nil {
		t.Fatal(err)
	}

	cp, err := (&sms.CPData{TransactionIdentifier: sms.TransactionIdentifier{Value: 0}, UserData: rp}).AppendBinary(nil)
	if err != nil {
		t.Fatal(err)
	}

	return cp
}

func nextCP(t *testing.T, sent chan []byte) sms.CPMessage {
	t.Helper()

	select {
	case b := <-sent:
		m, err := sms.ParseCP(b)
		if err != nil {
			t.Fatal(err)
		}

		return m
	case <-time.After(time.Second):
		t.Fatal("no CP message sent")
		return nil
	}
}

func TestProactiveSendShortMessage(t *testing.T) {
	submit := mustHex(t, "41ff0b915155001000f07ff603027100")

	cases := []struct {
		name   string
		report sms.RPMessage
		result string
	}{
		{name: "RP-ACK", report: &sms.RPAck{Direction: nas.DirectionDownlink}, result: "00"},
		{name: "RP-ERROR", report: &sms.RPError{Direction: nas.DirectionDownlink, Cause: sms.RPCauseProtocolErrorUnspecified}, result: "35" + "6f"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sent := make(chan []byte, 16)
			s := New(func(cp []byte) error { sent <- cp; return nil })

			uicc := &proactiveUICC{
				command:   sendShortMessage(t, cat.AppendTLV(nil, cat.TagAddress, mustHex(t, "915155000000f0")), cat.AppendTLV(nil, 0x8b, submit)),
				responses: make(chan []byte, 1),
			}
			s.SetUICC(uicc)

			if err := s.Deliver(mtCP(t, 7, mustHex(t, dataDownloadTPDU))); err != nil {
				t.Fatal(err)
			}

			nextCP(t, sent)

			if data, ok := nextCP(t, sent).(*sms.CPData); !ok {
				t.Fatalf("no RP report to the data download")
			} else if rp, err := sms.ParseRP(data.UserData, nas.DirectionUplink); err != nil {
				t.Fatal(err)
			} else if _, ok := rp.(*sms.RPAck); !ok {
				t.Fatalf("RP report %T, want an RP-ACK", rp)
			}

			mo, ok := nextCP(t, sent).(*sms.CPData)
			if !ok {
				t.Fatal("no MO CP-DATA")
			}

			rp, err := sms.ParseRP(mo.UserData, nas.DirectionUplink)
			if err != nil {
				t.Fatal(err)
			}

			data, ok := rp.(*sms.RPData)
			if !ok || data.Destination == nil || data.Destination.Digits != "15550000000" {
				t.Fatalf("MO RP message %+v", rp)
			}

			if want := append([]byte{submit[0], data.Reference}, submit[2:]...); !bytes.Equal(data.UserData, want) {
				t.Fatalf("SMS-SUBMIT %x, want %x", data.UserData, want)
			}

			switch r := tc.report.(type) {
			case *sms.RPAck:
				r.Reference = data.Reference
			case *sms.RPError:
				r.Reference = data.Reference
			}

			report, err := tc.report.AppendBinary(nil)
			if err != nil {
				t.Fatal(err)
			}

			peer := mo.TransactionIdentifier.Peer()

			cp, err := (&sms.CPData{TransactionIdentifier: peer, UserData: report}).AppendBinary(nil)
			if err != nil {
				t.Fatal(err)
			}

			if err := s.Deliver(cp); err != nil {
				t.Fatal(err)
			}

			select {
			case got := <-uicc.responses:
				want := "8103071300" + "82028281" + "83" + hex.EncodeToString([]byte{byte(len(tc.result) / 2)}) + tc.result

				if hex.EncodeToString(got) != want {
					t.Fatalf("TERMINAL RESPONSE %x, want %s", got, want)
				}
			case <-time.After(time.Second):
				t.Fatal("no TERMINAL RESPONSE")
			}
		})
	}
}

func TestProactiveUnsupported(t *testing.T) {
	cases := map[string]struct {
		command []byte
		result  string
	}{
		"packing required": {
			command: cat.AppendTLV(nil, cat.TagProactiveCommand, concat(
				cat.AppendTLV(nil, 0x81, []byte{0x01, cat.TypeSendShortMessage, cat.QualifierSMSPacking}),
				cat.AppendTLV(nil, 0x82, []byte{cat.DeviceUICC, cat.DeviceNetwork}),
			)),
			result: "8103011301" + "82028281" + "830130",
		},
		"no SMS TPDU": {
			command: sendShortMessage(t, cat.AppendTLV(nil, cat.TagAddress, mustHex(t, "915155000000f0"))),
			result:  "8103071300" + "82028281" + "830136",
		},
		"no service centre": {
			command: sendShortMessage(t, cat.AppendTLV(nil, 0x8b, mustHex(t, "41000b915155001000f07ff600"))),
			result:  "8103071300" + "82028281" + "83022000",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := New(func([]byte) error { return nil })

			if got := hex.EncodeToString(s.proactiveCommand(tc.command)); got != tc.result {
				t.Fatalf("TERMINAL RESPONSE %s, want %s", got, tc.result)
			}
		})
	}
}

func TestDataDownloadResponseTooLarge(t *testing.T) {
	sent := make(chan []byte, 16)
	s := New(func(cp []byte) error { sent <- cp; return nil })
	s.SetUICC(&fakeUICC{response: make([]byte, 160), sw: cat.SWOK})

	if err := s.Deliver(mtCP(t, 7, mustHex(t, dataDownloadTPDU))); err == nil {
		t.Fatal("no error")
	}

	nextCP(t, sent)

	data, ok := nextCP(t, sent).(*sms.CPData)
	if !ok {
		t.Fatal("no RP report")
	}

	rp, err := sms.ParseRP(data.UserData, nas.DirectionUplink)
	if err != nil {
		t.Fatal(err)
	}

	if e, ok := rp.(*sms.RPError); !ok || hex.EncodeToString(e.UserData) != "00d500" {
		t.Fatalf("RP report %+v, want an RP-ERROR of TP-FCS D5", rp)
	}
}

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}

	return out
}
