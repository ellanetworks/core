// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"slices"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runOperatorIMSMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	t.Cleanup(func() {
		if err := c.UpdateIMSSettings(ctx, &client.UpdateIMSSettingsOptions{AdvertiseVoice: true}); err != nil {
			t.Logf("cleanup: clear P-CSCF addresses: %v", err)
		}
	})

	cases := []struct {
		name       string
		addresses  []string
		homeDomain string
		voice      bool
		restrict   bool
	}{
		{name: "set", addresses: []string{"192.0.2.20", "2001:db8::20"}, voice: true},
		{name: "reorder", addresses: []string{"2001:db8::20", "192.0.2.20"}, voice: true},
		{name: "home_domain", addresses: []string{"2001:db8::20", "192.0.2.20"}, homeDomain: "ims.example.com", voice: true},
		{name: "voice_off", addresses: []string{"2001:db8::20"}, homeDomain: "ims.example.com"},
		{name: "restrict_peers", addresses: []string{"2001:db8::20"}, homeDomain: "ims.example.com", restrict: true},
		{name: "clear", addresses: []string{}, voice: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := c.UpdateIMSSettings(ctx, &client.UpdateIMSSettingsOptions{PCSCFAddresses: tc.addresses, HomeDomain: tc.homeDomain, AdvertiseVoice: tc.voice, RestrictPeers: tc.restrict}); err != nil {
				t.Fatalf("update IMS: %v", err)
			}

			ims, err := c.GetIMSSettings(ctx)
			if err != nil {
				t.Fatalf("get IMS settings after update: %v", err)
			}

			if !slices.Equal(ims.PCSCFAddresses, tc.addresses) || ims.HomeDomain != tc.homeDomain || ims.AdvertiseVoice != tc.voice || ims.RestrictPeers != tc.restrict {
				t.Fatalf("IMS settings: got %v %q %t %t, want %v %q %t %t", ims.PCSCFAddresses, ims.HomeDomain, ims.AdvertiseVoice, ims.RestrictPeers, tc.addresses, tc.homeDomain, tc.voice, tc.restrict)
			}
		})
	}
}
