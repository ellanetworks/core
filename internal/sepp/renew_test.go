// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"context"
	"crypto/x509"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"go.uber.org/zap"
)

type renewFixture struct {
	mu       sync.Mutex
	now      time.Time
	ca       testCA
	identity IdentityConfig
	calls    []int
	fail     bool
	m        *Manager
}

func newRenewFixture(t *testing.T, caNotAfter time.Time, leafNotAfter time.Time, generated bool) *renewFixture {
	t.Helper()

	f := &renewFixture{now: time.Now(), ca: newTestCAUntil(t, "cluster CA", caNotAfter)}
	f.identity = identityFor(t, f.ca, xFQDN, leafOpts{
		notBefore: time.Now().Add(-time.Hour),
		notAfter:  leafNotAfter,
		eku:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	f.identity.Generated = generated

	f.m = New(Config{
		Partners: func(context.Context) ([]PartnerConfig, error) { return nil, nil },
		Identity: func(context.Context) (*IdentityConfig, error) {
			f.mu.Lock()
			defer f.mu.Unlock()

			id := f.identity

			return &id, nil
		},
		Operator: func(context.Context) (wire.PlmnID, error) { return xPLMN, nil },
		Renew: func(_ context.Context, seppID int) error {
			f.mu.Lock()
			defer f.mu.Unlock()

			f.calls = append(f.calls, seppID)
			if f.fail {
				return errors.New("renewal failed")
			}

			renewed := identityFor(t, f.ca, xFQDN, leafOpts{
				notBefore: time.Now().Add(-time.Hour),
				notAfter:  caNotAfter,
				eku:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			})
			renewed.Generated = true
			f.identity = renewed

			return nil
		},
		N32: config.N32Interface{Enabled: true, Address: "127.0.0.1"},
	}, zap.NewNop(), WithClock(func() time.Time {
		f.mu.Lock()
		defer f.mu.Unlock()

		return f.now
	}))

	t.Cleanup(f.m.stopListener)

	return f
}

func (f *renewFixture) renewals() []int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]int(nil), f.calls...)
}

func (f *renewFixture) advance(d time.Duration) {
	f.mu.Lock()
	f.now = f.now.Add(d)
	f.mu.Unlock()
}

func TestRenewGeneratedCertificateBeforeExpiry(t *testing.T) {
	f := newRenewFixture(t, time.Now().AddDate(5, 0, 0), time.Now().AddDate(0, 0, 60), true)

	f.m.Reconcile(context.Background())

	if got := f.renewals(); len(got) != 1 || got[0] != 1 {
		t.Fatalf("renewals = %v, want [1]", got)
	}

	if na := f.m.snap.Load().identity.leaf.NotAfter; na.Before(time.Now().AddDate(4, 0, 0)) {
		t.Fatalf("serving a certificate that expires %s after renewal", na)
	}

	f.m.Reconcile(context.Background())

	if got := f.renewals(); len(got) != 1 {
		t.Fatalf("renewed again: %v", got)
	}
}

func TestRenewSkipped(t *testing.T) {
	for name, tc := range map[string]struct {
		caNotAfter, leafNotAfter time.Time
		generated                bool
	}{
		"imported certificate":      {time.Now().AddDate(5, 0, 0), time.Now().AddDate(0, 0, 60), false},
		"far from expiry":           {time.Now().AddDate(5, 0, 0), time.Now().AddDate(0, 0, 120), true},
		"CA expires within 90 days": {time.Now().AddDate(0, 0, 80), time.Now().AddDate(0, 0, 60), true},
		"CA expires with the leaf":  {time.Now().AddDate(0, 0, 60), time.Now().AddDate(0, 0, 60), true},
	} {
		f := newRenewFixture(t, tc.caNotAfter, tc.leafNotAfter, tc.generated)

		f.m.Reconcile(context.Background())

		if got := f.renewals(); len(got) != 0 {
			t.Errorf("%s: renewals = %v, want none", name, got)
		}
	}
}

func TestRenewRetriesAfterFailure(t *testing.T) {
	f := newRenewFixture(t, time.Now().AddDate(5, 0, 0), time.Now().AddDate(0, 0, 60), true)
	f.fail = true

	f.m.Reconcile(context.Background())
	f.m.Reconcile(context.Background())

	if got := f.renewals(); len(got) != 1 {
		t.Fatalf("renewals before the retry interval = %v, want one", got)
	}

	f.advance(DefaultTuning().RenewRetry)
	f.m.Reconcile(context.Background())

	if got := f.renewals(); len(got) != 2 {
		t.Fatalf("renewals after the retry interval = %v, want two", got)
	}
}
