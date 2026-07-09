// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	mmes1ap "github.com/ellanetworks/core/internal/mme/s1ap"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

func TestDetachSubscriberNetworkInitiated(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	m.DetachSubscriber(context.Background(), testSubscriber.IMSI)

	if len(cc.sent) != 1 {
		t.Fatalf("expected network Detach Request, got %d", len(cc.sent))
	}

	// The connected UE stays EMM-DEREGISTERED-INITIATED while T3422 guards the
	// DETACH REQUEST, reaching EMM-DEREGISTERED on Detach Accept (TS 24.301 §5.1.3.2).
	if ue.EMMState() != mme.EMMDeregistrationInitiated {
		t.Fatal("UE not EMM-DEREGISTERED-INITIATED after network-initiated detach")
	}

	wire := decodeDownlinkNAS(t, cc.sent[0])

	plain, err := eps.Unprotect(wire, nascommon.NASCount(0, wire[5]), nascommon.DirectionDownlink,
		ue.KnasIntForTest(), ue.KnasEncForTest(), nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatalf("Detach Request failed integrity check: %v", err)
	}

	if _, err := eps.ParseDetachRequestNetwork(plain); err != nil {
		t.Fatalf("not a network-originating Detach Request: %v", err)
	}

	handleDetachAccept(context.Background(), m, ue)
	parseUEContextReleaseCommand(t, cc.sent[1])

	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: ue.Conn().MMEUES1APID, ENBUES1APID: 7}

	b, _ := complete.Marshal()
	pdu, _ := s1ap.Unmarshal(b)

	mmes1ap.HandleUEContextReleaseComplete(m, context.Background(), mme.NewRadioForTest(cc), pdu.(*s1ap.SuccessfulOutcome).Value)

	if _, ok := m.LookupUeByIMSI(ue.IMSI()); ok {
		t.Fatal("UE context not deleted after network-initiated detach")
	}
}

// TestPlainDetachOnSecuredUEDiscarded asserts TS 24.301 §4.4.4.3: once secure
// exchange of NAS messages is established, a message that is not integrity
// protected is discarded. A spoofed plain DETACH REQUEST injected on a secured
// UE's connection must not deregister or release it.
func TestPlainDetachOnSecuredUEDiscarded(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	detach := &eps.DetachRequestUE{
		TypeOfDetach:      1,
		EPSMobileIdentity: eps.EPSMobileIdentity{Type: eps.IdentityGUTI, MCC: "001", MNC: "01", MMEGroupID: 1, MMECode: 1, MTMSI: 1},
	}

	plain, err := detach.Marshal()
	if err != nil {
		t.Fatalf("marshal detach: %v", err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), plain)

	if ue.EMMState() != mme.EMMRegistered {
		t.Fatal("a plain detach on a secured UE must be discarded, not deregister it (TS 24.301 §4.4.4.3)")
	}

	if _, ok := m.LookupUeByIMSI(ue.IMSI()); !ok {
		t.Fatal("secured UE context must remain after a discarded plain detach")
	}
}

// TestPlainDetachSecuredUEFreshConnectionRejected verifies a secured UE that has
// not yet established secure exchange on this connection (a fresh S1 link, so the
// chokepoint's per-connection guard does not fire) still cannot be deregistered by
// an unprotected DETACH REQUEST: the handler rejects it on ue.Secured(), mirroring
// the AMF (TS 24.301 §4.4.4.3 defense in depth).
func TestPlainDetachSecuredUEFreshConnectionRejected(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.Conn().SetSecureExchangeEstablishedForTest(false) // fresh connection: connSecured is false

	plain, err := (&eps.DetachRequestUE{
		TypeOfDetach:      eps.DetachTypeEPS,
		EPSMobileIdentity: eps.EPSMobileIdentity{Type: eps.IdentityGUTI, MCC: "001", MNC: "01", MMEGroupID: 1, MMECode: 1, MTMSI: 1},
	}).Marshal()
	if err != nil {
		t.Fatalf("marshal detach: %v", err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), plain)

	if ue.EMMState() != mme.EMMRegistered {
		t.Fatal("an unprotected detach from a secured UE on a fresh connection must be rejected")
	}

	if len(cc.sent) != 0 {
		t.Fatalf("no S1AP should be sent for a rejected detach, got %d", len(cc.sent))
	}

	if _, ok := m.LookupUeByIMSI(ue.IMSI()); !ok {
		t.Fatal("secured UE context must remain")
	}
}

func TestForgedMessageIgnoredForSecuredUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	plain, err := (&eps.DetachRequestUE{
		SwitchOff: false, TypeOfDetach: eps.DetachTypeEPS,
		EPSMobileIdentity: eps.EPSMobileIdentity{Type: eps.IdentityGUTI, MCC: "001", MNC: "01", MMEGroupID: 1, MMECode: 1, MTMSI: 1},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// Integrity-protected envelope (SHT=1) with a MAC the MME cannot reproduce.
	forged := append([]byte{0x17, 0xde, 0xad, 0xbe, 0xef, byte(ue.ULCount())}, plain...)

	HandleNAS(context.Background(), m, ue.Conn(), forged)

	if cc.count() != 0 {
		t.Fatalf("forged DETACH against a secured UE was acted on: %d downlink(s) sent", cc.count())
	}

	if _, ok := m.LookupUe(ue.Conn().MMEUES1APID); !ok || ue.EMMState() != mme.EMMRegistered || !ue.SecuredForTest() {
		t.Fatal("secured UE was disrupted by a forged, unverifiable DETACH")
	}
}

func TestDetachSwitchOff(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	HandleNAS(context.Background(), m, ue.Conn(), detachRequest(t, ue, true))

	// Switch-off: no Detach Accept, just the UE Context Release Command.
	if len(cc.sent) != 1 {
		t.Fatalf("expected 1 S1AP message (UE Context Release Command), got %d", len(cc.sent))
	}

	cmd := parseUEContextReleaseCommand(t, cc.sent[0])
	if !cmd.UES1APIDs.Pair || cmd.UES1APIDs.MMEUES1APID != ue.Conn().MMEUES1APID || cmd.UES1APIDs.ENBUES1APID != 7 {
		t.Fatalf("unexpected release command IDs: %+v", cmd.UES1APIDs)
	}

	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: ue.Conn().MMEUES1APID, ENBUES1APID: 7}

	b, _ := complete.Marshal()
	pdu, _ := s1ap.Unmarshal(b)

	mmes1ap.HandleUEContextReleaseComplete(m, context.Background(), mme.NewRadioForTest(cc), pdu.(*s1ap.SuccessfulOutcome).Value)

	if _, ok := m.LookupUeByIMSI(ue.IMSI()); ok {
		t.Fatal("UE context not deleted after release complete")
	}
}

// A switch-off DETACH REQUEST that fails the integrity check is ignored once a
// security context exists (TS 24.301 §4.4.4.3).
func TestDetachSwitchOffUnverifiableIgnoredForSecuredUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	plain, err := (&eps.DetachRequestUE{
		SwitchOff: true, TypeOfDetach: eps.DetachTypeEPS,
		EPSMobileIdentity: eps.EPSMobileIdentity{Type: eps.IdentityGUTI, MCC: "001", MNC: "01", MMEGroupID: 1, MMECode: 1, MTMSI: 1},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// Null algorithms: zero MAC, unciphered payload — unverifiable without the keys.
	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, 0, nascommon.DirectionUplink,
		[16]byte{}, [16]byte{}, nascommon.NullIntegrity{}, nascommon.NullCipher{})
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), wire)

	if len(cc.sent) != 0 {
		t.Fatalf("S1AP messages sent = %d, want 0", len(cc.sent))
	}

	if _, ok := m.LookupUe(ue.Conn().MMEUES1APID); !ok || ue.EMMState() != mme.EMMRegistered || !ue.SecuredForTest() {
		t.Fatal("secured UE state changed by an unverifiable switch-off detach")
	}
}

// Before a security context exists, an unverifiable switch-off detach is honoured
// (TS 24.301 §4.4.4.3).
func TestDetachSwitchOffUnsecuredAccepted(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	// Model a connection on which secure exchange has not been established
	// (TS 24.301 §4.4.4.3): a switch-off detach is then honoured without
	// integrity protection.
	ue.SetSecuredForTest(false)
	ue.Conn().SetSecureExchangeEstablishedForTest(false)

	plain, err := (&eps.DetachRequestUE{
		SwitchOff: true, TypeOfDetach: eps.DetachTypeEPS,
		EPSMobileIdentity: eps.EPSMobileIdentity{Type: eps.IdentityGUTI, MCC: "001", MNC: "01", MMEGroupID: 1, MMECode: 1, MTMSI: 1},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, 0, nascommon.DirectionUplink,
		[16]byte{}, [16]byte{}, nascommon.NullIntegrity{}, nascommon.NullCipher{})
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), wire)

	if len(cc.sent) != 1 {
		t.Fatalf("expected UE Context Release Command, got %d S1AP messages", len(cc.sent))
	}

	parseUEContextReleaseCommand(t, cc.sent[0])
}

func TestDetachNotSwitchOff(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	HandleNAS(context.Background(), m, ue.Conn(), detachRequest(t, ue, false))

	// Not switch-off: Detach Accept (downlink NAS), then UE Context Release Command.
	if len(cc.sent) != 2 {
		t.Fatalf("expected Detach Accept + Release Command, got %d", len(cc.sent))
	}

	acceptWire := decodeDownlinkNAS(t, cc.sent[0])

	plain, err := eps.Unprotect(acceptWire, nascommon.NASCount(0, acceptWire[5]), nascommon.DirectionDownlink,
		ue.KnasIntForTest(), ue.KnasEncForTest(), nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatalf("Detach Accept failed integrity check: %v", err)
	}

	if _, err := eps.ParseDetachAccept(plain); err != nil {
		t.Fatalf("not a Detach Accept: %v", err)
	}

	parseUEContextReleaseCommand(t, cc.sent[1])
}

// detachRequest builds an integrity-protected UE-initiated DETACH REQUEST.
func detachRequest(t *testing.T, ue *mme.UeContext, switchOff bool) []byte {
	t.Helper()

	plain, err := (&eps.DetachRequestUE{
		SwitchOff: switchOff, TypeOfDetach: eps.DetachTypeEPS,
		EPSMobileIdentity: eps.EPSMobileIdentity{Type: eps.IdentityGUTI, MCC: "001", MNC: "01", MMEGroupID: 1, MMECode: 1, MTMSI: 1},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nascommon.NASCount(0, uint8(ue.ULCount())),
		nascommon.DirectionUplink, ue.KnasIntForTest(), ue.KnasEncForTest(), nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatal(err)
	}

	return wire
}
