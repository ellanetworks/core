// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"context"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	mmes1ap "github.com/ellanetworks/core/internal/mme/s1ap"
	"github.com/ellanetworks/core/internal/models"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

// idleRegisteredUE returns a secured, EMM-REGISTERED UE with an assigned GUTI,
// parked in ECM-IDLE — the state a UE is in just before a Service Request.
func idleRegisteredUE(t *testing.T, m *mme.MME) (*mme.UeContext, eps.EPSMobileIdentity) {
	t.Helper()

	ue, _ := securedUE(t, m)
	ue.UeNetCap = eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}.Marshal()
	testPDN(ue).SgwFTEID = testSGWFTEID // S-GW S1-U persists across idle, as after a real attach
	guti := m.ReallocateGUTI(ue, models.PlmnID{Mcc: "001", Mnc: "01"}, 1, 1)
	m.FreeUeConn(ue)

	return ue, guti
}

// serviceRequestNAS builds the 4-octet SERVICE REQUEST a UE would send at its
// current uplink NAS COUNT.
func serviceRequestNAS(t *testing.T, ue *mme.UeContext) []byte {
	t.Helper()

	octet0 := uint8(eps.SHTServiceRequest)<<4 | 0x07 // security header type | PD (EMM)
	octet1 := uint8(ue.ULCount()) & 0x1f             // KSI 0 | 5-bit sequence

	mac, err := eps.ServiceRequestShortMAC([]byte{octet0, octet1}, ue.KnasIntForTest(), ue.ULCount(),
		nascommon.DirectionUplink, mme.IntegrityAlg(ue.EIA()))
	if err != nil {
		t.Fatal(err)
	}

	return []byte{octet0, octet1, mac[0], mac[1]}
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
		STMSI:       &s1ap.STMSI{MMEC: 1, MTMSI: guti.MTMSI},
	}

	HandleServiceRequest(m, context.Background(), cc, msg)

	if !ue.Connected() {
		t.Fatal("UE not ECM-CONNECTED after Service Request")
	}

	if ue.Conn().ENBUES1APID != 9 {
		t.Fatalf("UE not bound to the new eNB UE id, got %d", ue.Conn().ENBUES1APID)
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected Initial Context Setup Request, got %d S1AP messages", len(cc.sent))
	}

	ics := parseInitialContextSetup(t, cc.sent[0])

	// The stored UE Radio Capability is replayed on reconnect (TS 23.401 §5.11.2).
	if !bytes.Equal(ics.UERadioCapability, radioCap) {
		t.Fatalf("ICS UE Radio Capability = %x, want %x", ics.UERadioCapability, radioCap)
	}
}

// TestServiceRequestReactivatesAllBearers verifies a multi-PDN UE resuming from
// ECM-IDLE has every active EPS bearer set up in one Initial Context Setup — there
// is no per-bearer data-status IE in the S1 Service Request, so all active bearers
// are reactivated (TS 23.401 §5.3.4.1). The Attach Accept NAS PDU is absent on a
// Service Request, so no bearer carries one.
func TestServiceRequestReactivatesAllBearers(t *testing.T) {
	m := newTestMME(t)
	ue, guti := idleRegisteredUE(t, m)

	// A second PDN connection, as a UE with two APNs would hold across idle.
	const secondEBI = mme.DefaultERABID + 1

	second := ue.EnsurePDN(secondEBI)
	second.SgwFTEID = models.FTEID{TEID: 0x5678, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 3})}
	second.Qci = 8
	second.Arp = 10

	cc := &captureConn{}
	HandleServiceRequest(m, context.Background(), cc, &s1ap.InitialUEMessage{
		ENBUES1APID: 9,
		NASPDU:      s1ap.NASPDU(serviceRequestNAS(t, ue)),
		STMSI:       &s1ap.STMSI{MMEC: 1, MTMSI: guti.MTMSI},
	})

	if len(cc.sent) != 1 {
		t.Fatalf("expected Initial Context Setup Request, got %d S1AP messages", len(cc.sent))
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

// TestServiceRequestS1UTransportFamily checks the S1-U endpoint the MME signals
// to the eNB carries the S-GW N3 address family — IPv4 (4 octets), IPv6 (16), or
// dual-stack (20, IPv4||IPv6) — per TS 36.413, matching the configured N3.
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
			HandleServiceRequest(m, context.Background(), cc, &s1ap.InitialUEMessage{
				ENBUES1APID: 9,
				NASPDU:      s1ap.NASPDU(serviceRequestNAS(t, ue)),
				STMSI:       &s1ap.STMSI{MMEC: 1, MTMSI: guti.MTMSI},
			})

			if len(cc.sent) != 1 {
				t.Fatalf("expected Initial Context Setup Request, got %d S1AP messages", len(cc.sent))
			}

			ics := parseInitialContextSetup(t, cc.sent[0])
			got := []byte(ics.ERABToBeSetup[0].TransportLayerAddress)

			if !bytes.Equal(got, tc.wantTLA) {
				t.Fatalf("S1-U transport layer address = %x, want %x", got, tc.wantTLA)
			}
		})
	}
}

// TestServiceRequestAllocatesFreshMMEUES1APID checks that a UE returning from
// ECM-IDLE is bound to a fresh MME-UE-S1AP-ID and indexed under it (TS 36.413
// §3.1); an idle UE holds no connection identity until it returns.
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
		STMSI:       &s1ap.STMSI{MMEC: 1, MTMSI: guti.MTMSI},
	}

	HandleServiceRequest(m, context.Background(), cc, msg)

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

	HandleServiceRequest(m, context.Background(), cc, msg)

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

	nas := serviceRequestNAS(t, ue)
	nas[3] ^= 0xff // corrupt the short MAC

	cc := &captureConn{}
	msg := &s1ap.InitialUEMessage{
		ENBUES1APID: 9,
		NASPDU:      s1ap.NASPDU(nas),
		STMSI:       &s1ap.STMSI{MMEC: 1, MTMSI: guti.MTMSI},
	}

	HandleServiceRequest(m, context.Background(), cc, msg)

	if ue.Connected() {
		t.Fatal("UE reconnected despite a bad short MAC")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected Service Reject, got %d S1AP messages", len(cc.sent))
	}
}

// A resume (protected Initial UE Message) with an invalid MAC, carrying a
// victim's S-TMSI, must not move the victim's S1 binding (TS 24.301 §4.4.4.3).
func TestResumeBadMACDoesNotRebindVictim(t *testing.T) {
	m := newTestMME(t)
	ue, guti := idleRegisteredUE(t, m)

	nas := protectedUplink(t, ue, nascommon.NASCount(0, 0))
	nas[2] ^= 0xff // corrupt the MAC

	plmn := s1ap.PLMNIdentity{0x00, 0xf1, 0x10}

	b, err := (&s1ap.InitialUEMessage{
		ENBUES1APID:           9,
		NASPDU:                s1ap.NASPDU(nas),
		STMSI:                 &s1ap.STMSI{MMEC: 1, MTMSI: guti.MTMSI},
		TAI:                   s1ap.TAI{PLMNIdentity: plmn, TAC: 1},
		EUTRANCGI:             s1ap.EUTRANCGI{PLMNIdentity: plmn, CellID: 1},
		RRCEstablishmentCause: s1ap.RRCCauseMOSignalling,
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	mmes1ap.HandleInitialUEMessage(m, context.Background(), mme.NewRadioForTest(nil), initiatingValue(t, b))

	if ue.Connected() {
		t.Fatal("a forged resume connected the idle victim")
	}
}

// A Service Request with an invalid short MAC, on a different association, must
// not move the resolved UE's S1 binding (TS 24.301 §5.6.1).
func TestServiceRequestBadMACDoesNotRebindVictim(t *testing.T) {
	m := newTestMME(t)
	ue, guti := idleRegisteredUE(t, m)

	nas := serviceRequestNAS(t, ue)
	nas[3] ^= 0xff

	attacker := &captureConn{}
	HandleServiceRequest(m, context.Background(), attacker, &s1ap.InitialUEMessage{
		ENBUES1APID: 9,
		NASPDU:      s1ap.NASPDU(nas),
		STMSI:       &s1ap.STMSI{MMEC: 1, MTMSI: guti.MTMSI},
	})

	if ue.Connected() {
		t.Fatal("a forged Service Request connected the idle victim to the attacker")
	}
}
