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
		"2.3.4.5",
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
		"2.3.4.5",
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Error creating PFCP connection: %v", err)
	}

	seid := uint64(1)
	conn.AddSession(seid, &engine.Session{})

	err = conn.DeleteSession(context.Background(), &models.DeleteRequest{SEID: seid})
	if err != nil {
		t.Fatalf("Error deleting session: %v", err)
	}
}

func TestDeleteSessionNotFound(t *testing.T) {
	conn, err := engine.NewSessionEngine(
		"1.2.3.4",
		"nodeId",
		"2.3.4.5",
		"2.3.4.5",
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
		"2.3.4.5",
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Error creating PFCP connection: %v", err)
	}

	seid := uint64(1)
	conn.AddSession(seid, &engine.Session{})

	err = conn.ModifySession(context.Background(), &models.ModifyRequest{
		SEID: seid,
	})
	if err != nil {
		t.Fatalf("Error modifying session: %v", err)
	}
}
