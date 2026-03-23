package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthorize_MissingRoleReturns403(t *testing.T) {
	handler := Authorize(PermListSubscribers, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called when role is missing")
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/subscribers", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestAuthorize_AdminAllowed(t *testing.T) {
	called := false

	handler := Authorize(PermListSubscribers, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true

		w.WriteHeader(http.StatusOK)
	}))

	ctx := context.WithValue(context.Background(), contextKeyRoleID, RoleAdmin)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/subscribers", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("next handler should have been called for admin role")
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestAuthorize_ReadOnlyDeniedWrite(t *testing.T) {
	handler := Authorize(PermCreateSubscriber, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called for unauthorized permission")
	}))

	ctx := context.WithValue(context.Background(), contextKeyRoleID, RoleReadOnly)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/subscribers", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}
