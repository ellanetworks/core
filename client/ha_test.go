// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestHAClientResendsTheBodyToTheNextNode(t *testing.T) {
	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()

	var received []string

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		received = append(received, string(b))

		_, _ = w.Write([]byte(`{"result":{}}`))
	}))
	defer up.Close()

	c, err := client.New(&client.Config{BaseURLs: []string{down.URL, up.URL}})
	if err != nil {
		t.Fatal(err)
	}

	body := bytes.NewBufferString(`{"name":"imspolicy"}`)

	if _, err := c.Requester.Do(context.Background(), &client.RequestOptions{Type: client.SyncRequest, Method: http.MethodPost, Path: "api/v1/policies", Body: body}); err != nil {
		t.Fatalf("request: %v", err)
	}

	if len(received) != 1 || received[0] != `{"name":"imspolicy"}` {
		t.Fatalf("the next node received %q", received)
	}
}

func TestHAClientDoesNotRetryAStreamedBody(t *testing.T) {
	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()

	calls := 0

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++

		_, _ = w.Write([]byte(`{"result":{}}`))
	}))
	defer up.Close()

	c, err := client.New(&client.Config{BaseURLs: []string{down.URL, up.URL}})
	if err != nil {
		t.Fatal(err)
	}

	pr, pw := io.Pipe()

	go func() {
		_, _ = pw.Write([]byte("backup"))
		_ = pw.Close()
	}()

	if _, err := c.Requester.Do(context.Background(), &client.RequestOptions{Type: client.RawRequest, Method: http.MethodPost, Path: "api/v1/restore", Body: pr}); err == nil {
		t.Fatal("a streamed body was sent again to another node")
	}

	if calls != 0 {
		t.Fatalf("the next node got %d requests", calls)
	}
}
