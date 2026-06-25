// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"bytes"
	"context"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

// idleRegisteredUE returns a secured, EMM-REGISTERED UE with an assigned GUTI,
// parked in ECM-IDLE — the state a UE is in just before a Service Request.
func idleRegisteredUE(t *testing.T, m *MME) (*UeContext, eps.EPSMobileIdentity) {
	t.Helper()

	ue, _ := securedUE(t, m)
	ue.ueNetCap = eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}.Marshal()
	testPDN(ue).sgwFTEID = testSGWFTEID // S-GW S1-U persists across idle, as after a real attach
	guti := m.assignGUTI(ue, models.PlmnID{Mcc: "001", Mnc: "01"}, 1, 1)
	ue.ecmState.store(ECMIdle)

	return ue, guti
}

// serviceRequestNAS builds the 4-octet SERVICE REQUEST a UE would send at its
// current uplink NAS COUNT.
func serviceRequestNAS(t *testing.T, ue *UeContext) []byte {
	t.Helper()

	octet0 := uint8(eps.SHTServiceRequest)<<4 | 0x07 // security header type | PD (EMM)
	octet1 := uint8(ue.ulCount) & 0x1f               // KSI 0 | 5-bit sequence

	mac, err := eps.ServiceRequestShortMAC([]byte{octet0, octet1}, ue.knasInt, ue.ulCount,
		nascommon.DirectionUplink, integrityAlg(ue.eia))
	if err != nil {
		t.Fatal(err)
	}

	return []byte{octet0, octet1, mac[0], mac[1]}
}

func TestServiceRequestReestablishes(t *testing.T) {
	m := newTestMME(t)
	ue, guti := idleRegisteredUE(t, m)

	radioCap := []byte{0x10, 0x20, 0x30}
	ue.radioCapability = radioCap

	cc := &captureConn{}
	msg := &s1ap.InitialUEMessage{
		ENBUES1APID: 9,
		NASPDU:      s1ap.NASPDU(serviceRequestNAS(t, ue)),
		STMSI:       &s1ap.STMSI{MMEC: 1, MTMSI: guti.MTMSI},
	}

	m.onServiceRequest(context.Background(), cc, msg)

	if ue.ecmState.load() != ECMConnected {
		t.Fatal("UE not ECM-CONNECTED after Service Request")
	}

	if ue.ENBUES1APID != 9 {
		t.Fatalf("UE not bound to the new eNB UE id, got %d", ue.ENBUES1APID)
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
			testPDN(ue).sgwFTEID = models.FTEID{TEID: 0x1234, Addr: tc.sgwV4}
			testPDN(ue).sgwN3IPv6 = tc.sgwV6

			cc := &captureConn{}
			m.onServiceRequest(context.Background(), cc, &s1ap.InitialUEMessage{
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
// ECM-IDLE is rebound to a fresh MME-UE-S1AP-ID (TS 36.413 §3.1): the released
// identity is dropped from the active-connection index and the UE is reindexed
// under the new one. Reusing the old identity is what the eNB rejects with
// unknown-mme-ue-s1ap-id.
func TestServiceRequestAllocatesFreshMMEUES1APID(t *testing.T) {
	m := newTestMME(t)
	ue, guti := idleRegisteredUE(t, m)

	oldID := ue.MMEUES1APID

	cc := &captureConn{}
	msg := &s1ap.InitialUEMessage{
		ENBUES1APID: 42,
		NASPDU:      s1ap.NASPDU(serviceRequestNAS(t, ue)),
		STMSI:       &s1ap.STMSI{MMEC: 1, MTMSI: guti.MTMSI},
	}

	m.onServiceRequest(context.Background(), cc, msg)

	if ue.MMEUES1APID == oldID {
		t.Fatalf("MME-UE-S1AP-ID was reused (%d); a returning UE must get a fresh one", oldID)
	}

	if _, ok := m.lookupUe(oldID); ok {
		t.Fatal("released MME-UE-S1AP-ID still indexed after re-establishment")
	}

	if got, ok := m.lookupUe(ue.MMEUES1APID); !ok || got != ue {
		t.Fatal("UE not indexed under its fresh MME-UE-S1AP-ID")
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

	m.onServiceRequest(context.Background(), cc, msg)

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

	m.onServiceRequest(context.Background(), cc, msg)

	if ue.ecmState.load() == ECMConnected {
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

	victimConn := ue.conn
	victimID := ue.MMEUES1APID

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

	m.handleInitialUEMessage(context.Background(), nil, initiatingValue(t, b))

	if ue.conn != victimConn || ue.MMEUES1APID != victimID {
		t.Fatalf("victim binding changed on a forged resume: id %d -> %d", victimID, ue.MMEUES1APID)
	}
}

// A Service Request with an invalid short MAC, on a different association, must
// not move the resolved UE's S1 binding (TS 24.301 §5.6.1).
func TestServiceRequestBadMACDoesNotRebindVictim(t *testing.T) {
	m := newTestMME(t)
	ue, guti := idleRegisteredUE(t, m)

	victimConn := ue.conn
	victimID := ue.MMEUES1APID

	nas := serviceRequestNAS(t, ue)
	nas[3] ^= 0xff

	attacker := &captureConn{}
	m.onServiceRequest(context.Background(), attacker, &s1ap.InitialUEMessage{
		ENBUES1APID: 9,
		NASPDU:      s1ap.NASPDU(nas),
		STMSI:       &s1ap.STMSI{MMEC: 1, MTMSI: guti.MTMSI},
	})

	if ue.conn != victimConn || ue.MMEUES1APID != victimID {
		t.Fatalf("UE binding changed: conn moved=%v, id %d -> %d", ue.conn == nasWriter(attacker), victimID, ue.MMEUES1APID)
	}
}
