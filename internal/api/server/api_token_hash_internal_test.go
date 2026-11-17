// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"golang.org/x/crypto/bcrypt"
)

const (
	testTokenSecret = "abcdefghijklmnopqrstuvwx"
	wrongSecret     = "Xbcdefghijklmnopqrstuvwx"
)

func TestAPITokenMatches(t *testing.T) {
	legacy, err := bcrypt.GenerateFromPassword([]byte(testTokenSecret), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}

	sum := sha256.Sum256([]byte(testTokenSecret))
	current := hex.EncodeToString(sum[:])

	cases := []struct {
		name   string
		hash   string
		secret string
		want   bool
	}{
		{"bcrypt", string(legacy), testTokenSecret, true},
		{"bcrypt wrong secret", string(legacy), wrongSecret, false},
		{"sha256", current, testTokenSecret, true},
		{"sha256 wrong secret", current, wrongSecret, false},
		{"sha256 truncated hash", current[:32], testTokenSecret, false},
		{"empty hash", "", testTokenSecret, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := apiTokenMatches(tc.hash, tc.secret); got != tc.want {
				t.Fatalf("apiTokenMatches = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAuthenticateBcryptAndSHA256APITokens(t *testing.T) {
	ctx := t.Context()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("db: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	userID, err := database.CreateUser(ctx, &db.User{Email: "admin@ellanetworks.com", RoleID: db.RoleID(RoleAdmin), HashedPassword: "unused"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	legacy, err := bcrypt.GenerateFromPassword([]byte(testTokenSecret), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}

	current, err := hashAPIToken(ctx, database, testTokenSecret, bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if strings.HasPrefix(current, "$2") || len(current) != sha256.Size*2 {
		t.Fatalf("new token hash = %q, want SHA-256 hex", current)
	}

	for tokenID, hash := range map[string]string{"legacybcrypt": string(legacy), "currentsha256": current} {
		if err := database.CreateAPIToken(ctx, &db.APIToken{TokenID: tokenID, Name: tokenID, UserID: userID, TokenHash: hash}); err != nil {
			t.Fatalf("create token %s: %v", tokenID, err)
		}

		for secret, wantOK := range map[string]bool{testTokenSecret: true, wrongSecret: false} {
			r := httptest.NewRequestWithContext(ctx, "GET", "/api/v1/subscribers", nil)
			r.Header.Set("Authorization", "Bearer ellacore_"+tokenID+"_"+secret)

			gotID, _, _, err := authenticateRequest(r, nil, database)
			if wantOK && (err != nil || gotID != userID) {
				t.Fatalf("%s: authenticate = %q, %v, want %q", tokenID, gotID, err, userID)
			}

			if !wantOK && err == nil {
				t.Fatalf("%s: wrong secret authenticated", tokenID)
			}
		}
	}
}
