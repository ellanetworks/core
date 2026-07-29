// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestParseMessageDispatch checks that a message encoded by this package decodes
// back through ParseMessage as its own concrete type, for one message of each
// shape: an EMM message with mandatory elements, a header-only EMM message, and
// an ESM message.
func TestParseMessageDispatch(t *testing.T) {
	tests := []struct {
		name string
		msg  Message
		want any
	}{
		{"AttachRequest", &AttachRequest{
			EPSAttachType:       AttachTypeEPS,
			NASKeySetIdentifier: nas.NoKeySet,
			EPSMobileIdentity:   IMSIIdentity(IMSI("001010000000001")),
			ESMMessageContainer: []byte{0x02, 0x01, 0xD0, 0x11},
		}, (*AttachRequest)(nil)},
		{"EMMStatus", &EMMStatus{Cause: EMMCauseIllegalUE}, (*EMMStatus)(nil)},
		{"ESMStatus", &ESMStatus{EPSBearerIdentity: 5, Cause: ESMCauseRegularDeactivation}, (*ESMStatus)(nil)},
		{"ESMInformationRequest", &ESMInformationRequest{EPSBearerIdentity: 0, PTI: 1}, (*ESMInformationRequest)(nil)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := tc.msg.MarshalBinary()
			if err != nil {
				t.Fatalf("MarshalBinary: %v", err)
			}

			got, err := ParseMessage(b, nas.DirectionUplink)
			if err != nil {
				t.Fatalf("ParseMessage: %v", err)
			}

			if gotType, wantType := fmt.Sprintf("%T", got), fmt.Sprintf("%T", tc.want); gotType != wantType {
				t.Fatalf("ParseMessage returned %s, want %s", gotType, wantType)
			}

			round, err := got.MarshalBinary()
			if err != nil {
				t.Fatalf("re-encode: %v", err)
			}

			if !bytes.Equal(round, b) {
				t.Errorf("re-encode = % x, want % x", round, b)
			}
		})
	}
}

// TestParseMessageReportsItsType checks that the message a dispatch returns
// names the type the header carried.
func TestParseMessageReportsItsType(t *testing.T) {
	b, err := (&EMMStatus{Cause: EMMCauseIllegalUE}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseMessage(b, nas.DirectionDownlink)
	if err != nil {
		t.Fatal(err)
	}

	emm, ok := msg.(EMMMessage)
	if !ok {
		t.Fatalf("%T is not an EMMMessage", msg)
	}

	if emm.MessageType() != MsgEMMStatus {
		t.Errorf("MessageType = %s, want %s", emm.MessageType(), MsgEMMStatus)
	}
}

// TestParseMessageDetachDirection pins the one place this generation needs the
// direction: TS 24.301 table 9.8.1 gives DETACH REQUEST a single message type,
// and §8.2.11.1 and §8.2.11.2 define a different message in each direction.
func TestParseMessageDetachDirection(t *testing.T) {
	uplink, err := (&DetachRequestUE{
		TypeOfDetach:      DetachTypeEPS,
		SwitchOff:         true,
		EPSMobileIdentity: IMSIIdentity(IMSI("001010000000001")),
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseMessage(uplink, nas.DirectionUplink)
	if err != nil {
		t.Fatalf("uplink: %v", err)
	}

	if _, ok := msg.(*DetachRequestUE); !ok {
		t.Errorf("uplink DETACH REQUEST decoded as %T, want *DetachRequestUE", msg)
	}

	downlink, err := (&DetachRequestNetwork{TypeOfDetach: DetachTypeReattachRequired}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	msg, err = ParseMessage(downlink, nas.DirectionDownlink)
	if err != nil {
		t.Fatalf("downlink: %v", err)
	}

	if _, ok := msg.(*DetachRequestNetwork); !ok {
		t.Errorf("downlink DETACH REQUEST decoded as %T, want *DetachRequestNetwork", msg)
	}
}

// TestParseMessageServiceRequest checks that the message TS 24.301 §8.2.25
// frames as a security header type rather than as a plain message still reaches
// its parser.
func TestParseMessageServiceRequest(t *testing.T) {
	b, err := (&ServiceRequest{KSI: 3, SeqShort: 5, ShortMAC: [2]byte{0xAA, 0xBB}}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseMessage(b, nas.DirectionUplink)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	sr, ok := msg.(*ServiceRequest)
	if !ok {
		t.Fatalf("ParseMessage returned %T, want *ServiceRequest", msg)
	}

	if sr.KSI != 3 || sr.SeqShort != 5 || sr.ShortMAC != [2]byte{0xAA, 0xBB} {
		t.Errorf("SERVICE REQUEST = %+v", sr)
	}
}

// TestParseMessageUnknownType checks that a message type this package does not
// model survives as a message of its own protocol: a soft error, a value that
// satisfies the protocol's interface so a receiver can answer it with a STATUS
// (TS 24.301 §7.4), and the octets it arrived as.
func TestParseMessageUnknownType(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  []byte
		typ  uint8
		// esm reports which protocol the message belongs to, and so which
		// interface the decoded value has to satisfy.
		esm bool
	}{
		{"EMM", []byte{uint8(PDEMM), 0x64, 0xAA, 0xBB}, 0x64, false},
		{"ESM", []byte{0x50 | uint8(PDESM), 0x01, 0xDB, 0xAA}, 0xDB, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := ParseMessage(tc.raw, nas.DirectionDownlink)
			if !errors.Is(err, nas.ErrUnknownMessageType) {
				t.Fatalf("err = %v, want ErrUnknownMessageType", err)
			}

			if !nas.SoftOnly(err) {
				t.Error("an unknown message type must be a soft error")
			}

			var (
				unknown  Message
				gotType  uint8
				inDomain bool
			)

			if tc.esm {
				m, ok := msg.(*UnknownESMMessage)
				if !ok {
					t.Fatalf("ParseMessage returned %T, want *UnknownESMMessage", msg)
				}

				// The point of the split: an unmodelled message is a message of
				// its own protocol, so the domain interface reaches it.
				_, inDomain = msg.(ESMMessage)
				unknown, gotType = m, uint8(m.Type)
			} else {
				m, ok := msg.(*UnknownEMMMessage)
				if !ok {
					t.Fatalf("ParseMessage returned %T, want *UnknownEMMMessage", msg)
				}

				_, inDomain = msg.(EMMMessage)
				unknown, gotType = m, uint8(m.Type)
			}

			if !inDomain {
				t.Errorf("%T does not satisfy its protocol's interface", msg)
			}

			if gotType != tc.typ {
				t.Errorf("type %#x, want %#x", gotType, tc.typ)
			}

			round, err := unknown.MarshalBinary()
			if err != nil || !bytes.Equal(round, tc.raw) {
				t.Errorf("re-encode = % x (%v), want % x", round, err, tc.raw)
			}

			// The message owns its memory, so mutating the input cannot change it.
			input := bytes.Clone(tc.raw)

			again, _ := ParseMessage(input, nas.DirectionDownlink)
			for i := range input {
				input[i] ^= 0xFF
			}

			if round, _ := again.MarshalBinary(); !bytes.Equal(round, tc.raw) {
				t.Errorf("after overwriting the input, re-encode = % x, want % x", round, tc.raw)
			}
		})
	}
}

// TestParseMessageHardFailures checks that everything ParseMessage cannot decode
// returns a nil message, so a caller that ignores the error cannot act on a
// half-decoded one.
func TestParseMessageHardFailures(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want error
	}{
		{"empty", nil, nas.ErrTruncated},
		{"unknown protocol discriminator", []byte{0x01, 0x41}, nas.ErrUnknownProtocolDiscriminator},
		{"security protected", []byte{uint8(SHTIntegrityProtected)<<4 | uint8(PDEMM), 0, 0, 0, 0, 1, 0x41}, ErrNotPlain},
		{"truncated header", []byte{uint8(PDEMM)}, nas.ErrTruncated},
		{"truncated body", []byte{uint8(PDEMM), uint8(MsgEMMStatus)}, nas.ErrTruncated},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := ParseMessage(tc.raw, nas.DirectionUplink)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}

			if msg != nil {
				t.Errorf("message = %v, want nil", msg)
			}

			if nas.SoftOnly(err) {
				t.Error("a hard failure must not report as soft")
			}
		})
	}
}

// TestParseMessagePDULimit checks the cap on both directions.
func TestParseMessagePDULimit(t *testing.T) {
	over := make([]byte, nas.MaxPDULen+1)
	over[0] = uint8(PDEMM)

	if _, err := ParseMessage(over, nas.DirectionUplink); err == nil {
		t.Error("an over-long PDU decoded")
	}

	huge := &EMMStatus{Cause: EMMCauseIllegalUE, Unrecognized: []nas.RawIE{
		{IEI: 0x7B, Format: nas.IETLVE, Value: make([]byte, nas.MaxPDULen)},
	}}
	if _, err := huge.MarshalBinary(); err == nil {
		t.Error("an over-long message encoded")
	}
}

// TestDispatchTablesNameRealMessageTypes checks that every dispatch entry is
// keyed by an assigned message type, which catches a table wired to the wrong
// constant.
func TestDispatchTablesNameRealMessageTypes(t *testing.T) {
	for mt := range emmParsers {
		if strings.Contains(mt.String(), "unknown") {
			t.Errorf("EMM dispatch table names unassigned type %#x", uint8(mt))
		}
	}

	for mt := range esmParsers {
		if strings.Contains(mt.String(), "unknown") {
			t.Errorf("ESM dispatch table names unassigned type %#x", uint8(mt))
		}
	}
}

// TestUnknownESMMessageKeepsItsHeader checks that an unmodelled ESM message
// carries the header a receiver has to echo, readable through ESMMessage: TS 24.301 §7.4 has the network
// answer with an ESM STATUS, and §8.3.15 gives that STATUS the EPS bearer
// identity and procedure transaction identity of the message it answers.
func TestUnknownESMMessageKeepsItsHeader(t *testing.T) {
	const (
		bearer = EPSBearerIdentity(5)
		pti    = nas.ProcedureTransactionIdentity(9)
	)

	raw := []byte{uint8(bearer)<<4 | uint8(PDESM), uint8(pti), 0xDB, 0xAA}

	msg, err := ParseMessage(raw, nas.DirectionUplink)
	if !errors.Is(err, nas.ErrUnknownMessageType) {
		t.Fatalf("err = %v, want ErrUnknownMessageType", err)
	}

	if _, ok := msg.(*UnknownESMMessage); !ok {
		t.Fatalf("ParseMessage returned %T, want *UnknownESMMessage", msg)
	}

	// The header reads through the protocol interface, exactly as it does for a
	// message this package models.
	esm, ok := msg.(ESMMessage)
	if !ok {
		t.Fatalf("%T does not satisfy ESMMessage", msg)
	}

	if esm.BearerIdentity() != bearer || esm.TransactionIdentity() != pti {
		t.Errorf("header = bearer %v, PTI %v; want %v / %v",
			esm.BearerIdentity(), esm.TransactionIdentity(), bearer, pti)
	}

	// An EMM message has no bearer header, and lands in the other protocol.
	msg, _ = ParseMessage([]byte{uint8(PDEMM), 0x64}, nas.DirectionUplink)

	if _, ok := msg.(*UnknownEMMMessage); !ok {
		t.Errorf("an unknown EMM message decoded as %T, want *UnknownEMMMessage", msg)
	}
}
