// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"testing"
)

type fakeSMSCore struct {
	granted   bool
	connected bool
	err       error
	calls     int
	settled   int
}

func (c *fakeSMSCore) SMSRoute(string) (bool, bool) {
	return c.granted, c.connected
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

func TestSMSTransportRoutesToTheGrantedCore(t *testing.T) {
	cases := []struct {
		name         string
		fiveGS, eps  fakeSMSCore
		wantEPSCalls int
	}{
		{name: "granted on EPS only", eps: fakeSMSCore{granted: true}, wantEPSCalls: 2},
		{name: "granted on 5GS only", fiveGS: fakeSMSCore{granted: true}, wantEPSCalls: 0},
		{name: "granted on both, connected on EPS", fiveGS: fakeSMSCore{granted: true}, eps: fakeSMSCore{granted: true, connected: true}, wantEPSCalls: 2},
		{name: "granted on both, neither connected", fiveGS: fakeSMSCore{granted: true}, eps: fakeSMSCore{granted: true}, wantEPSCalls: 0},
		{name: "granted nowhere", wantEPSCalls: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transport := &smsTransport{cores: []smsCore{&tc.fiveGS, &tc.eps}}

			if err := transport.EnableUEReachability(context.Background(), "001010000000001"); err != nil {
				t.Fatalf("EnableUEReachability: %v", err)
			}

			if err := transport.SendSMS(context.Background(), "001010000000001", []byte{0x09}); err != nil {
				t.Fatalf("SendSMS: %v", err)
			}

			if tc.eps.calls != tc.wantEPSCalls || tc.fiveGS.calls != 2-tc.wantEPSCalls {
				t.Fatalf("EPS calls = %d, 5GS calls = %d, want %d EPS calls", tc.eps.calls, tc.fiveGS.calls, tc.wantEPSCalls)
			}
		})
	}
}

func TestSMSTransportSettlesSignallingOnBothCores(t *testing.T) {
	fiveGS, eps := &fakeSMSCore{}, &fakeSMSCore{granted: true}
	transport := &smsTransport{cores: []smsCore{fiveGS, eps}}

	transport.SignallingSettled(context.Background(), "001010000000001")

	if fiveGS.settled != 1 || eps.settled != 1 {
		t.Fatalf("settled 5GS %d, EPS %d times, want once each", fiveGS.settled, eps.settled)
	}
}
