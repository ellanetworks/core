// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

func runOperatorSMSMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	t.Cleanup(func() {
		if err := c.UpdateOperatorSMS(ctx, &client.UpdateOperatorSMSOptions{}); err != nil {
			t.Logf("cleanup: disable sms: %v", err)
		}
	})

	cases := []struct {
		name string
		opts *client.UpdateOperatorSMSOptions
		want client.GetOperatorSMSResponse
	}{
		{
			name: "enable",
			opts: &client.UpdateOperatorSMSOptions{Enabled: true, SMSCAddress: "192.0.2.10", SMSNumber: "+15550001111"},
			want: client.GetOperatorSMSResponse{Enabled: true, SMSCAddress: "192.0.2.10", SMSCPort: 3868, SMSNumber: "+15550001111"},
		},
		{
			name: "update_SMSC",
			opts: &client.UpdateOperatorSMSOptions{Enabled: true, SMSCAddress: "2001:db8::10", SMSCPort: 3869, SMSNumber: "+15550001111"},
			want: client.GetOperatorSMSResponse{Enabled: true, SMSCAddress: "2001:db8::10", SMSCPort: 3869, SMSNumber: "+15550001111"},
		},
		{
			name: "disable",
			opts: &client.UpdateOperatorSMSOptions{SMSCAddress: "2001:db8::10", SMSCPort: 3869, SMSNumber: "+15550001111"},
			want: client.GetOperatorSMSResponse{SMSCAddress: "2001:db8::10", SMSCPort: 3869, SMSNumber: "+15550001111"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := c.UpdateOperatorSMS(ctx, tc.opts); err != nil {
				t.Fatalf("update sms: %v", err)
			}

			op, err := c.GetOperator(ctx)
			if err != nil {
				t.Fatalf("get operator after update: %v", err)
			}

			if op.SMS != tc.want {
				t.Fatalf("sms: got %+v, want %+v", op.SMS, tc.want)
			}

			assertSMSCPeer(ctx, t, c, tc.want)
		})
	}
}

func assertSMSCPeer(ctx context.Context, t *testing.T, c *client.Client, sms client.GetOperatorSMSResponse) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)

	for {
		status, err := c.GetDiameterStatus(ctx)
		if err != nil {
			t.Fatalf("get diameter status: %v", err)
		}

		if status.Host == "" || status.Realm == "" {
			t.Fatalf("diameter identity missing: %+v", status)
		}

		matched := !sms.Enabled && len(status.Peers) == 0

		if sms.Enabled && len(status.Peers) == 1 {
			p := status.Peers[0]
			matched = p.Role == "smsc" && p.Address == sms.SMSCAddress && p.Port == sms.SMSCPort && p.State != "open"
		}

		if matched {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("diameter peers = %+v, want the SMSC %s:%d (enabled=%v)", status.Peers, sms.SMSCAddress, sms.SMSCPort, sms.Enabled)
		}

		time.Sleep(200 * time.Millisecond)
	}
}
