// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runOperatorSMSHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients

	t.Cleanup(func() {
		if err := h.Leader.UpdateOperatorSMS(ctx, &client.UpdateOperatorSMSOptions{}); err != nil {
			t.Logf("cleanup: disable sms: %v", err)
		}
	})

	cases := []struct {
		name   string
		writer int
		opts   *client.UpdateOperatorSMSOptions
		want   client.GetOperatorSMSResponse
	}{
		{
			name:   "enable",
			writer: 0,
			opts:   &client.UpdateOperatorSMSOptions{Enabled: true, SMSCAddress: "192.0.2.10", SMSNumber: "+15550001111"},
			want:   client.GetOperatorSMSResponse{Enabled: true, SMSCAddress: "192.0.2.10", SMSCPort: 3868, SMSNumber: "+15550001111"},
		},
		{
			name:   "disable",
			writer: 1,
			opts:   &client.UpdateOperatorSMSOptions{SMSCAddress: "192.0.2.10", SMSNumber: "+15550001111"},
			want:   client.GetOperatorSMSResponse{SMSCAddress: "192.0.2.10", SMSCPort: 3868, SMSNumber: "+15550001111"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := nodes[tc.writer].UpdateOperatorSMS(ctx, tc.opts); err != nil {
				t.Fatalf("update sms on node %d: %v", tc.writer+1, err)
			}

			awaitConvergence(ctx, t, h)

			for i, c := range nodes {
				op, err := c.GetOperator(ctx)
				if err != nil {
					t.Fatalf("node %d get operator after update: %v", i+1, err)
				}

				if op.SMS != tc.want {
					t.Fatalf("node %d sms: got %+v, want %+v", i+1, op.SMS, tc.want)
				}
			}
		})
	}
}
