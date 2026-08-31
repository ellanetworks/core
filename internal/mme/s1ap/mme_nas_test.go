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

func TestInitialContextSetupResponseRelaysENBFTEID(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)
	ue.SetIMSIForTest(testSubscriber.IMSI)
	testPDN(ue).Apn = "internet"

	resp := &s1ap.InitialContextSetupResponse{
		MMEUES1APID: s1ap.Ptr(ue.Conn().MMEUES1APID),
		ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(7)),
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

	handleInitialContextSetupResponse(m, context.Background(), mme.NewRadioForTest(cc), pdu.(*s1ap.SuccessfulOutcome).Value)

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
				MMEUES1APID: s1ap.Ptr(ue.Conn().MMEUES1APID),
				ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(7)),
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

			handleInitialContextSetupResponse(m, context.Background(), mme.NewRadioForTest(cc), pdu.(*s1ap.SuccessfulOutcome).Value)

			if testPDN(ue).EnbFTEID.Addr != tc.want {
				t.Fatalf("eNB F-TEID address = %v, want %v", testPDN(ue).EnbFTEID.Addr, tc.want)
			}
		})
	}
}

func TestInitialContextSetupResponseMultipleERABs(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)
	ue.SetIMSIForTest(testSubscriber.IMSI)

	p1 := testPDN(ue)
	p1.Apn = "internet"
	p2 := ue.EnsurePDN(6)
	p2.Apn = "ims"

	resp := &s1ap.InitialContextSetupResponse{
		MMEUES1APID: s1ap.Ptr(ue.Conn().MMEUES1APID),
		ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(7)),
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

	handleInitialContextSetupResponse(m, context.Background(), mme.NewRadioForTest(cc), pdu.(*s1ap.SuccessfulOutcome).Value)

	want1 := models.FTEID{TEID: 0x55, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 3})}
	want2 := models.FTEID{TEID: 0x66, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 4})}

	if p1.EnbFTEID != want1 {
		t.Fatalf("default bearer eNB F-TEID = %+v, want %+v", p1.EnbFTEID, want1)
	}

	if p2.EnbFTEID != want2 {
		t.Fatalf("second bearer eNB F-TEID = %+v, want %+v", p2.EnbFTEID, want2)
	}
}

// TS 36.413 §8.3.1.2
func TestInitialContextSetupResponseReleasesFailedERAB(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)
	ue.SetIMSIForTest(testSubscriber.IMSI)

	p1 := testPDN(ue)
	p1.Apn = "internet"
	p2 := ue.EnsurePDN(6)
	p2.Apn = "ims"

	resp := &s1ap.InitialContextSetupResponse{
		MMEUES1APID: s1ap.Ptr(ue.Conn().MMEUES1APID),
		ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(7)),
		ERABSetup: []s1ap.ERABSetupItemCtxtSURes{{
			ERABID:                s1ap.ERABID(mme.DefaultERABID),
			TransportLayerAddress: s1ap.TransportLayerAddress([]byte{10, 3, 0, 3}),
			GTPTEID:               s1ap.GTPTEID(0x55),
		}},
		ERABFailedToSetup: []s1ap.ERABItem{{
			ERABID: s1ap.ERABID(6),
			Cause:  s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0},
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

	handleInitialContextSetupResponse(m, context.Background(), mme.NewRadioForTest(cc), pdu.(*s1ap.SuccessfulOutcome).Value)

	if m.LookupPDN(ue, 6) != nil {
		t.Fatal("failed E-RAB's PDN connection must be released")
	}

	fsm, ok := m.Session.(*fakeSessionManager)
	if !ok {
		t.Fatal("session manager is not the fake")
	}

	if !fsm.released {
		t.Fatal("failed E-RAB's anchor session must be released")
	}

	if m.LookupPDN(ue, mme.DefaultERABID) == nil {
		t.Fatal("successfully set up bearer must be retained")
	}

	if p1.EnbFTEID.TEID != 0x55 {
		t.Fatalf("default bearer eNB F-TEID TEID = %#x, want 0x55", p1.EnbFTEID.TEID)
	}
}

// TS 36.413 §8.3.1.3
func TestInitialContextSetupFailureAbortsUE(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)
	ue.SetIMSIForTest(testSubscriber.IMSI)
	testPDN(ue).Apn = "internet"

	mmeID := ue.Conn().MMEUES1APID

	fail := &s1ap.InitialContextSetupFailure{
		MMEUES1APID: s1ap.Ptr(mmeID),
		ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(7)),
		Cause:       s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupTransport, Value: s1ap.CauseTransportResourceUnavailable}),
	}

	b, err := fail.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	handleInitialContextSetupFailure(m, context.Background(), mme.NewRadioForTest(cc), pdu.(*s1ap.UnsuccessfulOutcome).Value)

	if cc.count() != 0 {
		t.Fatalf("expected no S1AP command on Initial Context Setup Failure (local release), got %d", cc.count())
	}

	if _, ok := m.LookupUe(mmeID); ok {
		t.Fatal("the incomplete UE was not aborted on Initial Context Setup Failure")
	}
}
