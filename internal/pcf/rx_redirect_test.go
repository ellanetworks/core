// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pcf_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/rx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/pcf"
)

var otherNode = diameter.URI{Host: "mmec02.mmegi8100.mme.epc.mnc001.mcc001.3gppnetwork.org", Port: 3868, Transport: diameter.TransportSCTP}

type fakeDiameter struct{}

func (fakeDiameter) Node() *diameter.Node { return nil }

func (fakeDiameter) Handle(uint32, uint32, diameter.Handler) {}

type fakeOwners struct {
	owners map[netip.Addr]pcf.Owner
	err    error
	asked  []netip.Addr
}

func (f *fakeOwners) Owner(_ context.Context, ue netip.Addr) (pcf.Owner, error) {
	f.asked = append(f.asked, ue)

	if f.err != nil {
		return pcf.Owner{}, f.err
	}

	o, ok := f.owners[ue]
	if !ok {
		return pcf.Owner{}, pcf.ErrNoOwner
	}

	return o, nil
}

func addressedAA(t *testing.T, p *pcf.PCF, sessionID string, r rx.AARequest) *diameter.Message {
	t.Helper()

	env := envelope(sessionID)
	env.DestinationHost = pcfIdentity.OriginHost

	req, err := rx.NewAARequest(env, r)
	if err != nil {
		t.Fatalf("build AAR: %v", err)
	}

	return p.AA(context.Background(), pcfIdentity, req)
}

func TestAARedirectsToTheNodeOfTheUE(t *testing.T) {
	elsewhere := netip.MustParseAddr("10.60.0.9")
	owners := &fakeOwners{owners: map[netip.Addr]pcf.Owner{elsewhere: {URI: otherNode}}}

	p := newPCF(t)
	p.Attach(fakeDiameter{}, owners)

	ans := aa(t, p, "af;1", signallingAAR(elsewhere))

	r, err := diameter.ParseRedirect(ans)
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}

	if len(r.Hosts) != 1 || r.Hosts[0] != otherNode || r.Usage != diameter.AllSession || r.MaxCacheTime != 24*time.Hour {
		t.Fatalf("redirect = %+v", r)
	}

	requireResult(t, aa(t, p, "af;2", signallingAAR(testUEv4)), success)

	if len(owners.asked) != 1 {
		t.Fatalf("owners asked for %v, want only the UE hosted elsewhere", owners.asked)
	}
}

func TestAAForAUEThisNodeDoesNotServe(t *testing.T) {
	elsewhere := netip.MustParseAddr("10.60.0.9")
	notAvailable := tgpp.Experimental(tgpp.ResultIPCANSessionNotAvailable)

	cases := []struct {
		name   string
		owners *fakeOwners
		send   func(*testing.T, *pcf.PCF, string, rx.AARequest) *diameter.Message
	}{
		{name: "addressed to this node", owners: &fakeOwners{owners: map[netip.Addr]pcf.Owner{elsewhere: {URI: otherNode}}}, send: addressedAA},
		{name: "held by this node", owners: &fakeOwners{owners: map[netip.Addr]pcf.Owner{elsewhere: {Local: true}}}, send: aa},
		{name: "held by no node", owners: &fakeOwners{}, send: aa},
		{name: "owner unknown", owners: &fakeOwners{err: errors.New("database closed")}, send: aa},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newPCF(t)
			p.Attach(fakeDiameter{}, tc.owners)

			requireResult(t, tc.send(t, p, "af;1", signallingAAR(elsewhere)), notAvailable)
		})
	}
}

func TestAAAsksForTheIPv6PrefixOfTheUE(t *testing.T) {
	owners := &fakeOwners{}

	p := newPCF(t)
	p.Attach(fakeDiameter{}, owners)

	requireResult(t, aa(t, p, "af;1", signallingAAR(netip.MustParseAddr("fd60:0:0:9::1234"))), tgpp.Experimental(tgpp.ResultIPCANSessionNotAvailable))

	if len(owners.asked) != 1 || owners.asked[0] != netip.MustParseAddr("fd60:0:0:9::") {
		t.Fatalf("owners asked for %v, want the /64 prefix", owners.asked)
	}
}
