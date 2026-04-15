package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsWriteMethod(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{"GET", false},
		{"HEAD", false},
		{"OPTIONS", false},
		{"POST", true},
		{"PUT", true},
		{"PATCH", true},
		{"DELETE", true},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if got := isWriteMethod(tt.method); got != tt.want {
				t.Errorf("isWriteMethod(%q) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}

func TestLeaderProxyMiddleware_NilDB(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true

		w.WriteHeader(http.StatusOK)
	})

	handler := LeaderProxyMiddleware(nil, next)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/subscribers", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("expected next handler to be called when dbInstance is nil (standalone)")
	}
}
