// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestMigrationV26MovesSMSCPeersToSMSCNodes(t *testing.T) {
	ctx := context.Background()
	d := newDatabaseAtVersion(t, 25)

	peers := []legacySMSCPeer{
		{ID: "01890000-0000-7000-8000-000000000001", DiameterIdentity: "10.0.0.1", Address: "192.0.2.9", Port: 3868, ServiceCentres: []string{"15550000000", "15550000001"}},
		{ID: "01890000-0000-7000-8000-000000000002", Address: "10.0.0.1", Port: 3868, ServiceCentres: []string{"15550000002"}},
		{ID: "01890000-0000-7000-8000-000000000003", DiameterIdentity: "smsc.example.org", Address: "10.0.0.1", Port: 3869, ServiceCentres: []string{"15550000003"}},
	}

	for _, p := range peers {
		if _, err := d.applyCreateSMSCPeer(ctx, &p); err != nil {
			t.Fatalf("create legacy SMSC peer %s: %v", p.ID, err)
		}
	}

	want := []SMSC{
		{ID: peers[0].ID, Name: "10.0.0.1", ServiceCentres: []string{"15550000000", "15550000001"}, Nodes: []SMSCNode{{ID: peers[0].ID, DiameterIdentity: "10.0.0.1", Address: "192.0.2.9", Port: 3868}}},
		{ID: peers[1].ID, Name: "10.0.0.1 (2)", ServiceCentres: []string{"15550000002"}, Nodes: []SMSCNode{{ID: peers[1].ID, Address: "10.0.0.1", Port: 3868}}},
		{ID: peers[2].ID, Name: "smsc.example.org", ServiceCentres: []string{"15550000003"}, Nodes: []SMSCNode{{ID: peers[2].ID, DiameterIdentity: "smsc.example.org", Address: "10.0.0.1", Port: 3869}}},
	}

	assertSMSCs(t, d, "at v25", want)

	smsc := SMSC{ID: "01890000-0000-7000-8000-0000000000aa", Name: "new", ServiceCentres: []string{"15550000009"}, Nodes: []SMSCNode{{ID: "01890000-0000-7000-8000-0000000000ab", Address: "192.0.2.10", Port: 3868}}}
	if err := d.CreateSMSC(ctx, &smsc); !errors.Is(err, ErrMigrationPending) {
		t.Fatalf("CreateSMSC at v25 = %v, want ErrMigrationPending", err)
	}

	migrateTestDatabase(t, d, 26)

	assertSMSCs(t, d, "at v26", want)

	if _, err := d.applyCreateSMSCPeer(ctx, &legacySMSCPeer{ID: "01890000-0000-7000-8000-000000000004", Address: "192.0.2.4", Port: 3868, ServiceCentres: []string{"15550000004"}}); !errors.Is(err, ErrRetiredOperation) {
		t.Fatalf("legacy CreateSMSCPeer at v26 = %v, want ErrRetiredOperation", err)
	}

	second := SMSCNode{ID: "01890000-0000-7000-8000-0000000000ac", DiameterIdentity: "smsc2.example.org", Address: "10.0.0.2", Port: 3869}
	updated := want[2]
	updated.Nodes = append(slices.Clone(updated.Nodes), second)

	if _, err := d.applyUpdateSMSC(ctx, &updated); err != nil {
		t.Fatalf("add a node to a migrated SMSC: %v", err)
	}

	if _, err := d.applyDeleteSMSC(ctx, &stringPayload{Value: peers[0].ID}); err != nil {
		t.Fatalf("delete a migrated SMSC: %v", err)
	}

	assertSMSCs(t, d, "after edits", []SMSC{want[1], updated})

	var orphans int
	if err := d.PlainDB().QueryRowContext(ctx, "SELECT COUNT(*) FROM sms_smsc_service_centres WHERE smscId = ?", peers[0].ID).Scan(&orphans); err != nil || orphans != 0 {
		t.Fatalf("service centres of the deleted SMSC = %d, %v, want none", orphans, err)
	}
}

func assertSMSCs(t *testing.T, d *Database, label string, want []SMSC) {
	t.Helper()

	got, err := d.ListSMSCs(context.Background())
	if err != nil {
		t.Fatalf("ListSMSCs %s: %v", label, err)
	}

	if !slices.EqualFunc(got, want, func(x, y SMSC) bool {
		return x.ID == y.ID && x.Name == y.Name && slices.Equal(x.ServiceCentres, y.ServiceCentres) && slices.Equal(x.Nodes, y.Nodes)
	}) {
		t.Fatalf("SMSCs %s = %+v, want %+v", label, got, want)
	}
}

func migrateTestDatabase(t *testing.T, d *Database, version int) {
	t.Helper()

	ctx := context.Background()

	if err := runMigrations(ctx, d.PlainDB(), version); err != nil {
		t.Fatalf("runMigrations(%d): %v", version, err)
	}

	if err := d.refreshAppliedSchema(ctx); err != nil {
		t.Fatalf("refresh applied schema: %v", err)
	}
}
