// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"slices"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runOperatorIMSHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients

	t.Cleanup(func() {
		if err := h.Leader.UpdateIMSSettings(ctx, &client.UpdateIMSSettingsOptions{AdvertiseVoice: true}); err != nil {
			t.Logf("cleanup: clear P-CSCF addresses: %v", err)
		}
	})

	cases := []struct {
		name      string
		writer    int
		addresses []string
		voice     bool
	}{
		{name: "set", writer: 0, addresses: []string{"192.0.2.20", "2001:db8::20"}},
		{name: "clear", writer: 1, addresses: []string{}, voice: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := nodes[tc.writer].UpdateIMSSettings(ctx, &client.UpdateIMSSettingsOptions{PCSCFAddresses: tc.addresses, AdvertiseVoice: tc.voice}); err != nil {
				t.Fatalf("update IMS on node %d: %v", tc.writer+1, err)
			}

			awaitConvergence(ctx, t, h)

			for i, c := range nodes {
				ims, err := c.GetIMSSettings(ctx)
				if err != nil {
					t.Fatalf("node %d get IMS settings after update: %v", i+1, err)
				}

				if !slices.Equal(ims.PCSCFAddresses, tc.addresses) || ims.AdvertiseVoice != tc.voice {
					t.Fatalf("node %d IMS settings: got %v %t, want %v %t", i+1, ims.PCSCFAddresses, ims.AdvertiseVoice, tc.addresses, tc.voice)
				}
			}
		})
	}
}
