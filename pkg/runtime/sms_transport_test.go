// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/smsf"
)

type fakeSMSCore struct {
	err     error
	calls   int
	settled int
}

func (c *fakeSMSCore) EnableUEReachabilityForSMS(context.Context, string) error {
	c.calls++
	return c.err
}

func (c *fakeSMSCore) SendSMS(context.Context, string, []byte) error {
	c.calls++
	return c.err
}

func (c *fakeSMSCore) SMSSignallingSettled(context.Context, string) {
	c.settled++
}

func TestSMSTransportRoutesToTheRegisteredAccess(t *testing.T) {
	eps := &fakeSMSCore{}
	fiveGS := &fakeSMSCore{}
	transport := &smsTransport{cores: map[smsf.Access]smsCore{smsf.AccessEPS: eps, smsf.Access5GS: fiveGS}}

	if err := transport.SendSMS(context.Background(), "001010000000001", smsf.Access5GS, []byte{0x09}); err != nil {
		t.Fatalf("SendSMS: %v", err)
	}

	if err := transport.EnableUEReachability(context.Background(), "001010000000001", smsf.AccessEPS); err != nil {
		t.Fatalf("EnableUEReachability: %v", err)
	}

	transport.SignallingSettled(context.Background(), "001010000000001", smsf.AccessEPS)

	if eps.calls != 1 || fiveGS.calls != 1 || eps.settled != 1 || fiveGS.settled != 0 {
		t.Fatalf("EPS calls = %d settled = %d, 5GS calls = %d settled = %d", eps.calls, eps.settled, fiveGS.calls, fiveGS.settled)
	}
}

func TestSMSTransportMapsCoreErrorsToSMSFOutcomes(t *testing.T) {
	cases := []struct {
		name  string
		err   error
		check func(error) bool
	}{
		{
			name:  "no UE context",
			err:   amf.ErrSMSUENotRegistered,
			check: func(err error) bool { return errors.Is(err, smsf.ErrUserUnknown) },
		},
		{
			name:  "not registered for SMS",
			err:   mme.ErrSMSNotAllowed,
			check: func(err error) bool { return errors.Is(err, smsf.ErrNotRegisteredForSMS) },
		},
		{
			name: "no paging response",
			err:  mme.ErrSMSUEUnreachable,
			check: func(err error) bool {
				var absent *smsf.AbsentError
				return errors.As(err, &absent) && absent.Diagnostic == tgpp.AbsentUserNoPagingResponseMSC
			},
		},
		{
			name:  "deadline",
			err:   context.DeadlineExceeded,
			check: func(err error) bool { return errors.Is(err, context.DeadlineExceeded) },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transport := &smsTransport{cores: map[smsf.Access]smsCore{smsf.AccessEPS: &fakeSMSCore{err: tc.err}}}

			if err := transport.EnableUEReachability(context.Background(), "001010000000001", smsf.AccessEPS); !tc.check(err) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}
