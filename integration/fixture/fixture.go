// Package fixture provides per-subtest builders over the Ella Core client
// SDK for provisioning scenario fixtures (operator, profiles, slices,
// data networks, policies, subscribers).
package fixture

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// F is a fixture builder bound to one subtest. Methods fail the test via
// t.Fatalf on API error.
type F struct {
	t   *testing.T
	c   *client.Client
	ctx context.Context
}

func New(t *testing.T, ctx context.Context, c *client.Client) *F {
	t.Helper()

	return &F{
		t:   t,
		c:   c,
		ctx: ctx,
	}
}

func (f *F) fatalf(format string, a ...any) {
	f.t.Helper()
	f.t.Fatalf(format, a...)
}
