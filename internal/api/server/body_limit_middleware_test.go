// Copyright 2026 Ella Networks

package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaxBodySizeMiddleware_AllowsSmallBody(t *testing.T) {
	handler := MaxBodySizeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))

	body := strings.NewReader(`{"name": "test"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/subscribers", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMaxBodySizeMiddleware_RejectsOversizedBody(t *testing.T) {
	handler := MaxBodySizeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))

	// DefaultMaxBodySize is 1 MB; send 2 MB
	oversized := strings.NewReader(strings.Repeat("x", 2<<20))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/subscribers", oversized)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
}

func TestMaxBodySizeMiddleware_RestoreAllowsLargeBody(t *testing.T) {
	handler := MaxBodySizeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))

	// Send 2 MB to the restore endpoint — should be allowed (no limit on restore)
	largeBody := strings.NewReader(strings.Repeat("x", 2<<20))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/restore", largeBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for restore with large body, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMaxBodySizeMiddleware_GETRequestUnaffected(t *testing.T) {
	handler := MaxBodySizeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/subscribers", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
