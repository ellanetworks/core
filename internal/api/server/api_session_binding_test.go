// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAccessTokensEndWithTheirSession(t *testing.T) {
	e := newSSOEnv(t)

	unbound, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id": "someone", "email": FirstUserEmail, "role_id": 1,
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(),
	}).SignedString([]byte("testsecret"))
	if err != nil {
		t.Fatal(err)
	}

	if code, _ := e.do(t, e.admin, unbound, http.MethodGet, "/api/v1/users/me", nil); code != http.StatusUnauthorized {
		t.Fatalf("token without a session = %d, want %d", code, http.StatusUnauthorized)
	}

	tests := map[string]func(t *testing.T, email string){
		"a role change": func(t *testing.T, email string) {
			if code, resp := e.do(t, e.admin, e.token, http.MethodPut, "/api/v1/users/"+email, map[string]any{"role_id": 2}); code != http.StatusOK {
				t.Fatalf("change role = %d %q", code, resp.Error)
			}
		},
		"a password removal": func(t *testing.T, email string) {
			if code, resp := e.do(t, e.admin, e.token, http.MethodDelete, "/api/v1/users/"+email+"/password", nil); code != http.StatusOK {
				t.Fatalf("remove password = %d %q", code, resp.Error)
			}
		},
		"a password change by an admin": func(t *testing.T, email string) {
			if code, resp := e.do(t, e.admin, e.token, http.MethodPut, "/api/v1/users/"+email+"/password", map[string]any{"password": "another password"}); code != http.StatusOK {
				t.Fatalf("change password = %d %q", code, resp.Error)
			}
		},
		"a deletion": func(t *testing.T, email string) {
			if code, resp := e.do(t, e.admin, e.token, http.MethodDelete, "/api/v1/users/"+email, nil); code != http.StatusOK {
				t.Fatalf("delete = %d %q", code, resp.Error)
			}
		},
	}

	n := 0

	for name, end := range tests {
		t.Run(name, func(t *testing.T) {
			n++
			email := "user" + string(rune('a'+n)) + "@example.com"
			client := newTestClient(e.env.Server)

			token, err := createUserAndLogin(e.env.Server.URL, e.token, email, RoleNetworkManager, client)
			if err != nil {
				t.Fatal(err)
			}

			if code, _ := e.do(t, client, token, http.MethodGet, "/api/v1/users/me", nil); code != http.StatusOK {
				t.Fatalf("token before = %d, want %d", code, http.StatusOK)
			}

			end(t, email)

			if code, _ := e.do(t, client, token, http.MethodGet, "/api/v1/users/me", nil); code != http.StatusUnauthorized {
				t.Fatalf("token after %s = %d, want %d", name, code, http.StatusUnauthorized)
			}
		})
	}
}
