// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// TS 33.501 §8.5.2
func epsFramedTAU(t *testing.T, ue *UeContext, count nas.Count, bearer nas.Bearer) []byte {
	t.Helper()

	return epsFramedTAUCitingKSI(t, ue, count, bearer, 0)
}

// epsFramedTAUCitingKSI frames a TRACKING AREA UPDATE REQUEST whose NAS key set
// identifier half-octet names eksi (TS 24.301 §8.2.29.1).
func epsFramedTAUCitingKSI(t *testing.T, ue *UeContext, count nas.Count, bearer nas.Bearer, eksi uint8) []byte {
	t.Helper()

	return epsFramedEMM(t, ue, count, bearer, eps.MsgTrackingAreaUpdateRequest, eksi<<4|0x0b)
}

// epsFramedEMM frames a plain EMM message of type mt whose octet 3 is octet3,
// integrity protected as a 5G NAS message over 3GPP access (TS 33.501 §8.5.2).
func epsFramedEMM(t *testing.T, ue *UeContext, count nas.Count, bearer nas.Bearer, mt eps.MessageType, octet3 byte) []byte {
	t.Helper()

	plain := []byte{uint8(eps.PDEMM), uint8(mt), octet3}

	mac, err := ue.sc.MAC(append([]byte{count.SQN()}, plain...), count, bearer, nas.DirectionUplink)
	if err != nil {
		t.Fatalf("MAC: %v", err)
	}

	raw, err := (&eps.SecurityProtectedMessage{
		SecurityHeaderType: eps.SHTIntegrityProtected,
		MAC:                mac,
		SequenceNumber:     count.SQN(),
		UnverifiedPayload:  plain,
	}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	return raw
}

// TS 33.501 §8.5.2 step 4
func TestVerifyEnclosedEPSNASReturnsTheCountItVerifiedAt(t *testing.T) {
	ue := newSecuredUE(t)

	cnt, err := ue.ulCount.Estimate(0)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}

	got, err := ue.VerifyEnclosedEPSNAS(epsFramedTAU(t, ue, cnt, nas.Bearer3GPP))
	if err != nil {
		t.Fatalf("VerifyEnclosedEPSNAS: %v", err)
	}

	if got != cnt {
		t.Errorf("count = %d, want %d", got, cnt)
	}
}

// TS 33.501 §8.5.2 step 1
func TestVerifyEnclosedEPSNASCommitsTheCount(t *testing.T) {
	ue := newSecuredUE(t)

	cnt, err := ue.ulCount.Estimate(0)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}

	tau := epsFramedTAU(t, ue, cnt, nas.Bearer3GPP)

	if _, err := ue.VerifyEnclosedEPSNAS(tau); err != nil {
		t.Fatalf("VerifyEnclosedEPSNAS: %v", err)
	}

	if _, err := ue.VerifyEnclosedEPSNAS(tau); err == nil {
		t.Fatal("a replayed TAU REQUEST verified a second time")
	}

	if ue.ulCount.LastAccepted() != cnt {
		t.Errorf("uplink NAS COUNT = %d, want the %d the TAU was accepted at", ue.ulCount.LastAccepted(), cnt)
	}
}

func TestVerifyEnclosedEPSNASRefusesTheEPSBearer(t *testing.T) {
	ue := newSecuredUE(t)

	cnt, err := ue.ulCount.Estimate(0)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}

	if _, err := ue.VerifyEnclosedEPSNAS(epsFramedTAU(t, ue, cnt, nas.BearerEPS)); !errors.Is(err, nas.ErrMACMismatch) {
		t.Errorf("error = %v, want a MAC mismatch", err)
	}

	if ue.ulCount.Accepted() {
		t.Error("a refused message advanced the uplink NAS COUNT")
	}
}

func TestVerifyEnclosedEPSNASWithoutASecurityContext(t *testing.T) {
	ue := newSecuredUE(t)

	cnt, err := ue.ulCount.Estimate(0)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}

	tau := epsFramedTAU(t, ue, cnt, nas.Bearer3GPP)

	ue.secured = false

	if _, err := ue.VerifyEnclosedEPSNAS(tau); !errors.Is(err, ErrNo5GSecurityContext) {
		t.Errorf("error = %v, want no 5G security context", err)
	}
}
