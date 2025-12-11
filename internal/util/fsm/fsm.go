// Copyright 2024 Ella Networks
// Copyright 2019 Communication Service/Software Laboratory, National Chiao Tung University (free5gc.org)
// SPDX-License-Identifier: Apache-2.0

package fsm

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("ella-core/fsm")

type (
	EventType string
	ArgsType  map[string]any
)

type (
	Callback  func(context.Context, *State, EventType, ArgsType)
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
func (fsm *FSM) SendEvent(ctx context.Context, state *State, event EventType, args ArgsType) error {
	ctx, span := tracer.Start(ctx, "FSM SendEvent",
		trace.WithAttributes(
			attribute.String("fsm.event", string(event)),
			attribute.String("fsm.state.current", string(state.Current())),
		),
	)
	defer span.End()

	key := eventKey{
		From:  state.Current(),
		Event: event,
	}

	if trans, ok := fsm.transitions[key]; ok {
		// event callback
		ctx, span := tracer.Start(ctx, "FSM Event",
			trace.WithAttributes(
				attribute.String("fsm.event", string(event)),
				attribute.String("fsm.state_type.from", string(trans.From)),
				attribute.String("fsm.state_type.to", string(trans.To)),
				attribute.String("fsm.state.current", string(state.Current())),
			),
		)
		fsm.callbacks[trans.From](ctx, state, event, args)
		span.End()

		// exit callback
		if trans.From != trans.To {
			ctx, span := tracer.Start(ctx, "FSM Exit Event",
				trace.WithAttributes(
					attribute.String("fsm.event", string(ExitEvent)),
					attribute.String("fsm.state_type.from", string(trans.From)),
					attribute.String("fsm.state_type.to", string(trans.To)),
					attribute.String("fsm.state.current", string(state.Current())),
				),
			)
			defer fsm.callbacks[trans.From](ctx, state, ExitEvent, args)
			span.End()
		}

		// entry callback
		if trans.From != trans.To {
			ctx, span := tracer.Start(ctx, "FSM Entry Event",
				trace.WithAttributes(
					attribute.String("fsm.event", string(EntryEvent)),
					attribute.String("fsm.state_type.from", string(trans.From)),
					attribute.String("fsm.state_type.to", string(trans.To)),
					attribute.String("fsm.state.initial_state", string(state.Current())),
				),
			)
			state.Set(trans.To)
			span.SetAttributes(
				attribute.String("fsm.state.current", string(state.Current())),
			)
			fsm.callbacks[trans.To](ctx, state, EntryEvent, args)
			span.End()
		}
		return nil
	} else {
		return fmt.Errorf("unknown transition[From: %s, Event: %s]", state.Current(), event)
	}
}
