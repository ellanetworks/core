// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package joinreq

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrNotAccepting   = errors.New("this node is not waiting to join a cluster")
	ErrJoinInProgress = errors.New("a join attempt is already running on this node")
)

type State string

const (
	StateUnavailable State = "unavailable"
	StateWaiting     State = "waiting"
	StateJoining     State = "joining"
	StateJoined      State = "joined"
)

var ErrUnknownMode = errors.New("mode must be \"join\" or \"bootstrap\"")

type Mode string

const (
	ModeJoin      Mode = "join"
	ModeBootstrap Mode = "bootstrap"
)

type Request struct {
	Mode          Mode
	Token         string
	SeedAddresses []string
	Suffrage      string
}

type Status struct {
	State State
	Error string
}

type Coordinator struct {
	mu        sync.Mutex
	accepting bool
	state     State
	lastErr   string

	pending chan Request
}

func New() *Coordinator {
	return &Coordinator{state: StateUnavailable, pending: make(chan Request, 1)}
}

var defaultCoordinator = New()

func Default() *Coordinator { return defaultCoordinator }

func (c *Coordinator) Open() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.accepting = true
	c.state = StateWaiting
	c.lastErr = ""
}

func (c *Coordinator) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.accepting = false

	if c.state != StateJoined {
		c.state = StateUnavailable
	}
}

func (c *Coordinator) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()

	return Status{State: c.state, Error: c.lastErr}
}

func (c *Coordinator) Submit(req Request) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.accepting {
		return ErrNotAccepting
	}

	if req.Mode != ModeJoin && req.Mode != ModeBootstrap {
		return ErrUnknownMode
	}

	if c.state == StateJoining {
		return ErrJoinInProgress
	}

	select {
	case c.pending <- req:
		c.state = StateJoining
		c.lastErr = ""

		return nil
	default:
		return ErrJoinInProgress
	}
}

func (c *Coordinator) Await(ctx context.Context) (Request, error) {
	select {
	case req := <-c.pending:
		return req, nil
	case <-ctx.Done():
		return Request{}, ctx.Err()
	}
}

func (c *Coordinator) Report(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err != nil {
		c.state = StateWaiting
		c.lastErr = err.Error()

		return
	}

	c.state = StateJoined
	c.lastErr = ""
}
