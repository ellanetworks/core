// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/s1ap"
)

type n26Policy bool

func (p n26Policy) Allowed(context.Context, string) (bool, error) { return bool(p), nil }

type arrivalPartners struct{}

func (arrivalPartners) ListRoamingPartners(context.Context) ([]db.RoamingPartner, error) {
	sepp, peer := "sepp.5gc.mnc070.mcc999.3gppnetwork.org", "dea"

	return []db.RoamingPartner{{
		ID: "partner", Name: "partner", ProfileID: "roamers", SEPPFQDN: &sepp, DiameterPeerID: &peer,
		PLMNs: []db.RoamingPLMN{{Mcc: "999", Mnc: "70"}},
	}}, nil
}

func (arrivalPartners) CountSubscribersAndVisitors(context.Context) (int, error) { return 1, nil }

func arrivingVisitor(t *testing.T, allowed bool) (*amf.AMF, *amf.Radio, *arrivalSender, interworking.FiveGSRelocationRequest) {
	t.Helper()

	amfInstance, radio, sender, _ := arrivingAMF(t, &epsPeerStub{})
	amfInstance.DBInstance = &fakeDBInstance{
		Operator: arrivingOperator(),
		Visitor:  true,
		Slices:   []db.NetworkSlice{{ID: "slice-1", Name: "default", Sst: 1}},
	}
	amfInstance.Roaming = arrivalPartners{}
	amfInstance.N26 = n26Policy(allowed)

	supi, err := etsi.NewSUPIFromIMSI("999700000000001")
	if err != nil {
		t.Fatal(err)
	}

	req := arrivingRequest()
	req.SUPI = supi

	return amfInstance, radio, sender, req
}

func TestForwardRelocationAdmitsAVisitorOfAPartnerWithBothAgreements(t *testing.T) {
	amfInstance, radio, sender, req := arrivingVisitor(t, true)

	done := forwardArrival(t, amfInstance, req)
	hoReq := sender.awaitHandoverRequest(t)

	if len(hoReq.AllowedNSSAI) != 1 || hoReq.AllowedNSSAI[0].SNSSAI.SST != 1 {
		t.Errorf("Allowed NSSAI = %+v, want the partner profile's slice", hoReq.AllowedNSSAI)
	}

	admitArrival(t, amfInstance, radio, hoReq, arrivingPDUSessionID)

	if _, err := awaitArrival(t, done); err != nil {
		t.Fatalf("ForwardRelocation: %v", err)
	}

	HandleHandoverNotify(context.Background(), amfInstance, radio, &ngap.HandoverNotify{
		AMFUENGAPID: hoReq.AMFUENGAPID,
		RANUENGAPID: arrivingRANUEID,
	})

	ue, ok := amfInstance.LookupUeBySupi(req.SUPI)
	if !ok {
		t.Fatal("the arrived visitor was never indexed by its SUPI")
	}

	v, ok := ue.Visitor()
	if !ok || v.PartnerID != "partner" || v.Registered {
		t.Fatalf("visitor = %+v, %v, want an admitted visitor that registers at its home UDM with its mobility registration", v, ok)
	}

	if !ue.EPSInterworkingAllowed() {
		t.Error("the arrived visitor may not move back to EPS")
	}

	if ebi, ok := ue.EPSBearerIdentity(arrivingPDUSessionID); !ok || ebi != arrivingEPSBearerID {
		t.Errorf("EPS bearer identity = %d/%t, want %d kept for a move back", ebi, ok, arrivingEPSBearerID)
	}
}

func TestForwardRelocationRefusesAVisitorWithoutN26(t *testing.T) {
	amfInstance, _, sender, req := arrivingVisitor(t, false)

	_, err := amfInstance.ForwardRelocation(context.Background(), req)
	if !errors.Is(err, interworking.ErrN26NotAllowed) {
		t.Fatalf("error = %v, want ErrN26NotAllowed", err)
	}

	var refusal interworking.TargetRefusal
	if !errors.As(err, &refusal) || refusal.Cause.Value != s1ap.CauseRadioNetworkHOTargetNotAllowed {
		t.Fatalf("error = %v, want a handover-target-not-allowed refusal", err)
	}

	if len(sender.SentHandoverRequests) != 0 {
		t.Error("the target gNB was asked to admit the UE")
	}
}
