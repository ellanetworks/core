// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"context"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/roaming/wire"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestNotOperationalWarnsOnceWhenFirstPartnerAdded(t *testing.T) {
	var (
		mu       sync.Mutex
		partners []PartnerConfig
	)

	core, logs := observer.New(zapcore.DebugLevel)

	m := New(Config{
		Partners: func(context.Context) ([]PartnerConfig, error) {
			mu.Lock()
			defer mu.Unlock()

			return partners, nil
		},
		Identity: func(context.Context) (*IdentityConfig, error) { return nil, nil },
		Operator: func(context.Context) (wire.PlmnID, error) { return xPLMN, nil },
	}, zap.New(core))

	t.Cleanup(m.stopListener)

	count := func(level zapcore.Level) int {
		return logs.FilterMessage("SEPP is not operational on this node").FilterLevelExact(level).Len()
	}

	m.Reconcile(context.Background())
	m.Reconcile(context.Background())

	if debug, warn := count(zapcore.DebugLevel), count(zapcore.WarnLevel); debug != 1 || warn != 0 {
		t.Fatalf("without partners: debug = %d, warn = %d, want 1, 0", debug, warn)
	}

	mu.Lock()
	partners = []PartnerConfig{{
		ID: "p", Name: "p", PLMNs: []wire.PlmnID{yPLMN}, FQDN: yFQDN, N32cURI: "https://" + yFQDN,
		TrustAnchorPEM: newTestCA(t, "partner").pem(),
	}}
	mu.Unlock()

	m.Reconcile(context.Background())
	m.Reconcile(context.Background())

	if debug, warn := count(zapcore.DebugLevel), count(zapcore.WarnLevel); debug != 1 || warn != 1 {
		t.Fatalf("after adding a partner: debug = %d, warn = %d, want 1, 1", debug, warn)
	}

	if got := m.Problem(); got != errNoN32 {
		t.Fatalf("problem = %q, want %q", got, errNoN32)
	}
}
