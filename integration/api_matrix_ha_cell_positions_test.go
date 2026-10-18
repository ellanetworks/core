// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runCellPositionsHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients

	findOn := func(c *client.Client, cellIdentity string) *client.CellPosition {
		positions, err := c.ListCellPositions(ctx)
		if err != nil {
			t.Fatalf("list cell positions: %v", err)
		}

		for i := range positions {
			if positions[i].CellIdentity == cellIdentity {
				return &positions[i]
			}
		}

		return nil
	}

	createOpts := &client.CreateCellPositionOptions{
		RAT:          "nr",
		Mcc:          "001",
		Mnc:          "01",
		CellIdentity: "00000a001",
		Latitude:     45.5,
		Longitude:    -73.5,
	}

	if err := nodes[1].CreateCellPosition(ctx, createOpts); err != nil {
		t.Fatalf("create cell position on node 2: %v", err)
	}

	created := findOn(nodes[1], createOpts.CellIdentity)
	if created == nil {
		t.Fatalf("node 2 does not list the cell position it created")
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := h.Leader.DeleteCellPosition(ctx, created.ID); err != nil {
			t.Logf("cleanup: delete cell position: %v", err)
		}
	})

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		got, err := c.GetCellPosition(ctx, created.ID)
		if err != nil {
			t.Fatalf("node %d get after create: %v", i+1, err)
		}

		if got.Latitude != createOpts.Latitude || got.Longitude != createOpts.Longitude {
			t.Fatalf("node %d post-create mismatch: got %+v", i+1, got)
		}
	}

	updateOpts := &client.UpdateCellPositionOptions{
		RAT:          createOpts.RAT,
		Mcc:          createOpts.Mcc,
		Mnc:          createOpts.Mnc,
		CellIdentity: createOpts.CellIdentity,
		Latitude:     46.5,
		Longitude:    -72.5,
	}

	if err := nodes[2].UpdateCellPosition(ctx, created.ID, updateOpts); err != nil {
		t.Fatalf("update cell position on node 3: %v", err)
	}

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		got, err := c.GetCellPosition(ctx, created.ID)
		if err != nil {
			t.Fatalf("node %d get after update: %v", i+1, err)
		}

		if got.Latitude != updateOpts.Latitude || got.Longitude != updateOpts.Longitude {
			t.Fatalf("node %d post-update mismatch: got %+v", i+1, got)
		}
	}

	if err := nodes[0].DeleteCellPosition(ctx, created.ID); err != nil {
		t.Fatalf("delete cell position on node 1: %v", err)
	}

	deleted = true

	awaitConvergence(ctx, t, h)

	for i, c := range nodes {
		if got := findOn(c, createOpts.CellIdentity); got != nil {
			t.Fatalf("node %d still lists the deleted cell position: %+v", i+1, got)
		}
	}
}
