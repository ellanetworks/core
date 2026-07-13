// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runCellPositionsMatrix drives the cell-position CRUD lifecycle. The server
// assigns the ID, so the created record is located by its cell identity, which
// is unique within the (rat, mcc, mnc) tuple.
func runCellPositionsMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	const cellIdentity = "00000abc"

	listAll := func() []client.CellPosition {
		items, err := c.ListCellPositions(ctx)
		if err != nil {
			t.Fatalf("list cell positions: %v", err)
		}

		return items
	}

	findByCellIdentity := func(items []client.CellPosition, id string) *client.CellPosition {
		for i := range items {
			if items[i].CellIdentity == id {
				return &items[i]
			}
		}

		return nil
	}

	baseline := listAll()

	createOpts := &client.CreateCellPositionOptions{
		RAT:          "nr",
		Mcc:          "001",
		Mnc:          "01",
		CellIdentity: cellIdentity,
		Latitude:     45.5017,
		Longitude:    -73.5673,
	}

	if err := c.CreateCellPosition(ctx, createOpts); err != nil {
		t.Fatalf("create cell position: %v", err)
	}

	afterCreate := listAll()

	created := findByCellIdentity(afterCreate, cellIdentity)
	if created == nil {
		t.Fatalf("list after create missing cell identity %q", cellIdentity)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := c.DeleteCellPosition(ctx, created.ID); err != nil {
			t.Logf("cleanup: delete cell position %q: %v", created.ID, err)
		}
	})

	if len(afterCreate) != len(baseline)+1 {
		t.Fatalf("list count after create: got %d, want %d", len(afterCreate), len(baseline)+1)
	}

	if created.RAT != createOpts.RAT || created.Mcc != createOpts.Mcc || created.Mnc != createOpts.Mnc ||
		created.Latitude != createOpts.Latitude || created.Longitude != createOpts.Longitude {
		t.Fatalf("post-create round-trip mismatch: got %+v, want %+v", created, createOpts)
	}

	got, err := c.GetCellPosition(ctx, created.ID)
	if err != nil {
		t.Fatalf("get cell position %q: %v", created.ID, err)
	}

	if got.ID != created.ID || got.CellIdentity != cellIdentity {
		t.Fatalf("get cell position mismatch: got %+v, want id=%q cell_identity=%q", got, created.ID, cellIdentity)
	}

	updateOpts := &client.UpdateCellPositionOptions{
		RAT:          createOpts.RAT,
		Mcc:          createOpts.Mcc,
		Mnc:          createOpts.Mnc,
		CellIdentity: cellIdentity,
		Latitude:     48.8566,
		Longitude:    2.3522,
	}

	if err := c.UpdateCellPosition(ctx, created.ID, updateOpts); err != nil {
		t.Fatalf("update cell position %q: %v", created.ID, err)
	}

	updated, err := c.GetCellPosition(ctx, created.ID)
	if err != nil {
		t.Fatalf("get cell position %q after update: %v", created.ID, err)
	}

	if updated.Latitude != updateOpts.Latitude || updated.Longitude != updateOpts.Longitude {
		t.Fatalf("post-update mismatch: got lat=%v lon=%v, want lat=%v lon=%v", updated.Latitude, updated.Longitude, updateOpts.Latitude, updateOpts.Longitude)
	}

	if err := c.DeleteCellPosition(ctx, created.ID); err != nil {
		t.Fatalf("delete cell position %q: %v", created.ID, err)
	}

	deleted = true

	_, err = c.GetCellPosition(ctx, created.ID)
	assertNotFound(t, err, "cell position after delete")

	afterDelete := listAll()
	if len(afterDelete) != len(baseline) {
		t.Fatalf("list count after delete: got %d, want %d", len(afterDelete), len(baseline))
	}

	if findByCellIdentity(afterDelete, cellIdentity) != nil {
		t.Fatalf("list after delete still contains cell identity %q", cellIdentity)
	}
}
