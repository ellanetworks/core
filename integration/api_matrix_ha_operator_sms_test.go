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

	smscs := []client.SMSC{{Name: "main", ServiceCentres: []string{"+15550000000"}, Nodes: []client.SMSCNode{
		{OriginHost: "smsc1.node.example.org", Address: "192.0.2.10", Port: 3868},
		{OriginHost: "smsc2.node.example.org", Address: "192.0.2.11", Port: 3868},
	}}}

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
				_, err := c.CreateSMSC(ctx, &client.CreateSMSCOptions{
					Name:           "main",
					ServiceCentres: []string{"+15550000000"},
					Nodes: []client.SMSCNodeOptions{
						{OriginHost: "smsc1.node.example.org", Address: "192.0.2.10"},
						{OriginHost: "smsc2.node.example.org", Address: "192.0.2.11"},
					},
				})

				return err
			},
			want: smsState{SMS: client.SMSSettings{ServingNodeNumber: "+15550001111"}, SMSCs: smscs},
		},
		{
			name:   "set_number",
			writer: 0,
			apply: func(c *client.Client) error {
				return c.UpdateSMSSettings(ctx, &client.UpdateSMSSettingsOptions{ServingNodeNumber: "+15550009999"})
			},
			want: smsState{SMS: client.SMSSettings{ServingNodeNumber: "+15550009999"}, SMSCs: smscs},
		},
		{
			name:   "clear_number",
			writer: 1,
			apply: func(c *client.Client) error {
				return c.UpdateSMSSettings(ctx, &client.UpdateSMSSettingsOptions{})
			},
			want: smsState{SMSCs: smscs},
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
