// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"testing"

	"github.com/ellanetworks/core/nas/fgs"
)

func TestBuildRegistrationRequestCarriesTheFollowOnRequestItWasGiven(t *testing.T) {
	for _, want := range []bool{true, false} {
		pdu, err := BuildRegistrationRequest(&RegistrationRequestOpts{
			RegistrationType: uint8(fgs.RegistrationTypeInitial),
			FollowOnRequest:  want,
			UESecurity:       goldenUESecurity(),
		})
		if err != nil {
			t.Fatalf("build (follow-on %v): %v", want, err)
		}

		msg, err := fgs.ParseRegistrationRequest(pdu)
		if err != nil {
			t.Fatalf("parse (follow-on %v): %v", want, err)
		}

		if msg.FOR != want {
			t.Errorf("FOR = %v, want %v", msg.FOR, want)
		}
	}
}

// A UE that will establish its default PDU session next has follow-on
// signalling pending. One that asks for its user plane back through the uplink
// data status does not, and neither does one that establishes no session at
// all: the AMF is then free to release the connection (TS 24.501 §5.5.1.2.4).
// The cases below are the ones the reference network's handsets produce.
func TestFollowOnRequestTracksWhatTheUEWillSendNext(t *testing.T) {
	tests := []struct {
		name             string
		noAutoPDUSession bool
		uplinkDataStatus *[16]bool
		want             bool
	}{
		{name: "default PDU session follows", want: true},
		{name: "reactivation asked for through the uplink data status", uplinkDataStatus: &[16]bool{1: true}, want: false},
		{name: "nothing follows", noAutoPDUSession: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &UE{NoAutoPDUSession: tt.noAutoPDUSession}

			if got := u.followOnRequestPending(tt.uplinkDataStatus); got != tt.want {
				t.Errorf("follow-on request = %v, want %v", got, tt.want)
			}
		})
	}
}
