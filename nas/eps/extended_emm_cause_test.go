// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestExtendedEMMCauseBits(t *testing.T) {
	for _, tc := range []struct {
		nibble uint8
		want   ExtendedEMMCause
	}{
		{0x00, ExtendedEMMCause{}},
		{0x01, ExtendedEMMCause{EUTRANNotAllowed: true}},
		{0x02, ExtendedEMMCause{EPSOptimizationNotSupported: true}},
		{0x04, ExtendedEMMCause{NBIoTNotAllowed: true}},
		{0x07, ExtendedEMMCause{EUTRANNotAllowed: true, EPSOptimizationNotSupported: true, NBIoTNotAllowed: true}},
	} {
		got := ParseExtendedEMMCause(tc.nibble)
		if got != tc.want {
			t.Errorf("ParseExtendedEMMCause(%#x) = %+v, want %+v", tc.nibble, got, tc.want)
		}

		if n := tc.want.Nibble(); n != tc.nibble {
			t.Errorf("%+v.Nibble() = %#x, want %#x", tc.want, n, tc.nibble)
		}
	}
}

func TestExtendedEMMCauseIgnoresSpareBit(t *testing.T) {
	if got := ParseExtendedEMMCause(0x09); got != (ExtendedEMMCause{EUTRANNotAllowed: true}) {
		t.Fatalf("ParseExtendedEMMCause(0x09) = %+v, want only E-UTRAN not allowed", got)
	}

	if n := ParseExtendedEMMCause(0x0F).Nibble(); n != 0x07 {
		t.Fatalf("Nibble() = %#x, want the spare bit cleared (0x07)", n)
	}
}

func TestAttachRejectT3346AndExtendedEMMCause(t *testing.T) {
	wire := []byte{0x07, 0x44, 0x0F, 0x5F, 0x01, 0x22, 0x16, 0x01, 0x2C, 0xA1}

	t3346, err := nas.ParseGPRSTimer2([]byte{0x22})
	if err != nil {
		t.Fatal(err)
	}

	t3402, err := nas.ParseGPRSTimer2([]byte{0x2C})
	if err != nil {
		t.Fatal(err)
	}

	in := &AttachReject{
		Cause:            EMMCauseNoSuitableCellsInTrackingArea,
		T3346:            &t3346,
		T3402:            &t3402,
		ExtendedEMMCause: &ExtendedEMMCause{EUTRANNotAllowed: true},
	}

	b, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(b, wire) {
		t.Fatalf("encoding = % x, want % x", b, wire)
	}

	out, err := ParseAttachReject(wire)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(out, in) {
		t.Fatalf("decoded %+v, want %+v", out, in)
	}
}

func TestAttachRejectExtendedEMMCauseOutOfOrder(t *testing.T) {
	out, err := ParseAttachReject([]byte{0x07, 0x44, 0x0F, 0xA4, 0x5F, 0x01, 0x22})
	if err != nil {
		t.Fatal(err)
	}

	if out.ExtendedEMMCause == nil || !out.ExtendedEMMCause.NBIoTNotAllowed || out.T3346 == nil || len(out.Unrecognized) != 0 {
		t.Fatalf("decoded %+v", out)
	}

	b, err := out.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if want := []byte{0x07, 0x44, 0x0F, 0x5F, 0x01, 0x22, 0xA4}; !bytes.Equal(b, want) {
		t.Fatalf("re-encoding = % x, want % x", b, want)
	}
}

func TestTrackingAreaUpdateRejectT3346AndExtendedEMMCause(t *testing.T) {
	wire := []byte{0x07, 0x4B, 0x0F, 0x5F, 0x01, 0x22, 0xA6}

	t3346, err := nas.ParseGPRSTimer2([]byte{0x22})
	if err != nil {
		t.Fatal(err)
	}

	in := &TrackingAreaUpdateReject{
		Cause:            EMMCauseNoSuitableCellsInTrackingArea,
		T3346:            &t3346,
		ExtendedEMMCause: &ExtendedEMMCause{EPSOptimizationNotSupported: true, NBIoTNotAllowed: true},
	}

	b, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(b, wire) {
		t.Fatalf("encoding = % x, want % x", b, wire)
	}

	out, err := ParseTrackingAreaUpdateReject(wire)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(out, in) {
		t.Fatalf("decoded %+v, want %+v", out, in)
	}
}

func TestTrackingAreaUpdateRejectPreservesUnmodelledIEs(t *testing.T) {
	wire := []byte{0x07, 0x4B, 0x4E, 0x5F, 0x01, 0x22, 0xA0, 0x1C, 0x01, 0x21}

	out, err := ParseTrackingAreaUpdateReject(wire)
	if err != nil {
		t.Fatal(err)
	}

	if out.ExtendedEMMCause == nil || *out.ExtendedEMMCause != (ExtendedEMMCause{}) {
		t.Fatalf("extended EMM cause = %+v, want present and all clear", out.ExtendedEMMCause)
	}

	if len(out.Unrecognized) != 1 || out.Unrecognized[0].IEI != ieiLowerBoundTimer {
		t.Fatalf("unrecognized = %+v, want the lower bound timer", out.Unrecognized)
	}

	b, err := out.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(b, wire) {
		t.Fatalf("re-encoding = % x, want % x", b, wire)
	}
}
