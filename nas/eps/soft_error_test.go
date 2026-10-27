// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestSyntacticallyIncorrectOptionalIEIsAbsent pins TS 24.301 §7.7.1: an optional
// element the network cannot decode is treated as not present, the rest of the
// message decodes, and the element still re-encodes so nothing is lost.
func TestSyntacticallyIncorrectOptionalIEIsAbsent(t *testing.T) {
	// ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST with a malformed APN-AMBR
	// (IEI 0x5E, which needs at least two octets) ahead of a well-formed ESM cause.
	in := &ActivateDefaultEPSBearerContextRequest{
		EPSBearerIdentity: 5,
		PTI:               1,
		EPSQoS:            EPSQoS{QCI: 9},
		AccessPointName:   APN("internet"),
		PDNAddress:        PDNAddress{PDNType: PDNTypeIPv4, IPv4: [4]byte{10, 45, 0, 1}},
	}

	head, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	b := append(head, ieiAPNAMBR, 0x01, 0xfe, ieiESMCause, 0x32)

	msg, err := ParseActivateDefaultEPSBearerContextRequest(b)
	if err == nil {
		t.Fatal("a malformed optional element must be reported")
	}

	if !nas.SoftOnly(err) {
		t.Fatalf("a malformed optional element must be a soft error, got %v", err)
	}

	if msg == nil {
		t.Fatal("a soft error must still yield a usable message")
	}

	if msg.APNAMBR != nil {
		t.Errorf("the malformed element must be absent, got %+v", msg.APNAMBR)
	}

	// The mandatory part and the element after it still decoded.
	if msg.AccessPointName != "internet" || msg.Cause == nil || *msg.Cause != 0x32 {
		t.Errorf("surrounding elements lost: apn=%q cause=%v", msg.AccessPointName, msg.Cause)
	}

	var ie *nas.IEError
	if !errors.As(err, &ie) || ie.IEI != ieiAPNAMBR {
		t.Fatalf("error does not name the failed element: %v", err)
	}

	again, err := msg.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	if !bytes.Contains(again, []byte{ieiAPNAMBR, 0x01, 0xfe}) {
		t.Errorf("re-encoding dropped the malformed element: % x", again)
	}
}

// TestSecurityCriticalIEFailureIsHard pins the exception: the replayed NAS
// message proves a SECURITY MODE COMPLETE echoes what the UE actually sent, so a
// value that will not decode fails the message rather than vanishing.
func TestSecurityCriticalIEFailureIsHard(t *testing.T) {
	// A replayed NAS message container (IEI 0x4F, TLV-E) whose length runs past
	// the buffer is a framing failure the walk cannot step over.
	b := []byte{
		uint8(PDEMM), uint8(MsgSecurityModeComplete),
		ieiReplayedNASMessage, 0x00, 0x40, 0x01,
	}

	msg, err := ParseSecurityModeComplete(b)
	if err == nil {
		t.Fatal("a malformed security-critical element must be reported")
	}

	if nas.SoftOnly(err) {
		t.Fatalf("a malformed security-critical element must be a hard error, got %v", err)
	}

	if msg != nil {
		t.Fatal("a hard error must not yield a message")
	}
}

// TestRepeatedIEUsesTheFirst pins TS 24.301 §7.6.3: only the first occurrence of
// a repeated element is handled, and the rest are ignored without being lost.
func TestRepeatedIEUsesTheFirst(t *testing.T) {
	b := []byte{
		uint8(PDEMM), uint8(MsgAttachReject), 0x11,
		ieiT3402Value, 0x01, 0x21,
		ieiT3402Value, 0x01, 0x22, // repetition: must be ignored
	}

	msg, err := ParseAttachReject(b)
	if err != nil {
		t.Fatalf("ParseAttachReject: %v", err)
	}

	if msg.T3402 == nil || msg.T3402.Value != 1 {
		t.Fatalf("T3402 = %+v, want the first occurrence", msg.T3402)
	}

	again, err := msg.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	if got := bytes.Count(again, []byte{ieiT3402Value, 0x01}); got != 2 {
		t.Errorf("re-encoding kept %d occurrences, want both: % x", got, again)
	}
}

func TestRejectT3346AndExtendedEMMCauseRobustness(t *testing.T) {
	type view struct {
		t3346        *nas.GPRSTimer2
		extended     *ExtendedEMMCause
		unrecognized []nas.RawIE
		marshal      func() ([]byte, error)
	}

	messages := []struct {
		name  string
		mt    MessageType
		parse func([]byte) (view, error)
	}{
		{
			name: "AttachReject",
			mt:   MsgAttachReject,
			parse: func(b []byte) (view, error) {
				m, err := ParseAttachReject(b)
				if m == nil {
					return view{}, err
				}

				return view{m.T3346, m.ExtendedEMMCause, m.Unrecognized, m.MarshalBinary}, err
			},
		},
		{
			name: "TrackingAreaUpdateReject",
			mt:   MsgTrackingAreaUpdateReject,
			parse: func(b []byte) (view, error) {
				m, err := ParseTrackingAreaUpdateReject(b)
				if m == nil {
					return view{}, err
				}

				return view{m.T3346, m.ExtendedEMMCause, m.Unrecognized, m.MarshalBinary}, err
			},
		},
	}

	cases := []struct {
		name         string
		body         []byte
		softIEI      uint8
		t3346        *nas.GPRSTimer2
		extended     *ExtendedEMMCause
		unrecognized []nas.RawIE
		reencoded    []byte
	}{
		{
			name:         "T3346 missing its length",
			body:         []byte{0x0F, 0x5F},
			softIEI:      ieiT3346Value,
			unrecognized: []nas.RawIE{{IEI: ieiT3346Value, Format: nas.IETruncated, Value: []byte{}}},
		},
		{
			name:         "T3346 length past the buffer",
			body:         []byte{0x0F, 0x5F, 0x02},
			softIEI:      ieiT3346Value,
			unrecognized: []nas.RawIE{{IEI: ieiT3346Value, Format: nas.IETruncated, Value: []byte{0x02}}},
		},
		{
			name:         "T3346 two octets long",
			body:         []byte{0x0F, 0x5F, 0x02, 0x22, 0x33},
			softIEI:      ieiT3346Value,
			unrecognized: []nas.RawIE{{IEI: ieiT3346Value, Format: nas.IETLV, Value: []byte{0x22, 0x33}, Name: "T3346 value"}},
		},
		{
			name:         "T3346 empty",
			body:         []byte{0x0F, 0x5F, 0x00},
			softIEI:      ieiT3346Value,
			unrecognized: []nas.RawIE{{IEI: ieiT3346Value, Format: nas.IETLV, Value: []byte{}, Name: "T3346 value"}},
		},
		{
			name:         "repeated Extended EMM cause",
			body:         []byte{0x0F, 0xA1, 0xA4},
			extended:     &ExtendedEMMCause{EUTRANNotAllowed: true},
			unrecognized: []nas.RawIE{{IEI: ieiExtendedEMMCause, Format: nas.IETV1, Value: []byte{0x04}, Name: "Extended EMM cause", After: ieiExtendedEMMCause}},
		},
		{
			name:         "repeated T3346",
			body:         []byte{0x16, 0x5F, 0x01, 0x22, 0x5F, 0x01, 0x23},
			t3346:        &nas.GPRSTimer2{Unit: nas.GPRSTimer2Unit1Minute, Value: 2},
			unrecognized: []nas.RawIE{{IEI: ieiT3346Value, Format: nas.IETLV, Value: []byte{0x23}, Name: "T3346 value", After: ieiT3346Value}},
		},
		{
			name:  "T3346 deactivated",
			body:  []byte{0x16, 0x5F, 0x01, 0xE0},
			t3346: &nas.GPRSTimer2{Unit: nas.GPRSTimerUnitDeactivated},
		},
		{
			name:      "Extended EMM cause spare bit",
			body:      []byte{0x0F, 0xA8},
			extended:  &ExtendedEMMCause{},
			reencoded: []byte{0x0F, 0xA0},
		},
	}

	for _, msg := range messages {
		for _, tc := range cases {
			t.Run(msg.name+"/"+tc.name, func(t *testing.T) {
				b := append([]byte{uint8(PDEMM), uint8(msg.mt)}, tc.body...)

				got, err := msg.parse(b)
				if tc.softIEI == 0 {
					if err != nil {
						t.Fatalf("parse: %v", err)
					}
				} else {
					if err == nil || !nas.SoftOnly(err) {
						t.Fatalf("parse error = %v, want a soft error", err)
					}

					var ie *nas.IEError
					if !errors.As(err, &ie) || ie.IEI != tc.softIEI {
						t.Fatalf("error does not name IEI %#x: %v", tc.softIEI, err)
					}
				}

				if got.marshal == nil {
					t.Fatal("a soft error must still yield a usable message")
				}

				if !reflect.DeepEqual(got.t3346, tc.t3346) {
					t.Errorf("T3346 = %+v, want %+v", got.t3346, tc.t3346)
				}

				if !reflect.DeepEqual(got.extended, tc.extended) {
					t.Errorf("extended EMM cause = %+v, want %+v", got.extended, tc.extended)
				}

				if !reflect.DeepEqual(got.unrecognized, tc.unrecognized) {
					t.Errorf("unrecognized = %+v, want %+v", got.unrecognized, tc.unrecognized)
				}

				want := b
				if tc.reencoded != nil {
					want = append([]byte{uint8(PDEMM), uint8(msg.mt)}, tc.reencoded...)
				}

				again, err := got.marshal()
				if err != nil {
					t.Fatalf("MarshalBinary: %v", err)
				}

				if !bytes.Equal(again, want) {
					t.Errorf("re-encoding = % x, want % x", again, want)
				}
			})
		}
	}
}
