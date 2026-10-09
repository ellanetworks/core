// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pcf_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/rx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/pcf"
	"github.com/ellanetworks/core/internal/smf"
	"go.uber.org/zap"
)

const testRef = "imsi-001010000000001/5#1"

var (
	pcfIdentity = diameter.Identity{OriginHost: "core.epc.mnc001.mcc001.3gppnetwork.org", OriginRealm: "epc.mnc001.mcc001.3gppnetwork.org"}
	afIdentity  = diameter.Identity{OriginHost: "ims.ims.mnc001.mcc001.3gppnetwork.org", OriginRealm: "ims.mnc001.mcc001.3gppnetwork.org"}
	testUEv4    = netip.MustParseAddr("10.60.0.1")
	testUEv6    = netip.MustParseAddr("fd60:0:0:1::1234")
	signalling  = rx.FlowUsageAFSignalling
	audio       = rx.MediaAudio
)

func supi(t *testing.T, imsi string) etsi.SUPI {
	t.Helper()

	s, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		t.Fatal(err)
	}

	return s
}

type policyStore struct{}

func (policyStore) GetSessionPolicy(context.Context, string, int32, string, string) (*db.Policy, error) {
	return &db.Policy{ID: "policy"}, nil
}

func establish(t *testing.T, p *pcf.PCF, ref string, c smf.PolicyContext) {
	t.Helper()

	if _, err := p.CreateAssociation(context.Background(), ref, c); err != nil {
		t.Fatalf("CreateAssociation %s: %v", ref, err)
	}
}

func newPCF(t *testing.T) *pcf.PCF {
	t.Helper()

	p := pcf.New(policyStore{}, zap.NewNop())
	establish(t, p, testRef, smf.PolicyContext{
		Supi:       supi(t, "001010000000001"),
		Dnn:        models.IMSDataNetworkName,
		IPv4:       testUEv4,
		IPv6Prefix: netip.MustParsePrefix("fd60:0:0:1::/64"),
	})

	return p
}

func envelope(sessionID string) tgpp.Envelope {
	return tgpp.Envelope{SessionID: sessionID, Origin: afIdentity, DestinationRealm: pcfIdentity.OriginRealm}
}

func signallingAAR(ue netip.Addr) rx.AARequest {
	r := rx.AARequest{
		MediaComponents: []rx.MediaComponent{{SubComponents: []rx.MediaSubComponent{{FlowUsage: &signalling}}}},
		SpecificActions: []rx.SpecificAction{rx.ActionIndicationOfLossOfBearer, rx.ActionIndicationOfReleaseOfBearer},
		Features:        rx.FeatureRel8,
	}

	if ue.Is4() {
		r.FramedIPAddress = ue
	} else {
		r.FramedIPv6Address = ue
	}

	return r
}

func aa(t *testing.T, p *pcf.PCF, sessionID string, r rx.AARequest) *diameter.Message {
	t.Helper()

	req, err := rx.NewAARequest(envelope(sessionID), r)
	if err != nil {
		t.Fatalf("build AAR: %v", err)
	}

	return p.AA(context.Background(), pcfIdentity, req)
}

func str(t *testing.T, p *pcf.PCF, sessionID string) *diameter.Message {
	t.Helper()

	req, err := rx.NewSessionTerminationRequest(envelope(sessionID), rx.SessionTerminationRequest{Cause: rx.TerminationLogout})
	if err != nil {
		t.Fatalf("build STR: %v", err)
	}

	return p.SessionTermination(context.Background(), pcfIdentity, req)
}

func requireResult(t *testing.T, ans *diameter.Message, want tgpp.Result) {
	t.Helper()

	got, err := tgpp.ParseResult(ans)
	if err != nil {
		t.Fatalf("parse result: %v", err)
	}

	if got.Code != want.Code || got.Experimental != want.Experimental {
		t.Fatalf("result = %s, want %s", got, want)
	}
}

var success = tgpp.Result{Code: diameter.ResultSuccess}

func TestAA(t *testing.T) {
	update := rx.RequestUpdate
	restoration := rx.RequestPCSCFRestoration

	cases := []struct {
		name string
		req  rx.AARequest
		want tgpp.Result
	}{
		{name: "signalling bound by IPv4 address", req: signallingAAR(testUEv4), want: success},
		{name: "signalling bound by IPv6 address", req: signallingAAR(testUEv6), want: success},
		{
			name: "call media bound by IPv4 address",
			req: rx.AARequest{
				FramedIPAddress: testUEv4,
				MediaComponents: []rx.MediaComponent{{Number: 1, Type: &audio, SubComponents: []rx.MediaSubComponent{{FlowNumber: 1}}}},
			},
			want: success,
		},
		{name: "UE without an ims session", req: signallingAAR(netip.MustParseAddr("10.60.0.9")), want: tgpp.Experimental(tgpp.ResultIPCANSessionNotAvailable)},
		{name: "no UE address", req: rx.AARequest{}, want: tgpp.Experimental(tgpp.ResultIPCANSessionNotAvailable)},
		{name: "update of an unknown session", req: rx.AARequest{RequestType: &update}, want: tgpp.Result{Code: diameter.ResultUnknownSessionID}},
		{name: "P-CSCF restoration", req: rx.AARequest{FramedIPAddress: testUEv4, RequestType: &restoration, NoStateMaintained: true}, want: tgpp.Result{Code: diameter.ResultUnableToComply}},
		{name: "required unsupported feature", req: rx.AARequest{FramedIPAddress: testUEv4, Features: rx.FeatureRel8 | rx.FeatureNetLoc, FeaturesRequired: true}, want: tgpp.Experimental(tgpp.ResultErrorFeatureUnsupported)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newPCF(t)

			ans := aa(t, p, "af;1", tc.req)
			requireResult(t, ans, tc.want)

			if !tc.want.Success() {
				return
			}

			a, err := rx.ParseAAAnswer(ans)
			if err != nil {
				t.Fatalf("parse AAA: %v", err)
			}

			if a.Features != tc.req.Features&rx.FeatureRel8 {
				t.Fatalf("AAA features = %v, want %v", a.Features, tc.req.Features&rx.FeatureRel8)
			}

			requireResult(t, str(t, p, "af;1"), success)
		})
	}
}

func TestAAUpdatesAnOpenSession(t *testing.T) {
	p := newPCF(t)
	update := rx.RequestUpdate

	requireResult(t, aa(t, p, "af;1", signallingAAR(testUEv4)), success)
	requireResult(t, aa(t, p, "af;1", rx.AARequest{RequestType: &update}), success)
}

func TestAAOnlyBindsToTheIMSDataNetwork(t *testing.T) {
	p := pcf.New(policyStore{}, zap.NewNop())
	establish(t, p, "internet", smf.PolicyContext{Supi: supi(t, "001010000000001"), Dnn: "internet", IPv4: testUEv4})

	requireResult(t, aa(t, p, "af;1", signallingAAR(testUEv4)), tgpp.Experimental(tgpp.ResultIPCANSessionNotAvailable))
}

func TestSessionTermination(t *testing.T) {
	p := newPCF(t)

	requireResult(t, str(t, p, "af;1"), tgpp.Result{Code: diameter.ResultUnknownSessionID})
	requireResult(t, aa(t, p, "af;1", signallingAAR(testUEv4)), success)
	requireResult(t, str(t, p, "af;1"), success)
	requireResult(t, str(t, p, "af;1"), tgpp.Result{Code: diameter.ResultUnknownSessionID})
}

func TestTerminatedAssociationAbortsItsRxSessions(t *testing.T) {
	p := newPCF(t)
	update := rx.RequestUpdate
	other := netip.MustParseAddr("10.60.0.2")
	establish(t, p, "other", smf.PolicyContext{Supi: supi(t, "001010000000002"), Dnn: models.IMSDataNetworkName, IPv4: other})

	requireResult(t, aa(t, p, "af;1", signallingAAR(testUEv4)), success)
	requireResult(t, aa(t, p, "af;2", signallingAAR(testUEv6)), success)
	requireResult(t, aa(t, p, "af;3", signallingAAR(other)), success)

	p.TerminateAssociation(testRef)

	requireResult(t, aa(t, p, "af;1", rx.AARequest{RequestType: &update}), tgpp.Result{Code: diameter.ResultUnknownSessionID})
	requireResult(t, aa(t, p, "af;2", signallingAAR(testUEv6)), tgpp.Experimental(tgpp.ResultIPCANSessionNotAvailable))
	requireResult(t, aa(t, p, "af;3", rx.AARequest{RequestType: &update}), success)
	requireResult(t, str(t, p, "af;3"), success)

	deadline := time.Now().Add(5 * time.Second)

	for {
		ans := str(t, p, "af;1")
		if r, err := tgpp.ParseResult(ans); err == nil && r.Code == diameter.ResultUnknownSessionID {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("the aborted session is still held after the ASR failed")
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func TestReplacedAssociationAbortsItsRxSessionsAndRebinds(t *testing.T) {
	p := newPCF(t)
	update := rx.RequestUpdate
	moved := netip.MustParseAddr("10.60.0.7")

	requireResult(t, aa(t, p, "af;1", signallingAAR(testUEv4)), success)

	establish(t, p, testRef, smf.PolicyContext{Supi: supi(t, "001010000000001"), Dnn: models.IMSDataNetworkName, IPv4: moved})

	requireResult(t, aa(t, p, "af;1", rx.AARequest{RequestType: &update}), tgpp.Result{Code: diameter.ResultUnknownSessionID})
	requireResult(t, aa(t, p, "af;2", signallingAAR(testUEv4)), tgpp.Experimental(tgpp.ResultIPCANSessionNotAvailable))
	requireResult(t, aa(t, p, "af;3", signallingAAR(moved)), success)
}
