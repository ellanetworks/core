// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/udm"
)

type lookupBearerStore struct {
	fakeBearerStore
	sub *db.Subscriber
	err error
}

func (s lookupBearerStore) GetSubscriberOrVisitor(context.Context, string) (*db.Subscriber, error) {
	return s.sub, s.err
}

func TestResolveAccessOffersFiveGSInterworkingToHomeSubscribersOnly(t *testing.T) {
	const imsi = "001010000000001"

	partner := "partner"
	failure := errors.New("database unavailable")

	for _, tc := range []struct {
		name    string
		store   lookupBearerStore
		visitor bool
		want    bool
		wantErr error
	}{
		{name: "home subscriber", store: lookupBearerStore{sub: &db.Subscriber{Imsi: imsi, ProfileID: "test-profile"}}, want: true},
		{name: "roaming visitor", store: lookupBearerStore{sub: &db.Subscriber{Imsi: imsi, ProfileID: "test-profile", RoamingPartnerID: &partner}}, visitor: true},
		{name: "roaming visitor without a home subscription", store: lookupBearerStore{sub: &db.Subscriber{Imsi: imsi, ProfileID: "test-profile", RoamingPartnerID: &partner}}, wantErr: roaming.ErrNoHomeSubscription},
		{name: "subscriber row gone", store: lookupBearerStore{err: db.ErrNotFound}, wantErr: udm.ErrSubscriberUnknown},
		{name: "lookup failure", store: lookupBearerStore{err: failure}, wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMME(t)
			m.Bearer = tc.store

			if tc.visitor {
				idleRegisteredUE(t, m).SetVisitorForTest(roaming.Visitor{PartnerID: partner, Subscription: &s6a.SubscriptionData{}})
			}

			access, err := ResolveAccess(t.Context(), m, imsi)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if !access.Allow4G {
				t.Error("the visitor check took away 4G access")
			}

			if access.FiveGSInterworking != tc.want {
				t.Errorf("FiveGSInterworking = %v, want %v", access.FiveGSInterworking, tc.want)
			}
		})
	}
}

func TestHandoverToFiveGSOfARoamingVisitorIsRefusedAtTheSource(t *testing.T) {
	partner := "partner"
	m := newTestMME(t)
	peer := &fakeFiveGSPeer{}
	m.FiveGS = peer
	m.Bearer = lookupBearerStore{sub: &db.Subscriber{Imsi: "001010000000001", ProfileID: "test-profile", RoamingPartnerID: &partner}}

	ue, _ := idleMobilityUE(t, m)
	ue.SetVisitorForTest(roaming.Visitor{PartnerID: partner, Subscription: &s6a.SubscriptionData{}})

	access, err := ResolveAccess(t.Context(), m, ue.IMSI())
	if err != nil {
		t.Fatalf("ResolveAccess: %v", err)
	}

	ue.SetAccess(access)

	ue.mu.Lock()
	nh, ncc := ue.nh, ue.ncc
	ue.mu.Unlock()

	_, err = m.PrepareHandoverToFiveGS(ue, ue.Conn(), interworking.NGRANIdentity{}, nil, nil)
	if !errors.Is(err, ErrFiveGSMobilityBarred) {
		t.Fatalf("error = %v, want the source to refuse the handover to 5GS", err)
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.nh != nh || ue.ncc != ncc {
		t.Error("the refused handover advanced the next hop")
	}
}
