package client_test

import (
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestNew_HAClient(t *testing.T) {
	c, err := client.New(&client.Config{
		BaseURLs: []string{
			"https://node1:5002",
			"https://node2:5002",
			"https://node3:5002",
		},
		APIToken: "ellacore_test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Requester == nil {
		t.Error("expected HA requester to be set")
	}

	if c.GetToken() != "ellacore_test" {
		t.Errorf("expected token ellacore_test, got %s", c.GetToken())
	}
}

func TestNew_HAClient_EmptyURLs(t *testing.T) {
	_, err := client.New(&client.Config{
		BaseURLs: []string{},
	})
	if err == nil {
		t.Fatal("expected error for empty BaseURLs")
	}
}

func TestNew_HAClient_InvalidURL(t *testing.T) {
	_, err := client.New(&client.Config{
		BaseURLs: []string{"://invalid"},
	})
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}
