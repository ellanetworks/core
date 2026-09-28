// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"encoding/hex"
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestDownlinkNASTransportWire(t *testing.T) {
	m := &DownlinkNASTransport{NASMessageContainer: mustHex("090103010000")}

	b, err := m.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if want := "076206090103010000"; hex.EncodeToString(b) != want {
		t.Fatalf("DOWNLINK NAS TRANSPORT = %x, want %s", b, want)
	}

	got, err := ParseDownlinkNASTransport(b)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, m) {
		t.Fatalf("parsed %+v, want %+v", got, m)
	}
}

func TestUplinkNASTransportWire(t *testing.T) {
	b := mustHex("0763028904")

	got, err := ParseUplinkNASTransport(b)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got.NASMessageContainer, []byte{0x89, 0x04}) {
		t.Fatalf("container = % x, want 89 04", got.NASMessageContainer)
	}

	out, err := got.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out, b) {
		t.Fatalf("re-encode = % x, want % x", out, b)
	}
}

func TestNASTransportDispatch(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want any
	}{
		{"0762020904", &DownlinkNASTransport{NASMessageContainer: []byte{0x09, 0x04}}},
		{"0763020904", &UplinkNASTransport{NASMessageContainer: []byte{0x09, 0x04}}},
	} {
		msg, err := ParseMessage(mustHex(tc.in), nas.DirectionUplink)
		if err != nil {
			t.Fatalf("%s: %v", tc.in, err)
		}

		if !reflect.DeepEqual(msg, tc.want) {
			t.Fatalf("%s: parsed %+v, want %+v", tc.in, msg, tc.want)
		}
	}
}

func TestNASMessageContainerBounds(t *testing.T) {
	for _, n := range []int{0, 1, 256} {
		if _, err := (&UplinkNASTransport{NASMessageContainer: make([]byte, n)}).MarshalBinary(); err == nil {
			t.Errorf("%d-octet container encoded, want an error", n)
		}
	}

	for _, n := range []int{2, 251} {
		if _, err := (&DownlinkNASTransport{NASMessageContainer: make([]byte, n)}).MarshalBinary(); err != nil {
			t.Errorf("%d-octet container: %v", n, err)
		}
	}

	if _, err := ParseUplinkNASTransport(mustHex("07630109")); err == nil {
		t.Error("1-octet container parsed, want an error")
	}

	overlong := append(mustHex("0763fc"), make([]byte, 252)...)

	m, err := ParseUplinkNASTransport(overlong)
	if err != nil {
		t.Fatalf("252-octet container: %v", err)
	}

	b, err := m.MarshalBinary()
	if err != nil || !bytes.Equal(b, overlong) {
		t.Fatalf("re-encode of a 252-octet container = %v", err)
	}
}

func TestNASTransportTruncatedAndWrongType(t *testing.T) {
	if _, err := ParseUplinkNASTransport(mustHex("07630509")); !errors.Is(err, nas.ErrTruncated) {
		t.Fatalf("err = %v, want ErrTruncated", err)
	}

	if _, err := ParseDownlinkNASTransport(mustHex("0763020904")); !errors.Is(err, ErrWrongMessageType) {
		t.Fatalf("err = %v, want ErrWrongMessageType", err)
	}
}
