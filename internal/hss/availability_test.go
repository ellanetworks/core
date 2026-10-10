// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/hss"
)

func realmRouted() tgpp.Envelope {
	env := envelope()
	env.DestinationHost = ""

	return env
}

func sar(t *testing.T, env tgpp.Envelope, r cx.ServerAssignmentRequest) *diameter.Message {
	t.Helper()

	r.ServerName = testSCSCF

	req, err := cx.NewServerAssignmentRequest(env, r)
	if err != nil {
		t.Fatalf("build SAR: %v", err)
	}

	return req
}

func mar(t *testing.T, env tgpp.Envelope) *diameter.Message {
	t.Helper()

	req, err := cx.NewMultimediaAuthRequest(env, cx.MultimediaAuthRequest{
		PrivateIdentity: impiOf(testIMSI),
		PublicIdentity:  "sip:" + impiOf(testIMSI),
		Scheme:          cx.SchemeDigestAKAv1MD5,
		ServerName:      testSCSCF,
	})
	if err != nil {
		t.Fatalf("build MAR: %v", err)
	}

	return req
}

func retransmitted(req *diameter.Message) *diameter.Message {
	again := *req
	again.Flags |= diameter.FlagRetransmit

	return &again
}

func TestUnavailableStateAnswersSoTheClientTriesAnotherNode(t *testing.T) {
	cases := map[string]struct {
		env       tgpp.Envelope
		contended bool
		want      uint32
	}{
		"destination host": {env: envelope(), want: diameter.ResultTooBusy},
		"realm routed":     {env: realmRouted(), want: diameter.ResultUnableToDeliver},
		"contended":        {env: envelope(), contended: true, want: diameter.ResultTooBusy},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			for procedure, call := range map[string]func(h *hss.HSS) *diameter.Message{
				"SAR": func(h *hss.HSS) *diameter.Message {
					return h.ServerAssignment(context.Background(), hssIdentity, sar(t, tc.env, register(testIMSI, cx.AssignmentRegistration)))
				},
				"MAR": func(h *hss.HSS) *diameter.Message {
					return h.MultimediaAuth(context.Background(), hssIdentity, mar(t, tc.env))
				},
			} {
				store := newFakeIMSStore()
				store.unavailable = !tc.contended
				store.contended = tc.contended

				ans := call(newHSS(store))

				requireResult(t, ans, tgpp.Result{Code: tc.want})

				if ans.Flags&diameter.FlagError == 0 {
					t.Fatalf("%s answer has no E bit", procedure)
				}
			}
		})
	}
}

func TestUnavailableVectorAnswersSoTheClientTriesAnotherNode(t *testing.T) {
	store := newFakeIMSStore()
	h := newHSS(store)

	requireResult(t, h.ServerAssignment(context.Background(), hssIdentity, sar(t, envelope(), register(testIMSI, cx.AssignmentRegistration))),
		tgpp.Result{Code: diameter.ResultSuccess})

	store.unavailable = true

	requireResult(t, h.MultimediaAuth(context.Background(), hssIdentity, mar(t, envelope())), tgpp.Result{Code: diameter.ResultTooBusy})
}

func TestRetransmittedRequestsGetTheSameOutcome(t *testing.T) {
	store := newFakeIMSStore()
	h := newHSS(store)
	ctx := context.Background()

	steps := []struct {
		name string
		req  *diameter.Message
		want *hss.Registration
	}{
		{"MAR", mar(t, envelope()), &hss.Registration{State: hss.NotRegistered, ServerName: testSCSCF, AuthPending: true}},
		{"SAR registration", sar(t, envelope(), register(testIMSI, cx.AssignmentRegistration)), &hss.Registration{State: hss.Registered, ServerName: testSCSCF}},
		{"SAR re-registration", sar(t, envelope(), register(testIMSI, cx.AssignmentReRegistration)), &hss.Registration{State: hss.Registered, ServerName: testSCSCF}},
		{"SAR user deregistration", sar(t, envelope(), register(testIMSI, cx.AssignmentUserDeregistration)), nil},
	}

	for _, step := range steps {
		for _, req := range []*diameter.Message{step.req, retransmitted(step.req)} {
			var ans *diameter.Message

			if req.CommandCode == cx.CommandMultimediaAuth {
				ans = h.MultimediaAuth(ctx, hssIdentity, req)
			} else {
				ans = h.ServerAssignment(ctx, hssIdentity, req)
			}

			requireResult(t, ans, tgpp.Result{Code: diameter.ResultSuccess})
			requireRegistration(t, store, step.want)
		}
	}
}

func TestCommitThatOutlastsTheRequestAnswersUnavailable(t *testing.T) {
	store := newFakeIMSStore()
	store.block = make(chan struct{})

	t.Cleanup(func() { close(store.block) })

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	ans := newHSS(store).ServerAssignment(ctx, hssIdentity, sar(t, envelope(), register(testIMSI, cx.AssignmentRegistration)))

	requireResult(t, ans, tgpp.Result{Code: diameter.ResultTooBusy})
}
