// Copyright 2024 Ella Networks
// Copyright 2019 Communication Service/Software Laboratory, National Chiao Tung University (free5gc.org)
// SPDX-License-Identifier: Apache-2.0

package fsm_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/util/fsm"
)

const (
	Opened fsm.StateType = "Opened"
	Closed fsm.StateType = "Closed"
)

const (
	Open  fsm.EventType = "Open"
	Close fsm.EventType = "Close"
)

func TestState(t *testing.T) {
	s := fsm.NewState(Closed)

	if s.Current() != Closed {
		t.Errorf("Current() failed")
	}
	if !s.Is(Closed) {
		t.Errorf("Is() failed")
	}

	s.Set(Opened)

	if s.Current() != Opened {
		t.Errorf("Set() failed")
	}
	if !s.Is(Opened) {
		t.Errorf("Is() failed")
	}
}
