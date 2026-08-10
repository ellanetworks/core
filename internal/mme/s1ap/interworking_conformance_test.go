// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

func icsResponseFor(t *testing.T, m *mme.MME, cc *captureConn, ue *mme.UeContext, ebis ...uint8) {
	t.Helper()

	rsp := &s1ap.InitialContextSetupResponse{
		MMEUES1APID: s1ap.Ptr(ue.Conn().MMEUES1APID),
		ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(7)),
	}

	for _, ebi := range ebis {
		rsp.ERABSetup = append(rsp.ERABSetup, s1ap.ERABSetupItemCtxtSURes{
			ERABID:                s1ap.ERABID(ebi),
			TransportLayerAddress: s1ap.TransportLayerAddress{10, 3, 0, 3},
			GTPTEID:               s1ap.GTPTEID(0x90 + uint32(ebi)),
		})
	}

	pdu, err := s1ap.Unmarshal(mustMarshal(t, rsp.Marshal))
	if err != nil {
		t.Fatalf("parse Initial Context Setup Response: %v", err)
	}

	handleInitialContextSetupResponse(m, context.Background(), mme.NewRadioForTest(cc), pdu.(*s1ap.SuccessfulOutcome).Value)
}

// TS 36.413 §8.3.1.2
func TestInterworkingICSResponseReleasesABearerTheAnchorRefused(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)
	ue.SetIMSIForTest(testSubscriber.IMSI)
	testPDN(ue).Apn = "internet"
	secondPDN(ue)

	sessionManager(t, m).failModify(6, errAnchorRefused)

	icsResponseFor(t, m, cc, ue, mme.DefaultERABID, 6)

	if m.LookupPDN(ue, 6) != nil {
		t.Error("a PDN connection whose downlink the anchor refused was left on the UE with no user plane")
	}

	if m.LookupPDN(ue, mme.DefaultERABID) == nil {
		t.Error("the PDN connection the anchor accepted was released too")
	}
}

// TS 23.401 §5.4.4.1
func TestInterworkingICSResponseWithNoBearerLeavesNoRegisteredUE(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)
	ue.SetIMSIForTest(testSubscriber.IMSI)
	testPDN(ue).Apn = "internet"
	ue.TransitionTo(mme.EMMRegistrationInitiated)
	ue.TransitionTo(mme.EMMRegistered)

	if ue.EMMState() != mme.EMMRegistered {
		t.Fatalf("EMM state = %v, want EMM-REGISTERED before the response arrives", ue.EMMState())
	}

	sessionManager(t, m).failModify(mme.DefaultERABID, errAnchorRefused)

	icsResponseFor(t, m, cc, ue, mme.DefaultERABID)

	if ue.PDNCount() != 0 {
		t.Fatalf("PDN connections = %d, want 0: the anchor refused the only bearer", ue.PDNCount())
	}

	if ue.EMMState() != mme.EMMDeregistered {
		t.Error("the UE is EMM-REGISTERED holding no PDN connection: no E-RAB survives its next Service Request, its TAU reports an all-zero bearer bitmap, and nothing else ever detaches it")
	}
}
