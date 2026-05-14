package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runBGPSettingsHAMatrix updates the singleton on one node and asserts
// the other two retain their per-node baseline values. The cleanup
// restores the writer's pre-test settings; the other nodes are never
// touched.
func runBGPSettingsHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	const writerIdx = 2

	writer := h.Clients[writerIdx]

	baselines := make(map[int]*client.GetBGPSettingsResponse, len(h.Clients))

	for i, c := range h.Clients {
		got, err := c.GetBGPSettings(ctx)
		if err != nil {
			t.Fatalf("node %d get bgp settings baseline: %v", i+1, err)
		}

		baselines[i] = got
	}

	restore := &client.UpdateBGPSettingsOptions{
		Enabled:       baselines[writerIdx].Enabled,
		LocalAS:       baselines[writerIdx].LocalAS,
		RouterID:      baselines[writerIdx].RouterID,
		ListenAddress: baselines[writerIdx].ListenAddress,
	}

	if restore.LocalAS == 0 {
		restore.LocalAS = 65000
	}

	if restore.ListenAddress == "" {
		restore.ListenAddress = ":179"
	}

	if err := writer.UpdateBGPSettings(ctx, restore); err != nil {
		t.Fatalf("set bgp settings baseline on writer: %v", err)
	}

	t.Cleanup(func() {
		if err := writer.UpdateBGPSettings(ctx, restore); err != nil {
			t.Logf("cleanup: restore bgp settings on writer: %v", err)
		}
	})

	assertLocalityBGPSettings := func(t *testing.T, phase string) {
		t.Helper()

		stabilizeLocal()

		for i, c := range h.Clients {
			if i == writerIdx {
				continue
			}

			got, err := c.GetBGPSettings(ctx)
			if err != nil {
				t.Fatalf("node %d get bgp settings after %s: %v", i+1, phase, err)
			}

			if got.Enabled != baselines[i].Enabled ||
				got.LocalAS != baselines[i].LocalAS ||
				got.RouterID != baselines[i].RouterID ||
				got.ListenAddress != baselines[i].ListenAddress {
				t.Fatalf("node %d bgp settings drifted after writer-only %s: got %+v, want %+v",
					i+1, phase, got, baselines[i])
			}
		}
	}

	cases := []struct {
		field  string
		mutate func(o *client.UpdateBGPSettingsOptions)
		assert func(t *testing.T, s *client.GetBGPSettingsResponse)
	}{
		{
			field:  "Enabled",
			mutate: func(o *client.UpdateBGPSettingsOptions) { o.Enabled = !restore.Enabled },
			assert: func(t *testing.T, s *client.GetBGPSettingsResponse) {
				if s.Enabled != !restore.Enabled {
					t.Fatalf("Enabled: got %t, want %t", s.Enabled, !restore.Enabled)
				}
			},
		},
		{
			field:  "LocalAS",
			mutate: func(o *client.UpdateBGPSettingsOptions) { o.LocalAS = 65042 },
			assert: func(t *testing.T, s *client.GetBGPSettingsResponse) {
				if s.LocalAS != 65042 {
					t.Fatalf("LocalAS: got %d, want 65042", s.LocalAS)
				}
			},
		},
		{
			field:  "RouterID",
			mutate: func(o *client.UpdateBGPSettingsOptions) { o.RouterID = "10.99.99.99" },
			assert: func(t *testing.T, s *client.GetBGPSettingsResponse) {
				if s.RouterID != "10.99.99.99" {
					t.Fatalf("RouterID: got %q, want %q", s.RouterID, "10.99.99.99")
				}
			},
		},
		{
			field:  "ListenAddress",
			mutate: func(o *client.UpdateBGPSettingsOptions) { o.ListenAddress = ":1790" },
			assert: func(t *testing.T, s *client.GetBGPSettingsResponse) {
				if s.ListenAddress != ":1790" {
					t.Fatalf("ListenAddress: got %q, want %q", s.ListenAddress, ":1790")
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("update_"+tc.field, func(t *testing.T) {
			opts := *restore
			tc.mutate(&opts)

			if err := writer.UpdateBGPSettings(ctx, &opts); err != nil {
				t.Fatalf("update bgp settings on writer (%s): %v", tc.field, err)
			}

			got, err := writer.GetBGPSettings(ctx)
			if err != nil {
				t.Fatalf("get bgp settings on writer after update: %v", err)
			}

			tc.assert(t, got)

			assertLocalityBGPSettings(t, "update_"+tc.field)
		})
	}
}
