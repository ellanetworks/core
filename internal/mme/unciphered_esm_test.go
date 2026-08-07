// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// TS 24.301 §4.4.5: once ciphering has started the receiver discards a message
// that should have been ciphered. Only two EMM messages are exempt, so every ESM
// message arriving integrity-protected only is discarded — an attacker cannot
// replay one in the clear against a UE whose ciphering is up.
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

// The same message ciphered is accepted, so the discard above is the ciphering
// requirement and not a rejection of the message itself.
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
