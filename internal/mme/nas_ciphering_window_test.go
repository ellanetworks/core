// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func uplinkOn(t *testing.T, ue *UeContext, plain []byte, sht eps.SecurityHeaderType, sqn uint8) []byte {
	t.Helper()

	sc := mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())

	wire, err := eps.Protect(plain, sht, nas.MakeCount(0, sqn), nas.DirectionUplink, sc)
	if err != nil {
		t.Fatalf("protect uplink message: %v", err)
	}

	return wire
}

func encodePlainEPSSecurityModeReject(t *testing.T) []byte {
	t.Helper()

	payload, err := (&eps.SecurityModeReject{Cause: eps.EMMCauseSecurityModeRejectedUnspecified}).MarshalBinary()
	if err != nil {
		t.Fatalf("encode plain SecurityModeReject: %v", err)
	}

	return payload
}

// TS 24.301 §4.4.5
func TestDecodeNASMessageAcceptsAnUncipheredSecurityModeRejectBeforeCipheringStarts(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	ue.Conn().SetSecureExchangeEstablishedForTest(true)

	if ue.Conn().CipheringStarted() {
		t.Fatal("the MME counts itself as ciphering before it has replied to the UE")
	}

	if _, err := DecodeNASMessage(ue, uplinkOn(t, ue, encodePlainEPSSecurityModeReject(t), eps.SHTIntegrityProtected, 0)); err != nil {
		t.Fatalf("DecodeNASMessage(unciphered SECURITY MODE REJECT) = %v, want it accepted so the MME can abort the procedure", err)
	}
}

func TestDecodeNASMessageDiscardsAnUncipheredSecurityModeRejectAfterCipheringStarts(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	ue.Conn().SetSecureExchangeEstablishedForTest(true)
	ue.Conn().MarkCipheringStarted()

	res, err := DecodeNASMessage(ue, uplinkOn(t, ue, encodePlainEPSSecurityModeReject(t), eps.SHTIntegrityProtected, 0))
	if err == nil {
		t.Fatalf("DecodeNASMessage(unciphered SECURITY MODE REJECT) = %+v, nil, want it discarded once ciphering started", res)
	}
}
