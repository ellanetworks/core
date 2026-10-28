// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/internal/hss"
)

func TestSubscription(t *testing.T) {
	impi := impiOf(testIMSI)
	identities := func(mmtel, mcptt hss.UserState) []hss.PublicIdentity {
		return []hss.PublicIdentity{
			{Identity: "sip:+" + testMSISDN + "@" + testIMSDomain, RegistrationSet: hss.MMTel, UserState: mmtel},
			{Identity: "tel:+" + testMSISDN, RegistrationSet: hss.MMTel, UserState: mmtel},
			{Identity: "sip:" + impi, Barred: true, RegistrationSet: hss.MMTel, UserState: mmtel},
			{Identity: "sip:mcptt-" + testMSISDN + "@" + testIMSDomain, RegistrationSet: hss.MCPTT, UserState: mcptt},
		}
	}

	cases := []struct {
		name string
		reg  *hss.Registration
		want *hss.IMSSubscription
	}{
		{name: "not registered", want: &hss.IMSSubscription{PrivateIdentity: impi, PublicIdentities: identities(hss.UserNotRegistered, hss.UserNotRegistered)}},
		{
			name: "registered",
			reg:  &hss.Registration{ServerName: testSCSCF, Sets: sets{hss.MMTel: {State: hss.Registered}}},
			want: &hss.IMSSubscription{PrivateIdentity: impi, SCSCFName: testSCSCF, PublicIdentities: identities(hss.UserRegistered, hss.UserNotRegistered)},
		},
		{
			name: "registered for unregistered services",
			reg:  &hss.Registration{ServerName: testSCSCF, Sets: sets{hss.MMTel: {State: hss.Unregistered, AuthPending: true}}},
			want: &hss.IMSSubscription{PrivateIdentity: impi, SCSCFName: testSCSCF, PublicIdentities: identities(hss.UserRegisteredUnregServices, hss.UserNotRegistered)},
		},
		{
			name: "authentication pending",
			reg:  &hss.Registration{ServerName: testSCSCF, Sets: sets{hss.MMTel: {State: hss.NotRegistered, AuthPending: true}}},
			want: &hss.IMSSubscription{PrivateIdentity: impi, SCSCFName: testSCSCF, PublicIdentities: identities(hss.UserAuthenticationPending, hss.UserNotRegistered)},
		},
		{
			name: "mcptt registered alone",
			reg:  &hss.Registration{ServerName: testSCSCF, Sets: sets{hss.MCPTT: {State: hss.Registered}}},
			want: &hss.IMSSubscription{PrivateIdentity: impi, SCSCFName: testSCSCF, PublicIdentities: identities(hss.UserNotRegistered, hss.UserRegistered)},
		},
		{
			name: "both registered",
			reg:  &hss.Registration{ServerName: testSCSCF, Sets: sets{hss.MMTel: {State: hss.Registered}, hss.MCPTT: {State: hss.NotRegistered, AuthPending: true}}},
			want: &hss.IMSSubscription{PrivateIdentity: impi, SCSCFName: testSCSCF, PublicIdentities: identities(hss.UserRegistered, hss.UserAuthenticationPending)},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeIMSStore()
			if tc.reg != nil {
				store.registrations[testIMSI] = *tc.reg
			}

			got, err := newHSS(store).Subscription(context.Background(), testIMSI)
			if err != nil {
				t.Fatalf("Subscription: %v", err)
			}

			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Subscription = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestSubscriptionWithoutAnMSISDNHasOnlyTheBarredIdentity(t *testing.T) {
	got, err := newHSS(newFakeIMSStore()).Subscription(context.Background(), testNoMSISDN)
	if err != nil {
		t.Fatalf("Subscription: %v", err)
	}

	want := []hss.PublicIdentity{{Identity: "sip:" + impiOf(testNoMSISDN), Barred: true, RegistrationSet: hss.MMTel, UserState: hss.UserNotRegistered}}
	if got == nil || !reflect.DeepEqual(got.PublicIdentities, want) {
		t.Fatalf("Subscription = %+v, want public identities %+v", got, want)
	}
}

func TestSubscriptionOfASubscriberWithoutIMS(t *testing.T) {
	got, err := newHSS(newFakeIMSStore()).Subscription(context.Background(), testNoIMS)
	if err != nil || got != nil {
		t.Fatalf("Subscription = %+v, %v, want none", got, err)
	}

	if _, err := newHSS(newFakeIMSStore()).Subscription(context.Background(), "001010000000099"); !errors.Is(err, hss.ErrSubscriberUnknown) {
		t.Fatalf("Subscription of an unknown subscriber: %v, want ErrSubscriberUnknown", err)
	}
}

func TestSubscriptionStaysVisibleWhileRegistered(t *testing.T) {
	store := newFakeIMSStore()
	store.registrations[testNoIMS] = hss.Registration{ServerName: testSCSCF, Sets: sets{hss.MMTel: {State: hss.Registered}}}

	got, err := newHSS(store).Subscription(context.Background(), testNoIMS)
	if err != nil || got == nil || got.SCSCFName != testSCSCF || got.PublicIdentities[0].UserState != hss.UserRegistered {
		t.Fatalf("Subscription = %+v, %v, want the registration the HSS still holds", got, err)
	}
}
