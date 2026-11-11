// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func TestBuildAttachRequest(t *testing.T) {
	ue := &UE{IMSI: "001010000000001", netCapEEA: 0xf0, netCapEIA: 0x70}

	b, err := ue.buildAttachRequest()
	if err != nil {
		t.Fatal(err)
	}

	req, err := eps.ParseAttachRequest(b)
	if err != nil {
		t.Fatalf("parse attach request: %v", err)
	}

	if req.EPSAttachType != eps.AttachTypeEPS {
		t.Fatalf("attach type = %d, want %d", req.EPSAttachType, eps.AttachTypeEPS)
	}

	if imsi := req.EPSMobileIdentity.IMSI; imsi == nil || string(*imsi) != ue.IMSI {
		t.Fatalf("identity = %+v, want IMSI %s", req.EPSMobileIdentity, ue.IMSI)
	}

	if !req.UENetworkCapability.SupportsEEA(0) || len(req.ESMMessageContainer) == 0 {
		t.Fatalf("missing UE network capability or ESM container")
	}
}

// TestUEKeyDerivationRoundTrip checks the UE's NAS-key derivation and algorithm
// mapping are self-consistent: a message protected with the derived keys
// unprotects back to the original.
func TestUEKeyDerivationRoundTrip(t *testing.T) {
	for _, alg := range []uint8{0, 1, 2} { // null, SNOW3G, AES
		ue := &UE{kasme: make([]byte, 32), eea: alg, eia: alg}
		for i := range ue.kasme {
			ue.kasme[i] = byte(i + 1)
		}

		if err := ue.deriveNASKeys(); err != nil {
			t.Fatalf("alg %d: derive NAS keys: %v", alg, err)
		}

		plain := []byte{0x07, 0x42, 0x01, 0x02, 0x03}

		wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nas.MakeCount(0, 0), nas.DirectionUplink, mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, ue.knasInt, ue.knasEnc))
		if err != nil {
			t.Fatalf("alg %d: protect: %v", alg, err)
		}

		back, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionUplink, mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, ue.knasInt, ue.knasEnc)))
		if err != nil {
			t.Fatalf("alg %d: unprotect: %v", alg, err)
		}

		if !bytes.Equal(back, plain) {
			t.Fatalf("alg %d: round-trip = % x, want % x", alg, back, plain)
		}
	}
}

// TestHandleAuthenticationRequest checks the UE computes a RES and derives a
// 256-bit K_ASME from a challenge (the Milenage + KDF wiring).
func TestHandleAuthenticationRequest(t *testing.T) {
	ue := &UE{plmn: []byte{0x00, 0xf1, 0x10}}
	for i := range ue.K {
		ue.K[i] = byte(i)
		ue.OPc[i] = byte(0xff - i)
	}

	var rand [16]byte
	for i := range rand {
		rand[i] = byte(i + 1)
	}

	req, err := (&eps.AuthenticationRequest{RAND: rand, AUTN: [16]byte{}}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	respNAS, err := ue.handleAuthenticationRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	if len(ue.kasme) != 32 {
		t.Fatalf("K_ASME length = %d, want 32", len(ue.kasme))
	}

	resp, err := eps.ParseAuthenticationResponse(respNAS)
	if err != nil {
		t.Fatalf("parse authentication response: %v", err)
	}

	if len(resp.RES) != 8 {
		t.Fatalf("RES length = %d, want 8", len(resp.RES))
	}
}

func authenticationRequestFor(t *testing.T, ue *UE, sqn []byte) []byte {
	t.Helper()

	var rand [16]byte
	for i := range rand {
		rand[i] = byte(0x30 + i)
	}

	res, ck, ik, ak, akStar := make([]byte, 8), make([]byte, 16), make([]byte, 16), make([]byte, 6), make([]byte, 6)
	if err := udm.F2345(ue.OPc[:], ue.K[:], rand[:], res, ck, ik, ak, akStar); err != nil {
		t.Fatal(err)
	}

	amf := []byte{0x80, 0x00}
	macA, macS := make([]byte, 8), make([]byte, 8)

	if err := udm.F1(ue.OPc[:], ue.K[:], rand[:], sqn, amf, macA, macS); err != nil {
		t.Fatal(err)
	}

	var autn [16]byte
	for i := range 6 {
		autn[i] = sqn[i] ^ ak[i]
	}

	copy(autn[6:], amf)
	copy(autn[8:], macA)

	req, err := (&eps.AuthenticationRequest{RAND: rand, AUTN: autn}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	return req
}

func TestTrackedSequenceNumberResynchronisesAStaleChallenge(t *testing.T) {
	ue := &UE{plmn: []byte{0x99, 0xf9, 0x07}}
	for i := range ue.K {
		ue.K[i] = byte(i)
		ue.OPc[i] = byte(0xff - i)
	}

	ue.TrackSequenceNumber([6]byte{0, 0, 0, 0, 0, 0x40})

	failure, err := ue.handleAuthenticationRequest(authenticationRequestFor(t, ue, []byte{0, 0, 0, 0, 0, 0x20}))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := eps.ParseAuthenticationFailure(failure)
	if err != nil {
		t.Fatalf("parse authentication failure: %v", err)
	}

	if msg.Cause != eps.EMMCauseSynchFailure || len(msg.AUTS) != 14 || !ue.resyncPending || ue.Resynchronisations() != 1 {
		t.Fatalf("failure = %+v, pending %v, resyncs %d; want a synch failure with a 14-octet AUTS", msg, ue.resyncPending, ue.Resynchronisations())
	}

	resp, err := ue.handleAuthenticationRequest(authenticationRequestFor(t, ue, []byte{0, 0, 0, 0, 0, 0x60}))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := eps.ParseAuthenticationResponse(resp); err != nil || ue.resyncPending {
		t.Fatalf("fresh challenge: %v (pending %v), want an Authentication Response", err, ue.resyncPending)
	}

	if !bytes.Equal(ue.sqn, []byte{0, 0, 0, 0, 0, 0x60}) {
		t.Fatalf("tracked SQN = % x, want the accepted 00 00 00 00 00 60", ue.sqn)
	}
}

// TS 24.301 §4.4.3.2
func TestDownlinkCounterRejectsAReplay(t *testing.T) {
	var d downlinkCounter

	for _, sqn := range []uint8{1, 2, 3} {
		c := d.estimate(sqn)
		if err := d.admissible(c); err != nil {
			t.Fatalf("count %d: %v", c.Value(), err)
		}

		d.accept(c)
	}

	replayed := d.estimate(2)
	if err := d.admissible(replayed); err == nil {
		t.Fatal("a downlink NAS COUNT was accepted twice")
	}

	if err := d.admissible(d.estimate(3)); err == nil {
		t.Fatal("the most recent downlink NAS COUNT was accepted twice")
	}
}

func TestDownlinkCounterAcceptsOutOfOrderDelivery(t *testing.T) {
	var d downlinkCounter

	for _, sqn := range []uint8{1, 3, 2, 4} {
		c := d.estimate(sqn)
		if err := d.admissible(c); err != nil {
			t.Fatalf("count %d out of order: %v", c.Value(), err)
		}

		d.accept(c)
	}

	if err := d.admissible(d.estimate(2)); err == nil {
		t.Fatal("an out-of-order count was accepted twice")
	}
}

func TestDownlinkCounterKeepsTheHandoverFloor(t *testing.T) {
	var d downlinkCounter

	d.seed(nas.MakeCount(0, 8))

	if err := d.admissible(d.estimate(8)); err == nil {
		t.Fatal("the count the mapped context was taken over with was accepted again")
	}

	next := d.estimate(9)
	if err := d.admissible(next); err != nil {
		t.Fatalf("the count after the floor was refused: %v", err)
	}
}
