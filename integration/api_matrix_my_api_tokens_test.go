// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runMyAPITokensMatrix exercises the self-scoped API-token endpoints. It
// operates only on a uniquely named token it creates, so the bootstrap token
// the test client authenticates with is never deleted.
func runMyAPITokensMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	const tokenName = "apimat-my-token"

	listAll := func() *client.ListAPITokensResponse {
		resp, err := c.ListMyAPITokens(ctx, &client.ListParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list my api tokens: %v", err)
		}

		return resp
	}

	findByName := func(items []client.APIToken, name string) *client.APIToken {
		for i := range items {
			if items[i].Name == name {
				return &items[i]
			}
		}

		return nil
	}

	baseline := listAll()

	if _, err := c.CreateMyAPIToken(ctx, &client.CreateAPITokenOptions{Name: tokenName}); err != nil {
		t.Fatalf("create my api token: %v", err)
	}

	afterCreate := listAll()

	created := findByName(afterCreate.Items, tokenName)
	if created == nil {
		t.Fatalf("list after create missing token name %q", tokenName)
	}

	deleted := false

	t.Cleanup(func() {
		if deleted {
			return
		}

		if err := c.DeleteMyAPIToken(ctx, created.ID); err != nil {
			t.Logf("cleanup: delete my api token %q: %v", created.ID, err)
		}
	})

	if afterCreate.TotalCount != baseline.TotalCount+1 {
		t.Fatalf("list count after create: got %d, want %d", afterCreate.TotalCount, baseline.TotalCount+1)
	}

	if err := c.DeleteMyAPIToken(ctx, created.ID); err != nil {
		t.Fatalf("delete my api token %q: %v", created.ID, err)
	}

	deleted = true

	afterDelete := listAll()
	if afterDelete.TotalCount != baseline.TotalCount {
		t.Fatalf("list count after delete: got %d, want %d", afterDelete.TotalCount, baseline.TotalCount)
	}

	if findByName(afterDelete.Items, tokenName) != nil {
		t.Fatalf("list after delete still contains token name %q", tokenName)
	}
}
