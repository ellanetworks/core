// Copyright 2024 Ella Networks
// Copyright 2019 Communication Service/Software Laboratory, National Chiao Tung University (free5gc.org)
// SPDX-License-Identifier: Apache-2.0

package fsm_test

import (
	"fmt"
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

func TestFSM(t *testing.T) {
	f, err := fsm.NewFSM(fsm.Transitions{
		{Event: Open, From: Closed, To: Opened},
		{Event: Close, From: Opened, To: Closed},
		{Event: Open, From: Opened, To: Opened},
		{Event: Close, From: Closed, To: Closed},
	}, fsm.Callbacks{
		Opened: func(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
			fmt.Printf("event [%+v] at state [%+v]\n", event, state.Current())
		},
		Closed: func(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
			fmt.Printf("event [%+v] at state [%+v]\n", event, state.Current())
		},
	})

	s := fsm.NewState(Closed)

	if err != nil {
		t.Errorf("NewFSM() failed")
	}

	err = f.SendEvent(s, Open, fsm.ArgsType{"TestArg": "test arg"})
	if err != nil {
		t.Errorf("SendEvent() failed")
	}

	err = f.SendEvent(s, Close, fsm.ArgsType{"TestArg": "test arg"})
	if err != nil {
		t.Errorf("SendEvent() failed")
	}

	if !s.Is(Closed) {
		t.Errorf("Transition failed")
	}

	fakeEvent := fsm.EventType("fake event")

	err = f.SendEvent(s, fakeEvent, nil)
	if err == nil {
		t.Errorf("SendEvent() failed")
	}
}

func TestFSMInitFail(t *testing.T) {
	duplicateTrans := fsm.Transition{
		Event: Close, From: Opened, To: Closed,
	}
	_, err := fsm.NewFSM(fsm.Transitions{
		{Event: Open, From: Closed, To: Opened},
		duplicateTrans,
		duplicateTrans,
		{Event: Open, From: Opened, To: Opened},
		{Event: Close, From: Closed, To: Closed},
	}, fsm.Callbacks{
		Opened: func(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
			fmt.Printf("event [%+v] at state [%+v]\n", event, state.Current())
		},
		Closed: func(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
			fmt.Printf("event [%+v] at state [%+v]\n", event, state.Current())
		},
	})

	if err == nil {
		t.Errorf("NewFSM() failed")
	}

	fakeState := fsm.StateType("fake state")

	_, err = fsm.NewFSM(fsm.Transitions{
		{Event: Open, From: Closed, To: Opened},
		{Event: Close, From: Opened, To: Closed},
		{Event: Open, From: Opened, To: Opened},
		{Event: Close, From: Closed, To: Closed},
	}, fsm.Callbacks{
		Opened: func(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
			fmt.Printf("event [%+v] at state [%+v]\n", event, state.Current())
		},
		Closed: func(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
			fmt.Printf("event [%+v] at state [%+v]\n", event, state.Current())
		},
		fakeState: func(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
			fmt.Printf("event [%+v] at state [%+v]\n", event, state.Current())
		},
	})

	if err == nil {
		t.Errorf("NewFSM() failed")
	}
}
