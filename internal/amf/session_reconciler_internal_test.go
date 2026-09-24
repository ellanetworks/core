// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/smf"
)

func TestPermanentPolicyFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"no matching policy", smf.ErrNoPolicyMatch, true},
		{"data network gone", smf.ErrDNNNotFound, true},
		{"data network unbound from slice", smf.ErrDNNNotInSlice, true},
		{"wrapped", fmt.Errorf("get session policy: %w", smf.ErrDNNNotInSlice), true},
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

type signalingLookupSmf struct {
	*deregisterTestSmf

	lookups chan struct{}
}

func (s *signalingLookupSmf) GetSession(string) *smf.SMContext {
	s.lookups <- struct{}{}

	return nil
}

func TestSessionReconcilerReconcilesOnWakeup(t *testing.T) {
	a, ue, _, _ := registeredUE(t)
	ue.SmContextList[1] = &SmContext{Ref: "ref-1"}

	fakeSmf := &signalingLookupSmf{deregisterTestSmf: &deregisterTestSmf{}, lookups: make(chan struct{}, 2)}
	a.Session = fakeSmf

	wakeup := make(chan struct{})

	r := NewSessionReconciler(a, wakeup)
	r.backstop = time.Hour

	r.Start()
	defer r.Stop()

	select {
	case <-fakeSmf.lookups:
	case <-time.After(2 * time.Second):
		t.Fatal("the initial reconcile never ran")
	}

	wakeup <- struct{}{}

	select {
	case <-fakeSmf.lookups:
	case <-time.After(2 * time.Second):
		t.Fatal("a wakeup did not trigger a reconcile")
	}
}
