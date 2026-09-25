// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
)

func TestPermanentPolicyFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"no matching policy", ErrNoPolicyMatch, true},
		{"data network gone", ErrDNNNotFound, true},
		{"data network unbound from slice", ErrDNNNotInSlice, true},
		{"wrapped", fmt.Errorf("get session policy: %w", ErrDNNNotInSlice), true},
		{"transient infrastructure error", errors.New("raft: propose timeout"), false},
		{"nil", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := permanentPolicyFailure(tc.err); got != tc.want {
				t.Errorf("permanentPolicyFailure(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

type signalingPCF struct {
	lookups chan struct{}
}

func (p *signalingPCF) GetSessionPolicy(context.Context, string, *models.Snssai, string) (*Policy, error) {
	p.lookups <- struct{}{}

	return nil, errors.New("unavailable")
}

func (p *signalingPCF) GetEPSSessionPolicy(context.Context, string, string) (*Policy, *models.Snssai, error) {
	p.lookups <- struct{}{}

	return nil, nil, errors.New("unavailable")
}

func TestSessionReconcilerReconcilesOnWakeup(t *testing.T) {
	pcf := &signalingPCF{lookups: make(chan struct{}, 2)}
	s := New(pcf, nil, nil, nil)

	supi, err := etsi.NewSUPIFromPrefixed("imsi-001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.NewSession(supi, Access5G, SessionIdentity{PDUSessionID: 1}, "internet", &models.Snssai{Sst: 1}); err != nil {
		t.Fatal(err)
	}

	wakeup := make(chan struct{})

	r := NewSessionReconciler(s, wakeup)
	r.backstop = time.Hour

	r.Start()
	defer r.Stop()

	select {
	case <-pcf.lookups:
	case <-time.After(2 * time.Second):
		t.Fatal("the initial reconcile never ran")
	}

	wakeup <- struct{}{}

	select {
	case <-pcf.lookups:
	case <-time.After(2 * time.Second):
		t.Fatal("a wakeup did not trigger a reconcile")
	}
}
