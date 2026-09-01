// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/nas/fgs"
)

func TestBuildAuthenticationFailureCarriesTheAUTSOnASynchFailure(t *testing.T) {
	auts := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e}

	pdu, err := BuildAuthenticationFailure(&AuthenticationFailureOpts{
		Cause: fgs.GMMCauseSynchFailure,
		AUTS:  auts,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := fgs.ParseAuthenticationFailure(pdu)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.Cause != fgs.GMMCauseSynchFailure {
		t.Errorf("cause = %v, want synch failure", msg.Cause)
	}

	if !bytes.Equal(msg.AUTS, auts) {
		t.Errorf("AUTS = %x, want %x", msg.AUTS, auts)
	}
}

func TestBuildAuthenticationFailureOmitsTheAUTSOnAMACFailure(t *testing.T) {
	pdu, err := BuildAuthenticationFailure(&AuthenticationFailureOpts{Cause: fgs.GMMCauseMACFailure})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := fgs.ParseAuthenticationFailure(pdu)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.Cause != fgs.GMMCauseMACFailure {
		t.Errorf("cause = %v, want MAC failure", msg.Cause)
	}

	if len(msg.AUTS) != 0 {
		t.Errorf("AUTS = %x, want none", msg.AUTS)
	}
}

func TestBuildAuthenticationFailureRequiresTheAUTSToResynchronise(t *testing.T) {
	if _, err := BuildAuthenticationFailure(&AuthenticationFailureOpts{Cause: fgs.GMMCauseSynchFailure}); err == nil {
		t.Error("expected an error: the network has nothing to resynchronise from without the AUTS")
	}
}

func TestAuthenticationFailureCauseMapsTheAUTNChecks(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want fgs.GMMCause
		ok   bool
	}{
		{name: "sequence number out of range", err: ErrSQNOutOfRange, want: fgs.GMMCauseSynchFailure, ok: true},
		{name: "MAC failure", err: ErrMACFailure, want: fgs.GMMCauseMACFailure, ok: true},
		{name: "wrapped", err: fmt.Errorf("derive: %w", ErrSQNOutOfRange), want: fgs.GMMCauseSynchFailure, ok: true},
		{name: "unrelated", err: errors.New("boom"), ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := authenticationFailureCause(tt.err)
			if ok != tt.ok {
				t.Fatalf("reported = %v, want %v", ok, tt.ok)
			}

			if ok && got != tt.want {
				t.Errorf("cause = %v, want %v", got, tt.want)
			}
		})
	}
}
