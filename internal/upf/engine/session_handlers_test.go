// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/engine"
)

func TestModifySessionSessionNotFound(t *testing.T) {
	conn, err := engine.NewSessionEngine(
		"1.2.3.4",
		"nodeId",
		"2.3.4.5",
		"",
		"2.3.4.5",
		"",
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Error creating PFCP connection: %v", err)
	}

	err = conn.ModifySession(context.Background(), &models.ModifyRequest{
		SEID: 999,
	})
	if err == nil {
		t.Fatal("Expected error for unknown SEID, got nil")
	}
}

func TestDeleteSessionAccepted(t *testing.T) {
	conn, err := engine.NewSessionEngine(
		"1.2.3.4",
		"nodeId",
		"2.3.4.5",
		"",
		"2.3.4.5",
		"",
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Error creating PFCP connection: %v", err)
	}

	seid := uint64(1)
	conn.AddSession(seid, engine.NewSession(seid))

	err = conn.DeleteSession(context.Background(), &models.DeleteRequest{SEID: seid})
	if err != nil {
		t.Fatalf("Error deleting session: %v", err)
	}

	if got := conn.GetSession(seid); got != nil {
		t.Fatalf("session %d still held after a successful delete", seid)
	}

	if err := conn.DeleteSession(context.Background(), &models.DeleteRequest{SEID: seid}); err == nil {
		t.Fatal("deleting the same SEID twice must report it unknown")
	}
}

func TestDeleteSessionNotFound(t *testing.T) {
	conn, err := engine.NewSessionEngine(
		"1.2.3.4",
		"nodeId",
		"2.3.4.5",
		"",
		"2.3.4.5",
		"",
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Error creating PFCP connection: %v", err)
	}

	err = conn.DeleteSession(context.Background(), &models.DeleteRequest{SEID: 999})
	if err == nil {
		t.Fatal("Expected error for unknown SEID, got nil")
	}
}

func TestModifySessionAccepted(t *testing.T) {
	conn, err := engine.NewSessionEngine(
		"1.2.3.4",
		"nodeId",
		"2.3.4.5",
		"",
		"2.3.4.5",
		"",
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Error creating PFCP connection: %v", err)
	}

	seid := uint64(1)
	conn.AddSession(seid, engine.NewSession(seid))

	err = conn.ModifySession(context.Background(), &models.ModifyRequest{
		SEID: seid,
	})
	if err != nil {
		t.Fatalf("Error modifying session: %v", err)
	}

	got := conn.GetSession(seid)
	if got == nil {
		t.Fatalf("session %d dropped by a modification that changed nothing", seid)
	}

	if got.SEID != seid {
		t.Fatalf("session SEID = %d, want %d", got.SEID, seid)
	}
}
