// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss_test

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/hss"
	"github.com/ellanetworks/core/internal/udm"
	"go.uber.org/zap"
)

func TestPermanentTerminationUnacknowledged_TS29228_6_1_3_1(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(store *fakeIMSStore)
	}{
		{name: "MSISDN removed", change: func(store *fakeIMSStore) { store.subscribers[testIMSI].MSISDN = "" }},
		{name: "IMS data network lost", change: func(store *fakeIMSStore) { store.subscribers[testIMSI].IMSDataNetwork = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeIMSStore()
			scscf := &fakeSCSCF{rtrResult: tgpp.Result{Code: diameter.ResultUnableToComply}}
			h := connectedHSS(t, store, scscf)

			assigned(t, store, testIMSI, cx.AssignmentRegistration)
			tc.change(store)

			if err := h.Reconcile(context.Background(), testIMSI); !errors.Is(err, hss.ErrTerminationUnacknowledged) {
				t.Fatalf("Reconcile: %v, want %v", err, hss.ErrTerminationUnacknowledged)
			}

			if got := scscf.Terminations(); !slices.Equal(got, []cx.ReasonCode{cx.ReasonPermanentTermination}) {
				t.Fatalf("registration terminations = %v, want PERMANENT_TERMINATION", got)
			}

			if reg, kept := registration(t, store, testIMSI); kept {
				t.Fatalf("registration = %+v, want it cleared", reg)
			}
		})
	}
}

func TestDeregister_TS29228_6_1_3_1(t *testing.T) {
	for _, tc := range []struct {
		name           string
		rtrResult      tgpp.Result
		assignment     cx.AssignmentType
		wantErr        error
		wantTerminated [][]string
	}{
		{name: "registered", assignment: cx.AssignmentRegistration, wantTerminated: [][]string{{"sip:+" + testMSISDN + "@" + testIMSDomain}}},
		{name: "unregistered user", assignment: cx.AssignmentUnregisteredUser, wantTerminated: [][]string{{"sip:+" + testMSISDN + "@" + testIMSDomain}}},
		{name: "refused by the S-CSCF", assignment: cx.AssignmentRegistration, rtrResult: tgpp.Result{Code: diameter.ResultUnableToComply}, wantErr: hss.ErrTerminationUnacknowledged, wantTerminated: [][]string{{"sip:+" + testMSISDN + "@" + testIMSDomain}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeIMSStore()
			scscf := &fakeSCSCF{rtrResult: tc.rtrResult}
			sms := newFakeSMS()
			h := connectedHSSWithSMS(t, store, scscf, sms)

			assigned(t, store, testIMSI, tc.assignment)
			sms.registrations[testIMSI] = hss.IPSMGWRegistration{Number: "15550000100", DiameterHost: ipSMGWIdentity.OriginHost, DiameterRealm: ipSMGWIdentity.OriginRealm}

			if err := h.Deregister(context.Background(), testIMSI); !errors.Is(err, tc.wantErr) || (tc.wantErr == nil) != (err == nil) {
				t.Fatalf("Deregister: %v, want %v", err, tc.wantErr)
			}

			if got := scscf.Terminations(); !slices.Equal(got, []cx.ReasonCode{cx.ReasonServerChange}) {
				t.Fatalf("registration terminations = %v, want SERVER_CHANGE", got)
			}

			if got := scscf.TerminatedIdentities(); !slices.EqualFunc(got, tc.wantTerminated, slices.Equal) {
				t.Fatalf("terminated identities = %v, want %v", got, tc.wantTerminated)
			}

			if reg, kept := registration(t, store, testIMSI); kept {
				t.Fatalf("registration = %+v, want it cleared", reg)
			}

			if r, ok := sms.registrations[testIMSI]; ok {
				t.Fatalf("IP-SM-GW registration = %+v, want it cleared", r)
			}
		})
	}
}

func TestDeregisterWithoutTheSCSCF(t *testing.T) {
	store := newFakeIMSStore()
	sms := newFakeSMS()
	node := newTestDiameterNode(t, hssIdentity, netip.MustParseAddr("127.0.0.1"), diameter.HandlerFunc(func(_ context.Context, c *diameter.Conn, req *diameter.Message) *diameter.Message {
		return c.Answer(req, diameter.ResultCommandUnsupported)
	}))
	h := hss.New(store, udm.NewIMSCredentials(store), nodePeers{node: node}, sms, zap.NewNop())

	assigned(t, store, testIMSI, cx.AssignmentRegistration)
	sms.registrations[testIMSI] = hss.IPSMGWRegistration{Number: "15550000100", DiameterHost: ipSMGWIdentity.OriginHost, DiameterRealm: ipSMGWIdentity.OriginRealm}

	if err := h.Deregister(context.Background(), testIMSI); !errors.Is(err, hss.ErrTerminationUnacknowledged) {
		t.Fatalf("Deregister: %v, want %v", err, hss.ErrTerminationUnacknowledged)
	}

	if reg, kept := registration(t, store, testIMSI); kept {
		t.Fatalf("registration = %+v, want it cleared", reg)
	}

	if r, ok := sms.registrations[testIMSI]; ok {
		t.Fatalf("IP-SM-GW registration = %+v, want it cleared", r)
	}
}

func TestDeregisterNotRegistered(t *testing.T) {
	store := newFakeIMSStore()
	scscf := &fakeSCSCF{}
	sms := newFakeSMS()
	h := connectedHSSWithSMS(t, store, scscf, sms)

	sms.registrations[testIMSI] = hss.IPSMGWRegistration{Number: "15550000100", DiameterHost: ipSMGWIdentity.OriginHost, DiameterRealm: ipSMGWIdentity.OriginRealm}

	if err := h.Deregister(context.Background(), testIMSI); err != nil {
		t.Fatalf("Deregister: %v", err)
	}

	if got := scscf.Terminations(); len(got) != 0 {
		t.Fatalf("registration terminations = %v, want none", got)
	}

	if r, ok := sms.registrations[testIMSI]; ok {
		t.Fatalf("IP-SM-GW registration = %+v, want it cleared", r)
	}
}
