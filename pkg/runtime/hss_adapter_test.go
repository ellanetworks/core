// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/hss"
)

func TestHSSErrorMarksOnlyTransientRaftErrorsUnavailable(t *testing.T) {
	cases := map[error]bool{
		fmt.Errorf("couldn't advance the IMS sequence number: %w", db.ErrOutcomeUnknown): true,
		fmt.Errorf("cas: %w", db.ErrProposeTimeout):                                      true,
		fmt.Errorf("couldn't advance the IMS sequence number: %w", db.ErrNotFound):       false,
		errors.New("disk I/O error"):                                                     false,
	}

	for err, want := range cases {
		got := hssError(err)

		if errors.Is(got, hss.ErrUnavailable) != want || !errors.Is(got, err) {
			t.Errorf("hssError(%v) = %v, want unavailable %v and the original error kept", err, got, want)
		}
	}

	if hssError(nil) != nil {
		t.Error("hssError(nil) is not nil")
	}
}

func TestIMSRegistrationStatesMapExplicitly(t *testing.T) {
	pairs := map[hss.State]db.IMSRegistrationState{
		hss.NotRegistered: db.IMSNotRegistered,
		hss.Registered:    db.IMSRegistered,
		hss.Unregistered:  db.IMSUnregistered,
	}

	for hssState, dbState := range pairs {
		stored, err := dbIMSRegistration("001010000000001", &hss.Registration{State: hssState, ServerName: "sip:scscf.example.org"})
		if err != nil || stored.State != dbState {
			t.Fatalf("dbIMSRegistration(%d) = %+v, %v; want state %d", hssState, stored, err, dbState)
		}

		loaded, err := hssRegistration(stored)
		if err != nil || loaded.State != hssState {
			t.Fatalf("hssRegistration(%d) = %+v, %v; want state %d", dbState, loaded, err, hssState)
		}
	}

	if got, err := dbIMSRegistration("001010000000001", &hss.Registration{State: hss.Unregistered + 1}); err == nil {
		t.Fatalf("dbIMSRegistration of an unknown state = %+v, want an error", got)
	}

	if got, err := hssRegistration(&db.IMSRegistration{State: db.IMSUnregistered + 1}); err == nil {
		t.Fatalf("hssRegistration of an unknown state = %+v, want an error", got)
	}
}
