// Package fixture provides test-only builders over the Ella Core client SDK
// for provisioning scenario fixtures (operator settings, profiles, slices,
// data networks, policies, subscribers).
//
// Fixture builders are designed for Go subtests that share one Ella Core
// instance: each call creates resources with names suffixed by the subtest
// name, so subtests do not collide. Resources are not cleaned up — a fresh
// compose run per CI invocation caps accumulation, and leaving state
// inspectable on failure outweighs the cleanup cost.
package fixture

import (
	"context"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
)

// F is a fixture builder bound to one subtest. Each method fails the test
// via t.Fatalf on API error.
type F struct {
	t      *testing.T
	c      *client.Client
	ctx    context.Context
	suffix string
}

// New creates a fixture builder for t. The unique suffix is derived from
// t.Name() and applied to resource names created via Profile/Slice/etc.
func New(t *testing.T, ctx context.Context, c *client.Client) *F {
	t.Helper()

	return &F{
		t:      t,
		c:      c,
		ctx:    ctx,
		suffix: sanitizeName(t.Name()),
	}
}

// Suffix returns the per-subtest suffix applied to resource names.
func (f *F) Suffix() string {
	return f.suffix
}

// Name returns base with the subtest suffix appended, safe to use as an
// Ella Core resource name.
func (f *F) Name(base string) string {
	if f.suffix == "" {
		return base
	}

	return base + "-" + f.suffix
}

// sanitizeName converts a Go test name (which can contain "/" from t.Run)
// into a string safe to use as part of an Ella Core resource name.
func sanitizeName(s string) string {
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, " ", "-")

	return strings.ToLower(s)
}

// fatalf fails the test with a formatted message.
func (f *F) fatalf(format string, a ...any) {
	f.t.Helper()
	f.t.Fatalf(format, a...)
}
