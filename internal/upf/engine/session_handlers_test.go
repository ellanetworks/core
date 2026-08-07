// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/engine"
)

// An unheld SEID is created, not refused: the statement is what the session is
// to be, whether or not the UPF already holds one.
func TestApplyCreatesUnheldSession(t *testing.T) {
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

	if _, err := conn.Apply(context.Background(), &models.SessionState{SEID: 999}); err != nil {
		t.Fatalf("Apply for an unheld SEID: %v", err)
	}

	if conn.GetSession(999) == nil {
		t.Fatal("session 999 is not held after Apply")
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

	err = conn.Delete(context.Background(), seid)
	if err != nil {
		t.Fatalf("Error deleting session: %v", err)
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

	err = conn.Delete(context.Background(), 999)
	if err == nil {
		t.Fatal("Expected error for unknown SEID, got nil")
	}
}

func TestApplyAccepted(t *testing.T) {
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

	if _, err := conn.Apply(context.Background(), &models.SessionState{SEID: seid}); err != nil {
		t.Fatalf("Error applying session state: %v", err)
	}
}
