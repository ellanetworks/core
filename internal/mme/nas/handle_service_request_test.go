// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"context"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	mmes1ap "github.com/ellanetworks/core/internal/mme/s1ap"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

func idleRegisteredUE(t *testing.T, m *mme.MME) (*mme.UeContext, eps.EPSMobileIdentity) {
	t.Helper()

	ue, _ := securedUE(t, m)
	ue.SetUESecurityCapability(eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}, nil, mme.MintAuthProofForAttachRequest())
	testPDN(ue).SgwFTEID = testSGWFTEID

	guti, err := m.ReallocateGUTI(t.Context(), ue, models.PlmnID{Mcc: "001", Mnc: "01"}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}

	m.FreeUeConn(ue)

	return ue, guti
}

func serviceRequestNAS(t *testing.T, ue *mme.UeContext) []byte {
	t.Helper()

	sc := mustSecurityContext(t, ue.EIA(), nas.CipheringNull,
		ue.KnasIntForTest(), nas.CipherKey{})

	sr, err := eps.NewServiceRequest(0, nas.Count(ue.ULCount()), sc)
	if err != nil {
		t.Fatal(err)
	}

	wire, err := sr.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	return wire
}

func TestServiceRequestReestablishes(t *testing.T) {
	m := newTestMME(t)
	ue, guti := idleRegisteredUE(t, m)

	radioCap := []byte{0x10, 0x20, 0x30}
	ue.RadioCapability = radioCap

	cc := &captureConn{}
	msg := &s1ap.InitialUEMessage{
		ENBUES1APID: 9,
		NASPDU:      s1ap.NASPDU(serviceRequestNAS(t, ue)),
		STMSI:       &s1ap.STMSI{MMEC: 1, MTMSI: s1ap.MTMSI(binary.BigEndian.Uint32(guti.GUTI.TMSI[:]))},
	}

	HandleServiceRequest(context.Background(), m, cc, msg)

	if !ue.Connected() {
		t.Fatal("UE not ECM-CONNECTED after Service Request")
	}

	if ue.Conn().ENBUES1APID != 9 {
		t.Fatalf("UE not bound to the new eNB UE id, got %d", ue.Conn().ENBUES1APID)
	}

	if len(cc.sent) != 2 {
		t.Fatalf("expected Initial Context Setup Request + GUTI Reallocation Command, got %d S1AP messages", len(cc.sent))
	}

	ics := parseInitialContextSetup(t, cc.sent[0])

	if !bytes.Equal(ics.UERadioCapability, radioCap) {
		t.Fatalf("ICS UE Radio Capability = %x, want %x", ics.UERadioCapability, radioCap)
	}
}

// TS 23.401 §5.3.4.1
func TestServiceRequestReactivatesAllBearers(t *testing.T) {
	m := newTestMME(t)
	ue, guti := idleRegisteredUE(t, m)

	const secondEBI = mme.DefaultERABID + 1

	second := ue.EnsurePDN(secondEBI)
	second.SgwFTEID = models.FTEID{TEID: 0x5678, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 3})}
	second.Qci = 8
	second.Arp = 10

	cc := &captureConn{}
	HandleServiceRequest(context.Background(), m, cc, &s1ap.InitialUEMessage{
		ENBUES1APID: 9,
		NASPDU:      s1ap.NASPDU(serviceRequestNAS(t, ue)),
		STMSI:       &s1ap.STMSI{MMEC: 1, MTMSI: s1ap.MTMSI(binary.BigEndian.Uint32(guti.GUTI.TMSI[:]))},
	})

	if len(cc.sent) != 2 {
		t.Fatalf("expected Initial Context Setup Request + GUTI Reallocation Command, got %d S1AP messages", len(cc.sent))
	}

	ics := parseInitialContextSetup(t, cc.sent[0])

	got := map[uint8]s1ap.ERABToBeSetupItemCtxtSUReq{}
	for _, e := range ics.ERABToBeSetup {
		got[uint8(e.ERABID)] = e
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 E-RABs (both PDNs), got %d", len(got))
	}

	if _, ok := got[mme.DefaultERABID]; !ok {
		t.Fatalf("default E-RAB %d missing from Initial Context Setup", mme.DefaultERABID)
	}

	if _, ok := got[secondEBI]; !ok {
		t.Fatalf("secondary E-RAB %d missing from Initial Context Setup", secondEBI)
	}

	if uint32(got[secondEBI].GTPTEID) != 0x5678 {
		t.Fatalf("secondary E-RAB S-GW TEID = %#x, want 0x5678", uint32(got[secondEBI].GTPTEID))
	}

	for ebi, e := range got {
		if len(e.NASPDU) != 0 {
			t.Fatalf("E-RAB %d carries a NAS PDU on a Service Request (want none)", ebi)
		}
	}
}

func TestServiceRequestWithNoActiveBearersRejected(t *testing.T) {
	m := newTestMME(t)
	ue, guti := idleRegisteredUE(t, m)

	ue.Pdns = nil

	cc := &captureConn{}
	HandleServiceRequest(context.Background(), m, cc, &s1ap.InitialUEMessage{
		ENBUES1APID: 9,
		NASPDU:      s1ap.NASPDU(serviceRequestNAS(t, ue)),
		STMSI:       &s1ap.STMSI{MMEC: 1, MTMSI: s1ap.MTMSI(binary.BigEndian.Uint32(guti.GUTI.TMSI[:]))},
	})

	if len(cc.sent) == 0 {
		t.Fatal("no S1AP message sent; the UE is left registered with no user plane and no reject")
	}

	for _, pdu := range cc.sent {
		msg, err := s1ap.Unmarshal(pdu)
		if err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if im, ok := msg.(*s1ap.InitiatingMessage); ok && im.ProcedureCode == s1ap.ProcInitialContextSetup {
			t.Fatal("Initial Context Setup sent for a UE with no active EPS bearer context")
		}
	}

	rej, err := eps.ParseServiceReject(decodeProtectedDownlink(t, ue, cc.sent[0]))
	if err != nil {
		t.Fatalf("parse service reject: %v", err)
	}

	if rej.Cause != eps.EMMCauseNoEPSBearerContextActivated {
		t.Fatalf("EMM cause = %d, want %d (no EPS bearer context activated)", rej.Cause, eps.EMMCauseNoEPSBearerContextActivated)
	}
}

func TestServiceRequestS1UTransportFamily(t *testing.T) {
	v4 := netip.AddrFrom4([4]byte{10, 3, 0, 2})
	v6 := netip.MustParseAddr("2001:db8:3::10")
	v6Octets := v6.As16()

	tests := []struct {
		name    string
		sgwV4   netip.Addr
		sgwV6   netip.Addr
		wantTLA []byte
	}{
		{name: "ipv4 s1-u", sgwV4: v4, wantTLA: []byte{10, 3, 0, 2}},
		{name: "ipv6 s1-u", sgwV6: v6, wantTLA: v6Octets[:]},
		{name: "dualstack s1-u", sgwV4: v4, sgwV6: v6, wantTLA: append([]byte{10, 3, 0, 2}, v6Octets[:]...)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMME(t)
			ue, guti := idleRegisteredUE(t, m)
			testPDN(ue).SgwFTEID = models.FTEID{TEID: 0x1234, Addr: tc.sgwV4}
			testPDN(ue).SgwN3IPv6 = tc.sgwV6

			cc := &captureConn{}
			HandleServiceRequest(context.Background(), m, cc, &s1ap.InitialUEMessage{
				ENBUES1APID: 9,
				NASPDU:      s1ap.NASPDU(serviceRequestNAS(t, ue)),
				STMSI:       &s1ap.STMSI{MMEC: 1, MTMSI: s1ap.MTMSI(binary.BigEndian.Uint32(guti.GUTI.TMSI[:]))},
			})

			if len(cc.sent) != 2 {
				t.Fatalf("expected Initial Context Setup Request + GUTI Reallocation Command, got %d S1AP messages", len(cc.sent))
			}

			ics := parseInitialContextSetup(t, cc.sent[0])
			got := []byte(ics.ERABToBeSetup[0].TransportLayerAddress)

			if !bytes.Equal(got, tc.wantTLA) {
				t.Fatalf("S1-U transport layer address = %x, want %x", got, tc.wantTLA)
			}
		})
	}
}

func TestServiceRequestAllocatesFreshMMEUES1APID(t *testing.T) {
	m := newTestMME(t)
	ue, guti := idleRegisteredUE(t, m)

	if ue.Connected() {
		t.Fatal("a UE in ECM-IDLE must hold no S1-connection")
	}

	cc := &captureConn{}
	msg := &s1ap.InitialUEMessage{
		ENBUES1APID: 42,
		NASPDU:      s1ap.NASPDU(serviceRequestNAS(t, ue)),
		STMSI:       &s1ap.STMSI{MMEC: 1, MTMSI: s1ap.MTMSI(binary.BigEndian.Uint32(guti.GUTI.TMSI[:]))},
	}

	HandleServiceRequest(context.Background(), m, cc, msg)

	if !ue.Connected() {
		t.Fatal("UE not bound to a connection after Service Request")
	}

	if got, ok := m.LookupUe(ue.Conn().MMEUES1APID); !ok || got != ue {
		t.Fatal("UE not indexed under its fresh mme.MME-UE-S1AP-ID")
	}
}

func TestServiceRequestUnknownSTMSIRejected(t *testing.T) {
	m := newTestMME(t)

	cc := &captureConn{}
	msg := &s1ap.InitialUEMessage{
		ENBUES1APID: 9,
		NASPDU:      s1ap.NASPDU([]byte{0xc7, 0x00, 0x00, 0x00}),
		STMSI:       &s1ap.STMSI{MMEC: 1, MTMSI: 0xDEADBEEF},
	}

	HandleServiceRequest(context.Background(), m, cc, msg)

	if len(cc.sent) != 1 {
		t.Fatalf("expected Service Reject, got %d S1AP messages", len(cc.sent))
	}

	mt, err := eps.PeekMessageType(decodeDownlinkNAS(t, cc.sent[0]))
	if err != nil {
		t.Fatal(err)
	}

	if mt != eps.MsgServiceReject {
		t.Fatalf("expected Service Reject, got message type %#x", mt)
	}
}

func TestServiceRequestBadMACRejected(t *testing.T) {
	m := newTestMME(t)
	ue, guti := idleRegisteredUE(t, m)

	pdu := serviceRequestNAS(t, ue)
	pdu[3] ^= 0xff

	cc := &captureConn{}
	msg := &s1ap.InitialUEMessage{
		ENBUES1APID: 9,
		NASPDU:      s1ap.NASPDU(pdu),
		STMSI:       &s1ap.STMSI{MMEC: 1, MTMSI: s1ap.MTMSI(binary.BigEndian.Uint32(guti.GUTI.TMSI[:]))},
	}

	HandleServiceRequest(context.Background(), m, cc, msg)

	if ue.Connected() {
		t.Fatal("UE reconnected despite a bad short MAC")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected Service Reject, got %d S1AP messages", len(cc.sent))
	}
}

// TS 24.301 §5.6.1.7
func TestServiceRequestProtocolErrorRejected96(t *testing.T) {
	m := newTestMME(t)
	ue, guti := idleRegisteredUE(t, m)

	malformed := serviceRequestNAS(t, ue)[:2]

	cc := &captureConn{}
	msg := &s1ap.InitialUEMessage{
		ENBUES1APID: 9,
		NASPDU:      s1ap.NASPDU(malformed),
		STMSI:       &s1ap.STMSI{MMEC: 1, MTMSI: s1ap.MTMSI(binary.BigEndian.Uint32(guti.GUTI.TMSI[:]))},
	}

	HandleServiceRequest(context.Background(), m, cc, msg)

	if len(cc.sent) != 1 {
		t.Fatalf("expected Service Reject, got %d S1AP messages", len(cc.sent))
	}

	plain := decodeDownlinkNAS(t, cc.sent[0])

	mt, err := eps.PeekMessageType(plain)
	if err != nil {
		t.Fatal(err)
	}

	if mt != eps.MsgServiceReject {
		t.Fatalf("expected Service Reject, got message type %#x", mt)
	}

	if len(plain) < 3 || eps.EMMCause(plain[2]) != eps.EMMCauseInvalidMandatoryInformation {
		t.Fatalf("EMM cause = %#x, want #96 (invalid mandatory information)", plain)
	}
}

// TS 24.301 §4.4.4.3
func TestResumeBadMACDoesNotRebindVictim(t *testing.T) {
	m := newTestMME(t)
	ue, guti := idleRegisteredUE(t, m)

	pdu := protectedUplink(t, ue, nas.MakeCount(0, 0).Value())
	pdu[2] ^= 0xff

	plmn := s1ap.PLMNIdentity{0x00, 0xf1, 0x10}

	b, err := (&s1ap.InitialUEMessage{
		ENBUES1APID:           9,
		NASPDU:                s1ap.NASPDU(pdu),
		STMSI:                 &s1ap.STMSI{MMEC: 1, MTMSI: s1ap.MTMSI(binary.BigEndian.Uint32(guti.GUTI.TMSI[:]))},
		TAI:                   s1ap.TAI{PLMNIdentity: plmn, TAC: 1},
		EUTRANCGI:             s1ap.Ptr(s1ap.EUTRANCGI{PLMNIdentity: plmn, CellID: 1}),
		RRCEstablishmentCause: s1ap.Ptr(s1ap.RRCCauseMOSignalling),
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	mmes1ap.HandleInitialUEMessage(m, context.Background(), mme.NewRadioForTest(nil), initiatingValue(t, b))

	if ue.Connected() {
		t.Fatal("a forged resume connected the idle victim")
	}
}

// TS 24.301 §5.6.1
func TestServiceRequestBadMACDoesNotRebindVictim(t *testing.T) {
	m := newTestMME(t)
	ue, guti := idleRegisteredUE(t, m)

	pdu := serviceRequestNAS(t, ue)
	pdu[3] ^= 0xff

	attacker := &captureConn{}
	HandleServiceRequest(context.Background(), m, attacker, &s1ap.InitialUEMessage{
		ENBUES1APID: 9,
		NASPDU:      s1ap.NASPDU(pdu),
		STMSI:       &s1ap.STMSI{MMEC: 1, MTMSI: s1ap.MTMSI(binary.BigEndian.Uint32(guti.GUTI.TMSI[:]))},
	})

	if ue.Connected() {
		t.Fatal("a forged Service Request connected the idle victim to the attacker")
	}
}
