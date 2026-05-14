package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runBGPSettingsMatrix round-trips the BGP speaker configuration.
// Validation constraints enforced by the server: LocalAS in [1, 4294967295];
// RouterID is a valid IPv4 address or empty (server picks the effective
// one); ListenAddress is "host:port" or ":port", defaulting to ":179".
func runBGPSettingsMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	orig, err := c.GetBGPSettings(ctx)
	if err != nil {
		t.Fatalf("get bgp settings (baseline): %v", err)
	}

	restore := &client.UpdateBGPSettingsOptions{
		Enabled:       orig.Enabled,
		LocalAS:       orig.LocalAS,
		RouterID:      orig.RouterID,
		ListenAddress: orig.ListenAddress,
	}

	if restore.LocalAS == 0 {
		restore.LocalAS = 65000
	}

	if restore.ListenAddress == "" {
		restore.ListenAddress = ":179"
	}

	if err := c.UpdateBGPSettings(ctx, restore); err != nil {
		t.Fatalf("set bgp settings baseline: %v", err)
	}

	t.Cleanup(func() {
		if err := c.UpdateBGPSettings(ctx, restore); err != nil {
			t.Logf("cleanup: restore bgp settings: %v", err)
		}
	})

	cases := []struct {
		field  string
		mutate func(o *client.UpdateBGPSettingsOptions)
		assert func(t *testing.T, s *client.GetBGPSettingsResponse)
	}{
		{
			field: "Enabled",
			mutate: func(o *client.UpdateBGPSettingsOptions) {
				o.Enabled = !restore.Enabled
			},
			assert: func(t *testing.T, s *client.GetBGPSettingsResponse) {
				if s.Enabled != !restore.Enabled {
					t.Fatalf("Enabled: got %t, want %t", s.Enabled, !restore.Enabled)
				}
			},
		},
		{
			field: "LocalAS",
			mutate: func(o *client.UpdateBGPSettingsOptions) {
				o.LocalAS = 65042
			},
			assert: func(t *testing.T, s *client.GetBGPSettingsResponse) {
				if s.LocalAS != 65042 {
					t.Fatalf("LocalAS: got %d, want 65042", s.LocalAS)
				}
			},
		},
		{
			field: "RouterID",
			mutate: func(o *client.UpdateBGPSettingsOptions) {
				o.RouterID = "10.99.99.99"
			},
			assert: func(t *testing.T, s *client.GetBGPSettingsResponse) {
				if s.RouterID != "10.99.99.99" {
					t.Fatalf("RouterID: got %q, want %q", s.RouterID, "10.99.99.99")
				}
			},
		},
		{
			field: "ListenAddress",
			mutate: func(o *client.UpdateBGPSettingsOptions) {
				o.ListenAddress = ":1790"
			},
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

			if err := c.UpdateBGPSettings(ctx, &opts); err != nil {
				t.Fatalf("update bgp settings (%s): %v", tc.field, err)
			}

			got, err := c.GetBGPSettings(ctx)
			if err != nil {
				t.Fatalf("get bgp settings after update: %v", err)
			}

			tc.assert(t, got)
		})
	}
}
