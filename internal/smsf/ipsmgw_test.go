// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6c"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
)

const (
	ipSMGWHost  = "ipsmgw.ims.example.org"
	ipSMGWRealm = "ims.example.org"
)

func registerIPSMGW(store *fakeStore) {
	store.mu.Lock()
	store.ipSMGW = &db.IPSMGWRegistration{IMSI: imsi, Number: "15550000100", DiameterHost: ipSMGWHost, DiameterRealm: ipSMGWRealm}
	store.mu.Unlock()
}

func TestRoutingRequestIsForwardedToTheIPSMGW(t *testing.T) {
	type relayed struct{ from, host, realm string }

	var got []relayed

	s, store := newHSSWith(t, idleSMSC{relay: func(req *diameter.Message, from, host, realm string) (*diameter.Message, error) {
		got = append(got, relayed{from, host, realm})

		return s6c.NewSendRoutingInfoForSMAnswer(req, diameter.Identity{OriginHost: ipSMGWHost, OriginRealm: ipSMGWRealm},
			s6c.Routing{IMSI: imsi, ServingNodes: s6c.ServingNodes{Serving: &s6c.ServingNode{IPSMGW: &s6c.NodeAddress{Name: ipSMGWHost, Realm: ipSMGWRealm, Number: "15550000100"}}}}, false)
	}})
	store.register(db.UERegistrationTypeMME, remoteNode, false)
	registerIPSMGW(store)

	routing, err := route(t, s, s6c.RoutingRequest{MSISDN: msisdn})
	if err != nil || routing.Serving == nil || routing.Serving.IPSMGW == nil || routing.Serving.IPSMGW.Name != ipSMGWHost {
		t.Fatalf("SRA = %+v, %v, want the IP-SM-GW's", routing, err)
	}

	if len(got) != 1 || got[0] != (relayed{smscHost, ipSMGWHost, ipSMGWRealm}) {
		t.Fatalf("relayed = %+v", got)
	}
}

func TestRoutingRequestOfTheIPSMGWIsAnswered(t *testing.T) {
	s, store := newHSSWith(t, idleSMSC{relay: func(*diameter.Message, string, string, string) (*diameter.Message, error) {
		t.Fatal("relayed the IP-SM-GW's own routing request")
		return nil, nil
	}})
	store.register(db.UERegistrationTypeMME, remoteNode, false)
	registerIPSMGW(store)

	req, err := s6c.NewSendRoutingInfoForSMRequest(tgpp.Envelope{
		SessionID: ipSMGWHost + ";1;1", Origin: diameter.Identity{OriginHost: ipSMGWHost, OriginRealm: ipSMGWRealm},
		DestinationRealm: localIdent.Realm,
	}, s6c.RoutingRequest{MSISDN: msisdn, ServiceCentreAddress: serviceCentre})
	if err != nil {
		t.Fatal(err)
	}

	routing, err := s6c.ParseSendRoutingInfoForSMAnswer(s.SendRoutingInfoForSM(context.Background(), localIdentity, ipSMGWHost, req))
	if err != nil || routing.Serving == nil || routing.Serving.MME == nil || routing.Serving.MME.Name != remoteIdent.Host {
		t.Fatalf("SRA = %+v, %v, want the MME", routing, err)
	}
}

func TestRoutingRequestToAnUnreachableIPSMGW(t *testing.T) {
	cases := map[string]struct {
		err  error
		want uint32
	}{
		"unable to deliver": {diameter.ErrUnableToDeliver, diameter.ResultUnableToDeliver},
		"loop":              {diameter.ErrLoopDetected, diameter.ResultLoopDetected},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s, store := newHSSWith(t, idleSMSC{relay: func(*diameter.Message, string, string, string) (*diameter.Message, error) {
				return nil, tc.err
			}})
			store.register(db.UERegistrationTypeMME, remoteNode, false)
			registerIPSMGW(store)

			ans := s.SendRoutingInfoForSM(context.Background(), localIdentity, smscHost, routingRequest(t, s6c.RoutingRequest{MSISDN: msisdn}))

			r, err := tgpp.ParseResult(ans)
			if err != nil || r.Code != tc.want || r.Experimental || ans.Flags&diameter.FlagError == 0 {
				t.Fatalf("SRA = %s, %v, want %d with the E bit", r, err, tc.want)
			}
		})
	}
}

func TestDeliveryReportOfTheSMSGMSCIsIgnoredWhenAnIPSMGWIsRegistered(t *testing.T) {
	s, store := newHSS(t)
	store.register(db.UERegistrationTypeMME, localNode, false)
	registerIPSMGW(store)

	absent := tgpp.AbsentUserNoPagingResponseMSC

	res, err := report(t, s, s6c.DeliveryReport{
		MSISDN: msisdn,
		IPSMGW: &s6c.DeliveryOutcome{Cause: s6c.DeliveryCauseAbsentUser, AbsentDiagnostic: &absent},
		Failed: s6c.ServingNodes{Serving: &s6c.ServingNode{IPSMGW: &s6c.NodeAddress{Name: ipSMGWHost, Realm: ipSMGWRealm, Number: "15550000100"}}},
	})
	if err != nil {
		t.Fatalf("RDR: %v", err)
	}

	if res.Serving != nil || res.SMSF3GPP != nil {
		t.Fatalf("RDA = %+v, want no serving node to retry with", res)
	}

	if s.Waiting(imsi) {
		t.Fatal("the SMS-GMSC's report was recorded")
	}

	rep := s6c.DeliveryReport{
		MSISDN: msisdn, ServiceCentreAddress: serviceCentre, SMSFSupport: true,
		IPSMGW: &s6c.DeliveryOutcome{Cause: s6c.DeliveryCauseAbsentUser},
	}

	req, err := s6c.NewReportSMDeliveryStatusRequest(tgpp.Envelope{
		SessionID: ipSMGWHost + ";1;2", Origin: diameter.Identity{OriginHost: ipSMGWHost, OriginRealm: ipSMGWRealm},
		DestinationRealm: localIdent.Realm,
	}, rep)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s6c.ParseReportSMDeliveryStatusAnswer(s.ReportSMDeliveryStatus(context.Background(), localIdentity, req)); err != nil {
		t.Fatalf("RDR: %v", err)
	}

	if !s.Waiting(imsi) {
		t.Fatal("the IP-SM-GW's report was not recorded")
	}
}

func TestIPSMGWRegistrationAlertsTheSMSC(t *testing.T) {
	e := newEnv(t)

	if err := e.store.RecordSMSWaiting(context.Background(), db.SMSWaitingUpdate{IMSI: imsi, ServiceCentre: serviceCentre}); err != nil {
		t.Fatal(err)
	}

	e.smsf.IPSMGWRegistered(context.Background(), imsi)

	eventually(t, "the SMSC alert", func() bool { return len(e.smsc.alerted()) == 1 })
}
