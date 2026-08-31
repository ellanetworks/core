// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	mmes1ap "github.com/ellanetworks/core/internal/mme/s1ap"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

var attackerKey = [16]byte{
	0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA,
	0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA,
}

func TestDetachSubscriberNetworkInitiated(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	m.DetachSubscriber(context.Background(), testSubscriber.IMSI)

	if len(cc.sent) != 1 {
		t.Fatalf("expected network Detach Request, got %d", len(cc.sent))
	}

	if ue.EMMState() != mme.EMMDeregistrationInitiated {
		t.Fatal("UE not EMM-DEREGISTERED-INITIATED after network-initiated detach")
	}

	wire := decodeDownlinkNAS(t, cc.sent[0])

	plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionDownlink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
	if err != nil {
		t.Fatalf("Detach Request failed integrity check: %v", err)
	}

	if _, err := eps.ParseDetachRequestNetwork(plain); err != nil {
		t.Fatalf("not a network-originating Detach Request: %v", err)
	}

	handleDetachAccept(context.Background(), m, ue, ue.Conn())
	parseUEContextReleaseCommand(t, cc.sent[1])

	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: s1ap.Ptr(ue.Conn().MMEUES1APID), ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(7))}

	b, _ := complete.Marshal()
	pdu, _ := s1ap.Unmarshal(b)

	mmes1ap.HandleUEContextReleaseComplete(m, context.Background(), mme.NewRadioForTest(cc), pdu.(*s1ap.SuccessfulOutcome).Value)

	if _, ok := m.LookupUeByIMSI(ue.IMSI()); ok {
		t.Fatal("UE context not deleted after network-initiated detach")
	}
}

// TS 24.301 §4.4.4.3
func TestPlainDetachOnSecuredUEDiscarded(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	detach := &eps.DetachRequestUE{
		TypeOfDetach:      1,
		EPSMobileIdentity: eps.GUTIIdentity(eps.GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 1, MMECode: 1, TMSI: [4]byte{0x00, 0x00, 0x00, 0x01}}),
	}

	plain, err := detach.MarshalBinary()
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

// TS 24.301 §4.4.4.3
func TestPlainDetachSecuredUEFreshConnectionRejected(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.Conn().SetSecureExchangeEstablishedForTest(false)

	plain, err := (&eps.DetachRequestUE{
		TypeOfDetach:      eps.DetachTypeEPS,
		EPSMobileIdentity: eps.GUTIIdentity(eps.GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 1, MMECode: 1, TMSI: [4]byte{0x00, 0x00, 0x00, 0x01}}),
	}).MarshalBinary()
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
		EPSMobileIdentity: eps.GUTIIdentity(eps.GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 1, MMECode: 1, TMSI: [4]byte{0x00, 0x00, 0x00, 0x01}}),
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

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

	if len(cc.sent) != 1 {
		t.Fatalf("expected 1 S1AP message (UE Context Release Command), got %d", len(cc.sent))
	}

	cmd := parseUEContextReleaseCommand(t, cc.sent[0])
	if !cmd.UES1APIDs.Pair || cmd.UES1APIDs.MMEUES1APID != ue.Conn().MMEUES1APID || cmd.UES1APIDs.ENBUES1APID != 7 {
		t.Fatalf("unexpected release command IDs: %+v", cmd.UES1APIDs)
	}

	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: s1ap.Ptr(ue.Conn().MMEUES1APID), ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(7))}

	b, _ := complete.Marshal()
	pdu, _ := s1ap.Unmarshal(b)

	mmes1ap.HandleUEContextReleaseComplete(m, context.Background(), mme.NewRadioForTest(cc), pdu.(*s1ap.SuccessfulOutcome).Value)

	if _, ok := m.LookupUeByIMSI(ue.IMSI()); ok {
		t.Fatal("UE context not deleted after release complete")
	}
}

// TS 24.301 §4.4.4.3
func TestDetachSwitchOffUnverifiableIgnoredForSecuredUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	plain, err := (&eps.DetachRequestUE{
		SwitchOff: true, TypeOfDetach: eps.DetachTypeEPS,
		EPSMobileIdentity: eps.GUTIIdentity(eps.GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 1, MMECode: 1, TMSI: [4]byte{0x00, 0x00, 0x00, 0x01}}),
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, 0, nas.DirectionUplink, mustSecurityContext(t, nas.IntegrityNull, nas.CipheringNull, attackerKey, attackerKey))
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

// TS 24.301 §4.4.4.3
func TestDetachSwitchOffUnsecuredAccepted(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.SetSecuredForTest(false)
	ue.Conn().SetSecureExchangeEstablishedForTest(false)

	plain, err := (&eps.DetachRequestUE{
		SwitchOff: true, TypeOfDetach: eps.DetachTypeEPS,
		EPSMobileIdentity: eps.GUTIIdentity(eps.GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 1, MMECode: 1, TMSI: [4]byte{0x00, 0x00, 0x00, 0x01}}),
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, 0, nas.DirectionUplink, mustSecurityContext(t, nas.IntegrityNull, nas.CipheringNull, attackerKey, attackerKey))
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

	if len(cc.sent) != 2 {
		t.Fatalf("expected Detach Accept + Release Command, got %d", len(cc.sent))
	}

	acceptWire := decodeDownlinkNAS(t, cc.sent[0])

	plain, err := unprotected(eps.Unprotect(acceptWire, nas.MakeCount(0, acceptWire[5]), nas.DirectionDownlink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
	if err != nil {
		t.Fatalf("Detach Accept failed integrity check: %v", err)
	}

	if _, err := eps.ParseDetachAccept(plain); err != nil {
		t.Fatalf("not a Detach Accept: %v", err)
	}

	parseUEContextReleaseCommand(t, cc.sent[1])
}

func detachRequest(t *testing.T, ue *mme.UeContext, switchOff bool) []byte {
	t.Helper()

	plain, err := (&eps.DetachRequestUE{
		SwitchOff: switchOff, TypeOfDetach: eps.DetachTypeEPS,
		EPSMobileIdentity: eps.GUTIIdentity(eps.GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 1, MMECode: 1, TMSI: [4]byte{0x00, 0x00, 0x00, 0x01}}),
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nas.MakeCount(0, uint8(ue.ULCount())), nas.DirectionUplink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest()))
	if err != nil {
		t.Fatal(err)
	}

	return wire
}

// TS 24.301 §5.5.2.2.2
func TestDetachOfAUEHoldingAPDNStillReleasesTheContextAfterTheAccept(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	testPDN(ue)

	HandleNAS(context.Background(), m, ue.Conn(), detachRequest(t, ue, false))

	if len(cc.sent) != 2 {
		t.Fatalf("sent %d messages, want Detach Accept then UE Context Release Command", len(cc.sent))
	}

	acceptWire := decodeDownlinkNAS(t, cc.sent[0])

	plain, err := unprotected(eps.Unprotect(acceptWire, nas.MakeCount(0, acceptWire[5]), nas.DirectionDownlink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
	if err != nil {
		t.Fatalf("Detach Accept failed integrity check: %v", err)
	}

	if _, err := eps.ParseDetachAccept(plain); err != nil {
		t.Fatalf("first message is not a Detach Accept: %v", err)
	}

	parseUEContextReleaseCommand(t, cc.sent[1])
}

// TS 24.301 §5.5.2.3.2
func TestDetachAcceptReleasesWithTheDetachCause(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	testPDN(ue)

	handleDetachAccept(context.Background(), m, ue, ue.Conn())

	if len(cc.sent) != 1 {
		t.Fatalf("sent %d messages, want 1 UE Context Release Command", len(cc.sent))
	}

	cmd := parseUEContextReleaseCommand(t, cc.sent[0])
	if cmd.Cause == nil || *cmd.Cause != mme.CauseNASDetach {
		t.Errorf("release cause = %+v, want NAS detach: releasing the last PDN first claims the release, so the detach's own cause never reaches the eNB", cmd.Cause)
	}
}
