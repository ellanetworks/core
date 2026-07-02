// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

// TestInitialContextSetupResponseRelaysENBFTEID checks the MME extracts the
// eNB S1-U F-TEID from the Initial Context Setup Response and hands it to the
// anchor via ModifyEPSSession, completing the downlink path.
func TestInitialContextSetupResponseRelaysENBFTEID(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)
	ue.SetIMSIForTest(testSubscriber.IMSI)
	testPDN(ue).Apn = "internet"

	resp := &s1ap.InitialContextSetupResponse{
		MMEUES1APID: ue.S1.MMEUES1APID,
		ENBUES1APID: 7,
		ERABSetup: []s1ap.ERABSetupItemCtxtSURes{{
			ERABID:                s1ap.ERABID(mme.DefaultERABID),
			TransportLayerAddress: s1ap.TransportLayerAddress([]byte{10, 3, 0, 3}),
			GTPTEID:               s1ap.GTPTEID(0x55),
		}},
	}

	b, err := resp.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	handleInitialContextSetupResponse(m, context.Background(), cc, pdu.(*s1ap.SuccessfulOutcome).Value)

	want := models.FTEID{TEID: 0x55, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 3})}

	if testPDN(ue).EnbFTEID != want {
		t.Fatalf("testPDN(ue).enbFTEID = %+v, want %+v", testPDN(ue).EnbFTEID, want)
	}

	fsm, ok := m.Session.(*fakeSessionManager)
	if !ok {
		t.Fatal("session manager is not the fake")
	}

	if fsm.modifiedENB != want {
		t.Fatalf("ModifyEPSSession eNB F-TEID = %+v, want %+v", fsm.modifiedENB, want)
	}
}

// TestInitialContextSetupResponseENBTransportFamily checks the MME accepts an eNB
// S1-U endpoint reported as IPv4, IPv6, or dual-stack (TS 36.413), preferring the
// IPv6 endpoint when both are offered — matching the 5G N3 handling.
func TestInitialContextSetupResponseENBTransportFamily(t *testing.T) {
	v6 := netip.MustParseAddr("2001:db8:3::3")
	v6Octets := v6.As16()

	tests := []struct {
		name string
		tla  []byte
		want netip.Addr
	}{
		{name: "ipv4", tla: []byte{10, 3, 0, 3}, want: netip.AddrFrom4([4]byte{10, 3, 0, 3})},
		{name: "ipv6", tla: v6Octets[:], want: v6},
		{name: "dualstack prefers ipv6", tla: append([]byte{10, 3, 0, 3}, v6Octets[:]...), want: v6},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMME(t)
			cc := &captureConn{}
			ue := m.NewUe(cc, 7)
			ue.SetIMSIForTest(testSubscriber.IMSI)
			testPDN(ue).Apn = "internet"

			resp := &s1ap.InitialContextSetupResponse{
				MMEUES1APID: ue.S1.MMEUES1APID,
				ENBUES1APID: 7,
				ERABSetup: []s1ap.ERABSetupItemCtxtSURes{{
					ERABID:                s1ap.ERABID(mme.DefaultERABID),
					TransportLayerAddress: s1ap.TransportLayerAddress(tc.tla),
					GTPTEID:               s1ap.GTPTEID(0x55),
				}},
			}

			b, err := resp.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			pdu, err := s1ap.Unmarshal(b)
			if err != nil {
				t.Fatal(err)
			}

			handleInitialContextSetupResponse(m, context.Background(), cc, pdu.(*s1ap.SuccessfulOutcome).Value)

			if testPDN(ue).EnbFTEID.Addr != tc.want {
				t.Fatalf("eNB F-TEID address = %v, want %v", testPDN(ue).EnbFTEID.Addr, tc.want)
			}
		})
	}
}

// TestInitialContextSetupResponseMultipleERABs checks the MME records the eNB S1-U
// F-TEID for every E-RAB in the response, not only the first — a UE re-established
// from ECM-IDLE with multiple PDN connections sets up all its bearers at once.
func TestInitialContextSetupResponseMultipleERABs(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)
	ue.SetIMSIForTest(testSubscriber.IMSI)

	p1 := testPDN(ue) // default bearer, EBI 5
	p1.Apn = "internet"
	p2 := ue.EnsurePDN(6) // second PDN connection, EBI 6
	p2.Apn = "ims"

	resp := &s1ap.InitialContextSetupResponse{
		MMEUES1APID: ue.S1.MMEUES1APID,
		ENBUES1APID: 7,
		ERABSetup: []s1ap.ERABSetupItemCtxtSURes{
			{
				ERABID:                s1ap.ERABID(mme.DefaultERABID),
				TransportLayerAddress: s1ap.TransportLayerAddress([]byte{10, 3, 0, 3}),
				GTPTEID:               s1ap.GTPTEID(0x55),
			},
			{
				ERABID:                s1ap.ERABID(6),
				TransportLayerAddress: s1ap.TransportLayerAddress([]byte{10, 3, 0, 4}),
				GTPTEID:               s1ap.GTPTEID(0x66),
			},
		},
	}

	b, err := resp.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	handleInitialContextSetupResponse(m, context.Background(), cc, pdu.(*s1ap.SuccessfulOutcome).Value)

	want1 := models.FTEID{TEID: 0x55, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 3})}
	want2 := models.FTEID{TEID: 0x66, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 4})}

	if p1.EnbFTEID != want1 {
		t.Fatalf("default bearer eNB F-TEID = %+v, want %+v", p1.EnbFTEID, want1)
	}

	if p2.EnbFTEID != want2 {
		t.Fatalf("second bearer eNB F-TEID = %+v, want %+v", p2.EnbFTEID, want2)
	}
}
