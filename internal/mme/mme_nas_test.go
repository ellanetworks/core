// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

// TestInitialContextSetupResponseRelaysENBFTEID checks the MME extracts the
// eNB S1-U F-TEID from the Initial Context Setup Response and hands it to the
// anchor via ModifyEPSSession, completing the downlink path.
func TestInitialContextSetupResponseRelaysENBFTEID(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.newUe(cc, 7)
	ue.imsi = testSubscriber.IMSI
	testPDN(ue).apn = "internet"

	resp := &s1ap.InitialContextSetupResponse{
		MMEUES1APID: ue.MMEUES1APID,
		ENBUES1APID: 7,
		ERABSetup: []s1ap.ERABSetupItemCtxtSURes{{
			ERABID:                s1ap.ERABID(defaultERABID),
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

	m.handleInitialContextSetupResponse(context.Background(), cc, pdu.(*s1ap.SuccessfulOutcome).Value)

	want := models.FTEID{TEID: 0x55, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 3})}

	if testPDN(ue).enbFTEID != want {
		t.Fatalf("testPDN(ue).enbFTEID = %+v, want %+v", testPDN(ue).enbFTEID, want)
	}

	fsm, ok := m.session.(*fakeSessionManager)
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
			ue := m.newUe(cc, 7)
			ue.imsi = testSubscriber.IMSI
			testPDN(ue).apn = "internet"

			resp := &s1ap.InitialContextSetupResponse{
				MMEUES1APID: ue.MMEUES1APID,
				ENBUES1APID: 7,
				ERABSetup: []s1ap.ERABSetupItemCtxtSURes{{
					ERABID:                s1ap.ERABID(defaultERABID),
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

			m.handleInitialContextSetupResponse(context.Background(), cc, pdu.(*s1ap.SuccessfulOutcome).Value)

			if testPDN(ue).enbFTEID.Addr != tc.want {
				t.Fatalf("eNB F-TEID address = %v, want %v", testPDN(ue).enbFTEID.Addr, tc.want)
			}
		})
	}
}
