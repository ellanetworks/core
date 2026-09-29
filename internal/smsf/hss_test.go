// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6c"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/diameternode"
	"github.com/ellanetworks/core/internal/smsf"
	"go.uber.org/zap"
)

var hssIdentity = diameter.Identity{OriginHost: localIdent.Host, OriginRealm: localIdent.Realm}

type idleNode struct{}

func (idleNode) Node() *diameter.Node             { return nil }
func (idleNode) Peers() []diameternode.PeerStatus { return nil }

func newHSS(t *testing.T) (*smsf.SMSF, *fakeStore) {
	t.Helper()

	store := newFakeStore(netip.MustParseAddrPort("192.0.2.10:3868"))
	s := smsf.New(store, fakeDirectory{localNode: localIdent, remoteNode: remoteIdent}, idleNode{}, newFakeUE(), zap.NewNop(), fastTimers())

	return s, store
}

func routingRequest(t *testing.T, r s6c.RoutingRequest) *diameter.Message {
	t.Helper()

	r.ServiceCentreAddress = serviceCentre

	req, err := s6c.NewSendRoutingInfoForSMRequest(tgpp.Envelope{
		SessionID: "smsc.example.org;1;1", Origin: diameter.Identity{OriginHost: smscHost, OriginRealm: smscRealm},
		DestinationRealm: localIdent.Realm,
	}, r)
	if err != nil {
		t.Fatalf("build SRR: %v", err)
	}

	return req
}

func route(t *testing.T, s *smsf.SMSF, r s6c.RoutingRequest) (s6c.Routing, error) {
	t.Helper()

	return s6c.ParseSendRoutingInfoForSMAnswer(s.SendRoutingInfoForSM(context.Background(), hssIdentity, routingRequest(t, r)))
}

func routeError(t *testing.T, s *smsf.SMSF, r s6c.RoutingRequest) *s6c.ResultError {
	t.Helper()

	_, err := route(t, s, r)

	var re *s6c.ResultError
	if !errors.As(err, &re) {
		t.Fatalf("SRA error = %v, want a result error", err)
	}

	return re
}

func TestRoutingToTheMMEServingTheUE(t *testing.T) {
	s, store := newHSS(t)
	store.register(db.UERegistrationTypeMME, remoteNode, false)

	routing, err := route(t, s, s6c.RoutingRequest{MSISDN: msisdn, SMSFSupport: true})
	if err != nil {
		t.Fatalf("SRA: %v", err)
	}

	mme := routing.Serving
	if routing.IMSI != imsi || mme == nil || mme.MME == nil ||
		mme.MME.Name != remoteIdent.Host || mme.MME.Realm != remoteIdent.Realm || mme.MME.Number != smsNumber || routing.SMSF3GPP != nil {
		t.Fatalf("routing = %+v, want the MME of %s", routing, remoteNode)
	}
}

func TestRoutingToTheSMSFServingTheUE(t *testing.T) {
	s, store := newHSS(t)
	store.register(db.UERegistrationTypeAMF3GPPAccess, localNode, false)

	routing, err := route(t, s, s6c.RoutingRequest{MSISDN: msisdn, SMSFSupport: true})
	if err != nil {
		t.Fatalf("SRA: %v", err)
	}

	smsfAddr := routing.SMSF3GPP
	if routing.Serving != nil || smsfAddr == nil || smsfAddr.Name != localIdent.Host || smsfAddr.Realm != localIdent.Realm || smsfAddr.Number != smsNumber {
		t.Fatalf("routing = %+v, want the SMSF of %s", routing, localNode)
	}
}

func TestRoutingToTheSMSFForAnSMSCWithoutSMSFSupport(t *testing.T) {
	s, store := newHSS(t)
	store.register(db.UERegistrationTypeAMF3GPPAccess, localNode, false)

	routing, err := route(t, s, s6c.RoutingRequest{MSISDN: msisdn})
	if err != nil {
		t.Fatalf("SRA: %v", err)
	}

	if routing.SMSF3GPP != nil || routing.Serving == nil || routing.Serving.MME == nil || routing.Serving.MME.Name != localIdent.Host {
		t.Fatalf("routing = %+v, want the SMSF in the Serving-Node MME AVPs", routing)
	}
}

func TestRoutingByIMSI(t *testing.T) {
	s, store := newHSS(t)
	store.register(db.UERegistrationTypeMME, localNode, false)

	routing, err := route(t, s, s6c.RoutingRequest{IMSI: imsi})
	if err != nil || routing.IMSI != imsi {
		t.Fatalf("routing = %+v %v", routing, err)
	}
}

func TestRoutingFailures(t *testing.T) {
	t.Run("unknown MSISDN", func(t *testing.T) {
		s, _ := newHSS(t)

		if re := routeError(t, s, s6c.RoutingRequest{MSISDN: "15559999999"}); !re.IsExperimental(tgpp.ResultErrorUserUnknown) {
			t.Fatalf("SRA = %s, want user unknown", re)
		}
	})

	t.Run("subscriber without an MSISDN", func(t *testing.T) {
		s, store := newHSS(t)
		store.setMSISDN("")
		store.register(db.UERegistrationTypeMME, localNode, false)

		if re := routeError(t, s, s6c.RoutingRequest{IMSI: imsi}); !re.IsExperimental(tgpp.ResultErrorServiceNotSubscribed) {
			t.Fatalf("SRA = %s, want service not subscribed", re)
		}
	})

	t.Run("not registered", func(t *testing.T) {
		s, _ := newHSS(t)

		re := routeError(t, s, s6c.RoutingRequest{MSISDN: msisdn})
		if !re.IsExperimental(tgpp.ResultErrorAbsentUser) || re.Absent.MME != nil {
			t.Fatalf("SRA = %+v, want absent user without a diagnostic", re)
		}
	})

	t.Run("purged from a node that left the cluster", func(t *testing.T) {
		s, store := newHSS(t)
		store.register(db.UERegistrationTypeMME, "node-gone", true)

		re := routeError(t, s, s6c.RoutingRequest{MSISDN: msisdn})
		if !re.IsExperimental(tgpp.ResultErrorAbsentUser) || re.Absent.MME == nil || *re.Absent.MME != tgpp.AbsentUserPurgedNonGPRS {
			t.Fatalf("SRA = %+v, want absent user (MS purged)", re)
		}
	})

	t.Run("purged from the AMF of a node that left, without SMSF support", func(t *testing.T) {
		s, store := newHSS(t)
		store.register(db.UERegistrationTypeAMF3GPPAccess, "node-gone", true)

		re := routeError(t, s, s6c.RoutingRequest{MSISDN: msisdn})
		if re.Absent.MME == nil || *re.Absent.MME != tgpp.AbsentUserPurgedNonGPRS {
			t.Fatalf("SRA = %+v, want the diagnostic in MME-Absent-User-Diagnostic-SM", re)
		}
	})

	t.Run("purged from the AMF of a node that left, with SMSF support", func(t *testing.T) {
		s, store := newHSS(t)
		store.register(db.UERegistrationTypeAMF3GPPAccess, "node-gone", true)

		re := routeError(t, s, s6c.RoutingRequest{MSISDN: msisdn, SMSFSupport: true})
		if re.Absent.SMSF3GPP == nil || *re.Absent.SMSF3GPP != tgpp.AbsentUserPurgedNonGPRS || re.Absent.MME != nil {
			t.Fatalf("SRA = %+v, want the diagnostic in SMSF-3GPP-Absent-User-Diagnostic-SM", re)
		}
	})

	t.Run("serving node left the cluster", func(t *testing.T) {
		s, store := newHSS(t)
		store.register(db.UERegistrationTypeMME, "node-gone", false)

		if re := routeError(t, s, s6c.RoutingRequest{MSISDN: msisdn}); !re.IsExperimental(tgpp.ResultErrorAbsentUser) {
			t.Fatalf("SRA = %s, want absent user", re)
		}
	})
}

func report(t *testing.T, s *smsf.SMSF, rep s6c.DeliveryReport) (s6c.ReportResult, error) {
	t.Helper()

	rep.ServiceCentreAddress = serviceCentre
	rep.SMSFSupport = true

	req, err := s6c.NewReportSMDeliveryStatusRequest(tgpp.Envelope{
		SessionID: "smsc.example.org;1;2", Origin: diameter.Identity{OriginHost: smscHost, OriginRealm: smscRealm},
		DestinationRealm: localIdent.Realm,
	}, rep)
	if err != nil {
		t.Fatalf("build RDR: %v", err)
	}

	return s6c.ParseReportSMDeliveryStatusAnswer(s.ReportSMDeliveryStatus(context.Background(), hssIdentity, req))
}

func TestATransientLookupErrorIsNotAnsweredAsAbsent(t *testing.T) {
	cases := map[string]func(*fakeStore){
		"registration unreadable": func(store *fakeStore) {
			store.mu.Lock()
			store.regErr = errors.New("leader changed")
			store.mu.Unlock()
		},
		"serving node unreadable": func(store *fakeStore) {
			store.register(db.UERegistrationTypeMME, unreadableNode, false)
		},
	}

	for name, fail := range cases {
		t.Run(name, func(t *testing.T) {
			s, store := newHSS(t)
			fail(store)

			ans := s.SendRoutingInfoForSM(context.Background(), hssIdentity, routingRequest(t, s6c.RoutingRequest{MSISDN: msisdn}))

			result, err := tgpp.ParseResult(ans)
			if err != nil || result.Code != diameter.ResultUnableToComply {
				t.Fatalf("SRA result = %v %v, want unable to comply", result, err)
			}

			if s.Waiting(imsi) {
				t.Fatal("a transient lookup error recorded waiting data")
			}
		})
	}
}

func TestAnAbsentRoutingAnswerRecordsWaitingData(t *testing.T) {
	s, _ := newHSS(t)

	if re := routeError(t, s, s6c.RoutingRequest{MSISDN: msisdn, SingleAttempt: true}); !re.IsExperimental(tgpp.ResultErrorAbsentUser) {
		t.Fatalf("SRA = %s, want absent user", re)
	}

	if s.Waiting(imsi) {
		t.Fatal("a single-attempt routing request left waiting data")
	}

	if re := routeError(t, s, s6c.RoutingRequest{MSISDN: msisdn}); !re.IsExperimental(tgpp.ResultErrorAbsentUser) {
		t.Fatalf("SRA = %s, want absent user", re)
	}

	if !s.Waiting(imsi) {
		t.Fatal("an absent routing answer left no waiting data")
	}
}

func TestDeliveryReportIsAcknowledged(t *testing.T) {
	s, store := newHSS(t)
	store.register(db.UERegistrationTypeMME, localNode, false)

	absent := tgpp.AbsentUserNoPagingResponseMSC

	res, err := report(t, s, s6c.DeliveryReport{
		MSISDN: msisdn,
		MME:    &s6c.DeliveryOutcome{Cause: s6c.DeliveryCauseAbsentUser, AbsentDiagnostic: &absent},
		Failed: s6c.ServingNodes{Serving: &s6c.ServingNode{MME: &s6c.NodeAddress{Name: localIdent.Host, Realm: localIdent.Realm, Number: smsNumber}}},
	})
	if err != nil || res.Serving != nil || res.SMSF3GPP != nil {
		t.Fatalf("RDA = %+v %v, want a bare success", res, err)
	}
}

func TestDeliveryReportRecordsWaitingData(t *testing.T) {
	s, store := newHSS(t)
	store.register(db.UERegistrationTypeMME, localNode, false)

	absent := tgpp.AbsentUserNoPagingResponseMSC

	if _, err := report(t, s, s6c.DeliveryReport{
		MSISDN: msisdn,
		MME:    &s6c.DeliveryOutcome{Cause: s6c.DeliveryCauseAbsentUser, AbsentDiagnostic: &absent},
	}); err != nil {
		t.Fatalf("RDR: %v", err)
	}

	if !s.Waiting(imsi) {
		t.Fatal("an absent-user report left no waiting data")
	}

	if _, err := report(t, s, s6c.DeliveryReport{MSISDN: msisdn, SingleAttempt: true, MME: &s6c.DeliveryOutcome{Cause: s6c.DeliveryCauseSuccessfulTransfer}}); err != nil {
		t.Fatalf("RDR: %v", err)
	}

	if !s.Waiting(imsi) {
		t.Fatal("a single-attempt report changed the waiting data")
	}

	if _, err := report(t, s, s6c.DeliveryReport{MSISDN: msisdn, MME: &s6c.DeliveryOutcome{Cause: s6c.DeliveryCauseSuccessfulTransfer}}); err != nil {
		t.Fatalf("RDR: %v", err)
	}

	if s.Waiting(imsi) {
		t.Fatal("a successful transfer report left the waiting data")
	}
}

func TestDeliveryReportFromAStaleNodeReturnsTheCurrentOne(t *testing.T) {
	s, store := newHSS(t)
	store.register(db.UERegistrationTypeMME, remoteNode, false)

	absent := tgpp.AbsentUserNoPagingResponseMSC

	res, err := report(t, s, s6c.DeliveryReport{
		MSISDN: msisdn,
		MME:    &s6c.DeliveryOutcome{Cause: s6c.DeliveryCauseAbsentUser, AbsentDiagnostic: &absent},
		Failed: s6c.ServingNodes{Serving: &s6c.ServingNode{MME: &s6c.NodeAddress{Name: localIdent.Host, Realm: localIdent.Realm, Number: smsNumber}}},
	})
	if err != nil || res.Serving == nil || res.Serving.MME == nil || res.Serving.MME.Name != remoteIdent.Host {
		t.Fatalf("RDA = %+v %v, want the current serving MME", res, err)
	}
}

func TestDeliveryReportForAnUnknownUser(t *testing.T) {
	s, _ := newHSS(t)

	_, err := report(t, s, s6c.DeliveryReport{MSISDN: "15559999999", MME: &s6c.DeliveryOutcome{Cause: s6c.DeliveryCauseSuccessfulTransfer}})

	var re *s6c.ResultError
	if !errors.As(err, &re) || !re.IsExperimental(tgpp.ResultErrorUserUnknown) {
		t.Fatalf("RDA error = %v, want user unknown", err)
	}
}

func TestSMSAllowed(t *testing.T) {
	s, store := newHSS(t)

	check := func(name, subscriber string, wantAllowed, wantErr bool) {
		t.Helper()

		allowed, err := s.Allowed(context.Background(), subscriber)
		if allowed != wantAllowed || (err != nil) != wantErr {
			t.Fatalf("%s: allowed = %t, err = %v", name, allowed, err)
		}
	}

	check("subscriber with an MSISDN", imsi, true, false)
	check("unknown subscriber", "001010000009999", false, false)

	store.setMSISDN("")
	check("subscriber without an MSISDN", imsi, false, false)

	store.setMSISDN(msisdn)
	store.mu.Lock()
	store.settings.SMSCAddress = ""
	store.mu.Unlock()
	check("no SMSC", imsi, false, false)

	store.mu.Lock()
	store.settingsErr = errors.New("leader changed")
	store.mu.Unlock()
	check("settings unavailable", imsi, false, true)
}

func TestRoutingWithDeliveryNotIntendedReturnsTheIMSI(t *testing.T) {
	s, _ := newHSS(t)

	notIntended := s6c.SMDeliveryNotIntendedIMSI

	ans := s.SendRoutingInfoForSM(context.Background(), hssIdentity, routingRequest(t, s6c.RoutingRequest{MSISDN: msisdn, DeliveryNotIntended: &notIntended}))

	result, err := tgpp.ParseResult(ans)
	if err != nil || !result.Success() {
		t.Fatalf("SRA result = %v %v, want success for an IMSI-only query", result, err)
	}

	if name, ok := ans.Find(diameter.AVPUserName, 0); !ok || name.UTF8String() != imsi {
		t.Fatal("SRA without the IMSI")
	}
}

func TestRoutingPicksTheMostRecentRegistration(t *testing.T) {
	s, store := newHSS(t)
	store.registerVersion(db.UERegistrationTypeMME, localNode, 7)
	store.registerVersion(db.UERegistrationTypeAMF3GPPAccess, remoteNode, 9)

	routing, err := route(t, s, s6c.RoutingRequest{MSISDN: msisdn})
	if err != nil {
		t.Fatalf("SRA: %v", err)
	}

	if routing.Serving == nil || routing.Serving.MME == nil || routing.Serving.MME.Name != remoteIdent.Host {
		t.Fatalf("routing = %+v, want only the most recent registration (%s)", routing, remoteNode)
	}
}

func TestRoutingToTheNodeThatLastServedADetachedUE(t *testing.T) {
	s, store := newHSS(t)
	store.register(db.UERegistrationTypeMME, remoteNode, true)

	routing, err := route(t, s, s6c.RoutingRequest{MSISDN: msisdn})
	if err != nil {
		t.Fatalf("SRA: %v", err)
	}

	if routing.Serving == nil || routing.Serving.MME == nil || routing.Serving.MME.Name != remoteIdent.Host {
		t.Fatalf("routing = %+v, want the node that last served the UE, which holds its not-reachable flag", routing)
	}
}
