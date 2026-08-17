// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func uplinkOn(t *testing.T, ue *UeContext, plain []byte, sht eps.SecurityHeaderType) []byte {
	t.Helper()

	sc := mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())

	wire, err := eps.Protect(plain, sht, nas.MakeCount(0, 0), nas.DirectionUplink, sc)
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

func encodePlainEPSSecurityModeComplete(t *testing.T) []byte {
	t.Helper()

	payload, err := (&eps.SecurityModeComplete{}).MarshalBinary()
	if err != nil {
		t.Fatalf("encode plain SecurityModeComplete: %v", err)
	}

	return payload
}

func encodePlainEPSEMMStatus(t *testing.T) []byte {
	t.Helper()

	payload, err := (&eps.EMMStatus{Cause: eps.EMMCauseMessageNotCompatible}).MarshalBinary()
	if err != nil {
		t.Fatalf("encode plain EMMStatus: %v", err)
	}

	return payload
}

// TS 24.301 §4.4.2.3, §4.4.5
func TestDecodeNASMessageAcceptsUncipheredOrdinarySignallingBeforeCipheringStarts(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	ue.Conn().SetSecureExchangeEstablishedForTest(true)

	if ue.Conn().CipheringStarted() {
		t.Fatal("the MME counts itself as ciphering before it has replied to the UE")
	}

	if _, err := DecodeNASMessage(ue, uplinkOn(t, ue, encodePlainEPSEMMStatus(t), eps.SHTIntegrityProtected)); err != nil {
		t.Fatalf("DecodeNASMessage(unciphered EMM STATUS) = %v, want it accepted before the MME has replied ciphered", err)
	}
}

// TS 24.301 §4.4.5
func TestDecodeNASMessageDiscardsUncipheredOrdinarySignallingAfterCipheringStarts(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	ue.Conn().SetSecureExchangeEstablishedForTest(true)
	ue.Conn().MarkCipheringStarted()

	res, err := DecodeNASMessage(ue, uplinkOn(t, ue, encodePlainEPSEMMStatus(t), eps.SHTIntegrityProtected))
	if err == nil {
		t.Fatalf("DecodeNASMessage(unciphered EMM STATUS) = %+v, nil, want it discarded once ciphering started", res)
	}
}

// TS 24.301 §4.4.2.3, §4.4.5, §5.4.3.5
func TestDecodeNASMessageAcceptsAnUncipheredSecurityModeRejectBeforeCipheringStarts(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	ue.Conn().SetSecureExchangeEstablishedForTest(true)

	if ue.Conn().CipheringStarted() {
		t.Fatal("the MME counts itself as ciphering before it has replied to the UE")
	}

	if _, err := DecodeNASMessage(ue, uplinkOn(t, ue, encodePlainEPSSecurityModeReject(t), eps.SHTIntegrityProtected)); err != nil {
		t.Fatalf("DecodeNASMessage(unciphered SECURITY MODE REJECT) = %v, want it accepted so the MME can abort the procedure", err)
	}
}

// TS 24.301 §4.4.5, §5.4.3.5
func TestDecodeNASMessageDiscardsAnUncipheredSecurityModeRejectAfterCipheringStarts(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	ue.Conn().SetSecureExchangeEstablishedForTest(true)
	ue.Conn().MarkCipheringStarted()

	res, err := DecodeNASMessage(ue, uplinkOn(t, ue, encodePlainEPSSecurityModeReject(t), eps.SHTIntegrityProtected))
	if err == nil {
		t.Fatalf("DecodeNASMessage(unciphered SECURITY MODE REJECT) = %+v, nil, want it discarded once ciphering started", res)
	}
}

// TS 24.301 §5.4.3.3, table 9.3.1
func TestDecodeNASMessageAcceptsASecurityModeCompleteWithTheNewContextHeaderType(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	ue.Conn().SetSecureExchangeEstablishedForTest(true)

	if _, err := DecodeNASMessage(ue, uplinkOn(t, ue, encodePlainEPSSecurityModeComplete(t), eps.SHTIntegrityProtectedCipheredNewContext)); err != nil {
		t.Fatalf("DecodeNASMessage(SECURITY MODE COMPLETE, new-context header type) = %v, want it accepted", err)
	}

	if !ue.Conn().CipheringStarted() {
		t.Error("a ciphered SECURITY MODE COMPLETE did not start ciphering on the connection")
	}
}

// TS 24.301 §5.4.3.3, table 9.3.1
func TestDecodeNASMessageDiscardsASecurityModeCompleteWithoutTheNewContextHeaderType(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	ue.Conn().SetSecureExchangeEstablishedForTest(true)

	if ue.Conn().CipheringStarted() {
		t.Fatal("the MME counts itself as ciphering before it has replied to the UE")
	}

	res, err := DecodeNASMessage(ue, uplinkOn(t, ue, encodePlainEPSSecurityModeComplete(t), eps.SHTIntegrityProtected))
	if err == nil {
		t.Fatalf("DecodeNASMessage(unciphered SECURITY MODE COMPLETE) = %+v, nil, want it discarded (TS 24.301 §5.4.3.3)", res)
	}
}
