// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"testing"
)

func TestIMSSettingsAndPeersBeforeV27(t *testing.T) {
	ctx := context.Background()
	d := newDatabaseAtVersion(t, 26)

	if err := d.InitializeLocalSettings(ctx); err != nil {
		t.Fatalf("initialize local settings: %v", err)
	}

	if err := d.Initialize(ctx); err != nil {
		t.Fatalf("Initialize at schema 26: %v", err)
	}

	settings, err := d.GetIMSSettings(ctx)
	if err != nil || *settings != DefaultIMSSettings() {
		t.Fatalf("GetIMSSettings at schema 26 = %+v, %v; want the defaults", settings, err)
	}

	domains, err := d.GetIMSHomeDomains(ctx)
	if err != nil || domains.Primary == "" || domains.Primary != domains.Derived {
		t.Fatalf("GetIMSHomeDomains at schema 26 = %+v, %v; want the derived domain", domains, err)
	}

	peers, err := d.ListIMSPeers(ctx)
	if err != nil || len(peers) != 0 {
		t.Fatalf("ListIMSPeers at schema 26 = %+v, %v; want none", peers, err)
	}

	if _, err := d.GetIMSPeer(ctx, "scscf"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetIMSPeer at schema 26: err = %v, want ErrNotFound", err)
	}

	peer := func() *IMSPeer {
		return &IMSPeer{ID: "1", Name: "scscf", OriginHost: "scscf.example.org", Addresses: []string{"10.0.0.0/8"}}
	}

	writes := map[string]func() error{
		"UpdateIMSConfiguration": func() error {
			return d.UpdateIMSConfiguration(ctx, &IMSSettings{AdvertiseVoice: true, RestrictPeers: true}, []string{"10.0.0.5"})
		},
		"CreateIMSPeer": func() error { return d.CreateIMSPeer(ctx, peer()) },
		"UpdateIMSPeer": func() error { return d.UpdateIMSPeer(ctx, peer()) },
		"DeleteIMSPeer": func() error { return d.DeleteIMSPeer(ctx, "1") },
	}

	for name, write := range writes {
		if err := write(); !errors.Is(err, ErrMigrationPending) {
			t.Errorf("%s at schema 26: err = %v, want ErrMigrationPending", name, err)
		}
	}
}
