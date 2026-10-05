// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runOperatorSMSHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients

	t.Cleanup(func() { cleanupSMS(ctx, t, h.Leader) })

	peers := []client.SMSCPeer{{Address: "192.0.2.10", Port: 3868, ServiceCentres: []string{"+15550000000"}}}

	cases := []struct {
		name   string
		writer int
		apply  func(c *client.Client) error
		want   smsState
	}{
		{
			name:   "add_SMSC",
			writer: 2,
			apply: func(c *client.Client) error {
				_, err := c.CreateSMSCPeer(ctx, &client.SMSCPeerOptions{Address: "192.0.2.10", ServiceCentres: []string{"+15550000000"}})
				return err
			},
			want: smsState{Peers: peers},
		},
		{
			name:   "enable",
			writer: 0,
			apply: func(c *client.Client) error {
				return c.UpdateOperatorSMS(ctx, &client.UpdateOperatorSMSOptions{Enabled: true, SMSNumber: "+15550001111"})
			},
			want: smsState{SMS: client.GetOperatorSMSResponse{Enabled: true, SMSNumber: "+15550001111"}, Peers: peers},
		},
		{
			name:   "disable",
			writer: 1,
			apply: func(c *client.Client) error {
				return c.UpdateOperatorSMS(ctx, &client.UpdateOperatorSMSOptions{SMSNumber: "+15550001111"})
			},
			want: smsState{SMS: client.GetOperatorSMSResponse{SMSNumber: "+15550001111"}, Peers: peers},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.apply(nodes[tc.writer]); err != nil {
				t.Fatalf("update sms on node %d: %v", tc.writer+1, err)
			}

			awaitConvergence(ctx, t, h)

			for i, c := range nodes {
				if got := readSMSState(ctx, t, c); !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("node %d sms: got %+v, want %+v", i+1, got, tc.want)
				}
			}
		})
	}
}
