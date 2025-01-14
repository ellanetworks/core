// Copyright 2024 Ella Networks
// Copyright 2019 Communication Service/Software Laboratory, National Chiao Tung University (free5gc.org)
// SPDX-License-Identifier: Apache-2.0

package fsm

import (
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
)

type (
	EventType string
	ArgsType  map[string]interface{}
)

type (
	Callback  func(*State, EventType, ArgsType)
	Callbacks map[StateType]Callback
)

// Transition defines a transition
// that a Event is triggered at From state,
// and transfer to To state after the Event
type Transition struct {
	Event EventType
	From  StateType
	To    StateType
}

type Transitions []Transition

type eventKey struct {
	Event EventType
	From  StateType
}

// Entry/Exit event are defined by fsm package
const (
	EntryEvent EventType = "Entry event"
	ExitEvent  EventType = "Exit event"
)

type FSM struct {
	// transitions stores one transition for each event
	transitions map[eventKey]Transition
	// callbacks stores one callback function for one state
	callbacks map[StateType]Callback
}

// NewFSM create a new FSM object then registers transitions and callbacks to it
func NewFSM(transitions Transitions, callbacks Callbacks) (*FSM, error) {
	fsm := &FSM{
		transitions: make(map[eventKey]Transition),
		callbacks:   make(map[StateType]Callback),
	}

	allStates := make(map[StateType]bool)

	for _, transition := range transitions {
		key := eventKey{
			Event: transition.Event,
			From:  transition.From,
		}
		if _, ok := fsm.transitions[key]; ok {
			return nil, fmt.Errorf("duplicate transition: %+v", transition)
		} else {
			fsm.transitions[key] = transition
			allStates[transition.From] = true
			allStates[transition.To] = true
		}
	}

	for state, callback := range callbacks {
		if _, ok := allStates[state]; !ok {
			return nil, fmt.Errorf("unknown state: %+v", state)
		} else {
			fsm.callbacks[state] = callback
		}
	}
	return fsm, nil
}

// SendEvent triggers a callback with an event, and do transition after callback if need
// There are 3 types of callback:
//   - on exit callback: call when fsm leave one state, with ExitEvent event
//   - event callback: call when user trigger a user-defined event
//   - on entry callback: call when fsm enter one state, with EntryEvent event
func (fsm *FSM) SendEvent(state *State, event EventType, args ArgsType) error {
	key := eventKey{
		From:  state.Current(),
		Event: event,
	}

	if trans, ok := fsm.transitions[key]; ok {
		logger.UtilLog.Infof("handle event[%s], transition from [%s] to [%s]", event, trans.From, trans.To)

		// event callback
		fsm.callbacks[trans.From](state, event, args)

		// exit callback
		if trans.From != trans.To {
			fsm.callbacks[trans.From](state, ExitEvent, args)
		}

		// entry callback
		if trans.From != trans.To {
			state.Set(trans.To)
			fsm.callbacks[trans.To](state, EntryEvent, args)
		}
		return nil
	} else {
		return fmt.Errorf("unknown transition[From: %s, Event: %s]", state.Current(), event)
	}
}
