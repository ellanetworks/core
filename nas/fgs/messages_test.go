// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

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
// shape: a 5GMM message with mandatory elements, a header-only 5GMM message, and
// a 5GSM message.
func TestParseMessageDispatch(t *testing.T) {
	guti := GUTIIdentity(GUTI{
		PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 1, AMFSetID: 1, AMFPointer: 1,
		TMSI: [4]byte{1, 2, 3, 4},
	})

	tests := []struct {
		name string
		msg  Message
		want any
	}{
		{"RegistrationRequest", &RegistrationRequest{
			RegistrationType: RegistrationTypeInitial, NgKSI: nas.NoKeySet, MobileIdentity: guti,
		}, (*RegistrationRequest)(nil)},
		{"GMMStatus", &GMMStatus{Cause: GMMCauseIllegalUE}, (*GMMStatus)(nil)},
		{"GSMStatus", &GSMStatus{PDUSessionID: 5, Cause: GSMCauseRegularDeactivation}, (*GSMStatus)(nil)},
		{"PDUSessionReleaseComplete", &PDUSessionReleaseComplete{PDUSessionID: 5}, (*PDUSessionReleaseComplete)(nil)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := tc.msg.MarshalBinary()
			if err != nil {
				t.Fatalf("MarshalBinary: %v", err)
			}

			got, err := ParseMessage(b)
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
	b, err := (&GMMStatus{Cause: GMMCauseIllegalUE}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseMessage(b)
	if err != nil {
		t.Fatal(err)
	}

	gmm, ok := msg.(GMMMessage)
	if !ok {
		t.Fatalf("%T is not a GMMMessage", msg)
	}

	if gmm.MessageType() != MsgGMMStatus {
		t.Errorf("MessageType = %s, want %s", gmm.MessageType(), MsgGMMStatus)
	}
}

// TestParseMessageUnknownType checks that a message type this package does not
// model survives as an UnknownMessage: a soft error, the header fields a STATUS
// needs, and the octets it arrived as.
func TestParseMessageUnknownType(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  []byte
		pd   ProtocolDiscriminator
		typ  uint8
	}{
		{"5GMM", []byte{uint8(EPD5GMM), 0x00, 0x50, 0xAA, 0xBB}, EPD5GMM, 0x50},
		{"5GSM", []byte{uint8(EPD5GSM), 0x05, 0x01, 0xC5, 0xAA}, EPD5GSM, 0xC5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := ParseMessage(tc.raw)
			if !errors.Is(err, nas.ErrUnknownMessageType) {
				t.Fatalf("err = %v, want ErrUnknownMessageType", err)
			}

			if !nas.SoftOnly(err) {
				t.Error("an unknown message type must be a soft error")
			}

			unknown, ok := msg.(*UnknownMessage)
			if !ok {
				t.Fatalf("ParseMessage returned %T, want *UnknownMessage", msg)
			}

			if unknown.PD != tc.pd || unknown.Type != tc.typ {
				t.Errorf("PD %#x type %#x, want %#x / %#x", uint8(unknown.PD), unknown.Type, uint8(tc.pd), tc.typ)
			}

			round, err := unknown.MarshalBinary()
			if err != nil || !bytes.Equal(round, tc.raw) {
				t.Errorf("re-encode = % x (%v), want % x", round, err, tc.raw)
			}

			// The message owns its memory, so mutating the input cannot change it.
			input := bytes.Clone(tc.raw)

			again, _ := ParseMessage(input)
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
		{"unknown protocol discriminator", []byte{0x01, 0x00, 0x41}, nas.ErrUnknownProtocolDiscriminator},
		{"security protected", []byte{uint8(EPD5GMM), 0x01, 0x00, 0x00, 0x00, 0x00, 0x01}, ErrNotPlain},
		{"truncated header", []byte{uint8(EPD5GMM), 0x00}, nas.ErrTruncated},
		{"truncated body", []byte{uint8(EPD5GMM), 0x00, uint8(MsgGMMStatus)}, nas.ErrTruncated},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := ParseMessage(tc.raw)
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
	over[0] = uint8(EPD5GMM)

	if _, err := ParseMessage(over); err == nil {
		t.Error("an over-long PDU decoded")
	}

	huge := &GMMStatus{Cause: GMMCauseIllegalUE, Unrecognized: []nas.RawIE{
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
	for mt := range gmmParsers {
		if strings.Contains(mt.String(), "unknown") {
			t.Errorf("5GMM dispatch table names unassigned type %#x", uint8(mt))
		}
	}

	for mt := range gsmParsers {
		if strings.Contains(mt.String(), "unknown") {
			t.Errorf("5GSM dispatch table names unassigned type %#x", uint8(mt))
		}
	}
}

// TestParseMessageDeregistrationAccept pins both accepts of the de-registration
// procedure to their own message types: TS 24.501 §8.2.13 numbers the network's
// answer to a UE-originating de-registration 0x46, and §8.2.15 numbers the UE's
// answer to a network-initiated one 0x48. Dispatching a received 0x48 as the
// former leaves a network-initiated de-registration unanswered.
func TestParseMessageDeregistrationAccept(t *testing.T) {
	for _, tc := range []struct {
		msg  Message
		want MessageType
	}{
		{&DeregistrationAcceptUEOriginating{}, MsgDeregistrationAcceptUEOrig},
		{&DeregistrationAcceptUETerminated{}, MsgDeregistrationAcceptUETerm},
	} {
		b, err := tc.msg.MarshalBinary()
		if err != nil {
			t.Fatalf("%T: %v", tc.msg, err)
		}

		got, err := ParseMessage(b)
		if err != nil {
			t.Fatalf("%T: ParseMessage: %v", tc.msg, err)
		}

		if fmt.Sprintf("%T", got) != fmt.Sprintf("%T", tc.msg) {
			t.Errorf("message type %#02x decoded as %T, want %T", uint8(tc.want), got, tc.msg)
		}

		gmm, ok := got.(GMMMessage)
		if !ok || gmm.MessageType() != tc.want {
			t.Errorf("%T reports %v, want %v", got, gmm.MessageType(), tc.want)
		}
	}
}
