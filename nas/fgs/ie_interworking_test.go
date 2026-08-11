// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestN1ModeToS1ModeContainerCarriesTheSequenceNumber pins TS 24.501 §9.11.2.7:
// the value part is the one Sequence number octet of §9.10, which TS 33.501
// §8.3.2 fills with the 8 least significant bits of the downlink NAS COUNT used
// to derive K'ASME. Taking a whole count is what keeps the caller from
// truncating it differently from the derivation.
func TestN1ModeToS1ModeContainerCarriesTheSequenceNumber(t *testing.T) {
	c := NewN1ModeToS1ModeNASTransparentContainer(nas.MakeCount(0x1234, 0x5a))
	if c.SequenceNumber != 0x5a {
		t.Fatalf("sequence number = %#02x, want 0x5a", c.SequenceNumber)
	}

	raw, err := c.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if want := []byte{0x5a}; !bytes.Equal(raw, want) {
		t.Fatalf("container = % x, want % x", raw, want)
	}

	back, err := ParseN1ModeToS1ModeNASTransparentContainer(raw)
	if err != nil {
		t.Fatal(err)
	}

	if back != c {
		t.Fatalf("round trip = %+v, want %+v", back, c)
	}

	for _, b := range [][]byte{{}, {1, 2}} {
		if _, err := ParseN1ModeToS1ModeNASTransparentContainer(b); err == nil {
			t.Errorf("%d octets: want an error, got none", len(b))
		}
	}
}

// s1ToN1Golden is the container of TS 24.501 figure 9.11.2.9.1 laid out by hand:
// MAC 11223344, ciphering 128-5G-EA2 and integrity 128-5G-IA1 in octet 7, NCC 5
// with a mapped ngKSI 3 in octet 8, and the two spare octets §9.11.2.9 codes as
// zero.
var (
	s1ToN1Golden    = []byte{0x11, 0x22, 0x33, 0x44, 0x21, 0x5b, 0x00, 0x00}
	s1ToN1Container = S1ModeToN1ModeNASTransparentContainer{
		MessageAuthenticationCode: [4]byte{0x11, 0x22, 0x33, 0x44},
		CipheringAlgorithm:        nas.CipheringAES,
		IntegrityAlgorithm:        nas.IntegritySNOW3G,
		NCC:                       5,
		NgKSI:                     nas.KeySetIdentifier{Value: 3, Mapped: true},
		Spare:                     []byte{0x00, 0x00},
	}
)

func TestS1ModeToN1ModeContainerWire(t *testing.T) {
	raw, err := s1ToN1Container.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(raw, s1ToN1Golden) {
		t.Fatalf("container = % x, want % x", raw, s1ToN1Golden)
	}

	back, err := ParseS1ModeToN1ModeNASTransparentContainer(s1ToN1Golden)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(back, s1ToN1Container) {
		t.Fatalf("round trip = %+v, want %+v", back, s1ToN1Container)
	}
}

// TestS1ModeToN1ModeContainerMACSpan pins TS 24.501 §4.4.3.3 b): the integrity
// protection covers the value part from octet 7, i.e. every octet after the code
// itself. Getting the span wrong fails the MAC for every UE.
func TestS1ModeToN1ModeContainerMACSpan(t *testing.T) {
	protected, err := s1ToN1Container.MACProtected()
	if err != nil {
		t.Fatal(err)
	}

	if want := s1ToN1Golden[4:]; !bytes.Equal(protected, want) {
		t.Fatalf("MAC-protected span = % x, want % x", protected, want)
	}

	// The code is not covered by itself, so changing it must not move the span.
	other := s1ToN1Container
	other.MessageAuthenticationCode = [4]byte{0xff, 0xff, 0xff, 0xff}

	again, err := other.MACProtected()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(again, protected) {
		t.Fatalf("MAC-protected span changed with the code: % x, want % x", again, protected)
	}
}

// TestS1ModeToN1ModeContainerSpareDefaultsToZero checks that a container built
// without spare octets still reaches the 8-octet value part §9.11.2.9 fixes,
// with octets 9 and 10 coded as zero.
func TestS1ModeToN1ModeContainerSpareDefaultsToZero(t *testing.T) {
	c := s1ToN1Container
	c.Spare = nil

	raw, err := c.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(raw, s1ToN1Golden) {
		t.Fatalf("container = % x, want % x", raw, s1ToN1Golden)
	}
}

// TestS1ModeToN1ModeContainerKeepsExcessOctets covers the §4.4.3.3 note: a
// receiver takes every octet from octet 7 into the integrity check whatever
// release it supports, so octets a later release adds have to survive decoding
// and stay under the MAC rather than be dropped as unknown.
func TestS1ModeToN1ModeContainerKeepsExcessOctets(t *testing.T) {
	long := append(bytes.Clone(s1ToN1Golden), 0xde, 0xad)

	back, err := ParseS1ModeToN1ModeNASTransparentContainer(long)
	if err != nil {
		t.Fatal(err)
	}

	if want := []byte{0x00, 0x00, 0xde, 0xad}; !bytes.Equal(back.Spare, want) {
		t.Fatalf("spare = % x, want % x", back.Spare, want)
	}

	protected, err := back.MACProtected()
	if err != nil {
		t.Fatal(err)
	}

	if want := long[4:]; !bytes.Equal(protected, want) {
		t.Fatalf("MAC-protected span = % x, want % x", protected, want)
	}

	raw, err := back.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(raw, long) {
		t.Fatalf("re-encode = % x, want % x", raw, long)
	}
}

func TestS1ModeToN1ModeContainerRefusesUnencodableValues(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*S1ModeToN1ModeNASTransparentContainer)
	}{
		{"short value part", func(c *S1ModeToN1ModeNASTransparentContainer) { c.Spare = []byte{0x00} }},
		{"NCC wider than three bits", func(c *S1ModeToN1ModeNASTransparentContainer) { c.NCC = 8 }},
		{"ciphering algorithm wider than four bits", func(c *S1ModeToN1ModeNASTransparentContainer) {
			c.CipheringAlgorithm = 0x10
		}},
		{"integrity algorithm wider than four bits", func(c *S1ModeToN1ModeNASTransparentContainer) {
			c.IntegrityAlgorithm = 0x10
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := s1ToN1Container
			tc.mut(&c)

			if _, err := c.MarshalBinary(); err == nil {
				t.Error("want an error, got none")
			}
		})
	}

	for _, b := range [][]byte{{}, make([]byte, s1ModeToN1ModeContainerLen-1)} {
		if _, err := ParseS1ModeToN1ModeNASTransparentContainer(b); err == nil {
			t.Errorf("%d octets: want an error, got none", len(b))
		}
	}
}

// TestInterworkingElementsDegradeSoftly pins TS 24.501 §7.7.1 for the elements
// this tranche started modelling. Each was previously delimited and preserved
// without being decoded, so a malformed one could not fail anything; now that
// they are parsed, a syntactically incorrect optional element must still be
// treated as not present and kept for re-encoding rather than rejecting the
// message. None of them is a security element, so none is Critical.
func TestInterworkingElementsDegradeSoftly(t *testing.T) {
	tests := []struct {
		name string
		iei  uint8
		tlve bool
		bad  []byte
	}{
		// UE status is one octet; two is malformed.
		{"UE status", ieiUEStatus, false, []byte{0x00, 0x00}},
		// Type of identity 010 is the 5G-GUTI, which needs eleven octets;
		// one is a truncated identity, not the "no identity" type 000.
		{"Additional GUTI", ieiAdditionalGUTI, true, []byte{0x02}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var o nas.OptionalWriter
			if tc.tlve {
				o.TLVE(tc.iei, tc.bad)
			} else {
				o.TLV(tc.iei, tc.bad)
			}

			w := nas.NewWriter(nil)
			writeGMMHeader(w, MsgRegistrationRequest)
			w.U8(registrationHeader{RegistrationType: RegistrationTypeMobilityUpdating}.octet())

			mi, err := GUTIIdentity(GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}}).MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}

			w.LVE(mi)
			o.WriteTo(w)

			raw, err := w.Bytes()
			if err != nil {
				t.Fatal(err)
			}

			msg, err := ParseRegistrationRequest(raw)
			if err == nil {
				t.Fatal("a malformed element decoded without even a soft error")
			}

			if !nas.SoftOnly(err) {
				t.Fatalf("err = %v, want a soft error so §7.7.1 leaves the element absent", err)
			}

			if msg == nil {
				t.Fatal("no message returned for a soft error")
			}

			if msg.UEStatus != nil || msg.AdditionalGUTI != nil {
				t.Errorf("a malformed element was decoded anyway: %+v", msg)
			}

			// Preserved, so the element still re-encodes.
			if len(msg.Unrecognized) != 1 || msg.Unrecognized[0].IEI != tc.iei {
				t.Fatalf("unrecognized = %+v, want the malformed element preserved", msg.Unrecognized)
			}
		})
	}
}
