// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// TS 24.301 §4.4.5.
func TestDecodeNASMessageDiscardsAnUncipheredESMMessage(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	esm, err := (&eps.PDNConnectivityRequest{PTI: 1, RequestType: 1, PDNType: 1}).MarshalBinary()
	if err != nil {
		t.Fatalf("marshal PDN CONNECTIVITY REQUEST: %v", err)
	}

	sc := mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())

	wire, err := eps.Protect(esm, eps.SHTIntegrityProtected, nas.MakeCount(0, 0), nas.DirectionUplink, sc)
	if err != nil {
		t.Fatalf("protect: %v", err)
	}

	res, err := DecodeNASMessage(ue, wire)
	if err == nil {
		t.Fatalf("DecodeNASMessage(unciphered ESM) = %+v, nil, want it discarded", res)
	}

	if res != nil {
		t.Errorf("DecodeNASMessage returned %+v alongside the error, want no message", res)
	}
}

// Pins the discard above to the ciphering requirement, not the message itself.
func TestDecodeNASMessageAcceptsTheSameESMMessageCiphered(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	esm, err := (&eps.PDNConnectivityRequest{PTI: 1, RequestType: 1, PDNType: 1}).MarshalBinary()
	if err != nil {
		t.Fatalf("marshal PDN CONNECTIVITY REQUEST: %v", err)
	}

	sc := mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())

	wire, err := eps.Protect(esm, eps.SHTIntegrityProtectedCiphered, nas.MakeCount(0, 0), nas.DirectionUplink, sc)
	if err != nil {
		t.Fatalf("protect: %v", err)
	}

	if _, err := DecodeNASMessage(ue, wire); err != nil {
		t.Fatalf("DecodeNASMessage(ciphered ESM) = %v, want it accepted", err)
	}
}

// The two TS 24.301 §4.4.5 exempts.
func TestDecodeNASMessageAcceptsTheUncipheredExemptMessages(t *testing.T) {
	tests := []struct {
		name string
		msg  interface{ MarshalBinary() ([]byte, error) }
	}{
		{"ATTACH REQUEST", &eps.AttachRequest{
			EPSAttachType:       eps.AttachTypeEPS,
			NASKeySetIdentifier: nas.KeySetIdentifier{Value: 7},
			EPSMobileIdentity:   eps.IMSIIdentity(eps.IMSI(testSubscriber.IMSI)),
			UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
			ESMMessageContainer: []byte{0x02, 0x00, 0xc1},
		}},
		{"TRACKING AREA UPDATE REQUEST", &eps.TrackingAreaUpdateRequest{
			NASKeySetIdentifier: nas.KeySetIdentifier{Value: 7},
			OldGUTI:             eps.IMSIIdentity(eps.IMSI(testSubscriber.IMSI)),
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMME(t)
			ue, _ := securedUE(t, m)

			plain, err := tc.msg.MarshalBinary()
			if err != nil {
				t.Fatalf("marshal %s: %v", tc.name, err)
			}

			sc := mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())

			wire, err := eps.Protect(plain, eps.SHTIntegrityProtected, nas.MakeCount(0, 0), nas.DirectionUplink, sc)
			if err != nil {
				t.Fatalf("protect: %v", err)
			}

			if _, err := DecodeNASMessage(ue, wire); err != nil {
				t.Fatalf("DecodeNASMessage(unciphered %s) = %v, want it accepted", tc.name, err)
			}
		})
	}
}
