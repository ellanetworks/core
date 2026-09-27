// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/hex"
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestDownlinkGenericNASTransportMarshal(t *testing.T) {
	b, err := (&DownlinkGenericNASTransport{
		ContainerType:         GenericMessageContainerTypeLPP,
		Container:             []byte{0xaa, 0xbb, 0xcc},
		AdditionalInformation: []byte{0x01, 0x02},
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	want := "0768010003aabbcc65020102"
	if hex.EncodeToString(b) != want {
		t.Fatalf("DOWNLINK GENERIC NAS TRANSPORT = %s, want %s", hex.EncodeToString(b), want)
	}
}

func TestDownlinkGenericNASTransportMarshalWithoutAdditionalInformation(t *testing.T) {
	b, err := (&DownlinkGenericNASTransport{
		ContainerType: GenericMessageContainerTypeLPP,
		Container:     []byte{0xaa},
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	want := "0768010001aa"
	if hex.EncodeToString(b) != want {
		t.Fatalf("DOWNLINK GENERIC NAS TRANSPORT = %s, want %s", hex.EncodeToString(b), want)
	}
}

func TestParseUplinkGenericNASTransport(t *testing.T) {
	b := mustHex("0769010004deadbeef650107")

	got, err := ParseUplinkGenericNASTransport(b)
	if err != nil {
		t.Fatal(err)
	}

	want := &UplinkGenericNASTransport{
		ContainerType:         GenericMessageContainerTypeLPP,
		Container:             []byte{0xde, 0xad, 0xbe, 0xef},
		AdditionalInformation: []byte{0x07},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseUplinkGenericNASTransport = %+v, want %+v", got, want)
	}
}

func TestParseUplinkGenericNASTransportTruncatedContainer(t *testing.T) {
	_, err := ParseUplinkGenericNASTransport(mustHex("0769010004dead"))
	if !errors.Is(err, nas.ErrTruncated) {
		t.Fatalf("err = %v, want ErrTruncated", err)
	}
}

func TestParseGenericNASTransportWrongDirection(t *testing.T) {
	_, err := ParseDownlinkGenericNASTransport(mustHex("0769010001aa"))
	if !errors.Is(err, ErrWrongMessageType) {
		t.Fatalf("err = %v, want ErrWrongMessageType", err)
	}
}

func TestParseMessageGenericNASTransport(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  []byte
		want Message
	}{
		{
			"downlink", mustHex("0768010001aa"),
			&DownlinkGenericNASTransport{ContainerType: GenericMessageContainerTypeLPP, Container: []byte{0xaa}},
		},
		{
			"uplink", mustHex("0769020001bb"),
			&UplinkGenericNASTransport{ContainerType: GenericMessageContainerTypeLocationServices, Container: []byte{0xbb}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseMessage(tc.raw, nas.DirectionUplink)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseMessage = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestGenericNASTransportKeepsUnrecognizedIEs(t *testing.T) {
	b := mustHex("0769010001aa6501072a020a0b")

	msg, err := ParseUplinkGenericNASTransport(b)
	if err != nil {
		t.Fatal(err)
	}

	if len(msg.Unrecognized) != 1 {
		t.Fatalf("Unrecognized = %+v, want one element", msg.Unrecognized)
	}

	round, err := msg.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if hex.EncodeToString(round) != hex.EncodeToString(b) {
		t.Fatalf("re-encoded = %x, want %x", round, b)
	}
}

func TestGenericMessageContainerTypeName(t *testing.T) {
	if got := GenericMessageContainerTypeLPP.Name(); got != "LTE Positioning Protocol (LPP) message container" {
		t.Errorf("LPP name = %q", got)
	}

	if got := GenericMessageContainerType(0x03).Name(); got != "" {
		t.Errorf("unassigned name = %q, want empty", got)
	}
}
