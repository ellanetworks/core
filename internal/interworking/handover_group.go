// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"context"
	"sync"
)

type HandoverGroup struct {
	mu      sync.Mutex
	running int
	quiet   chan struct{}
}

func (g *HandoverGroup) Go(complete func()) {
	g.mu.Lock()

	if g.running == 0 && g.quiet != nil {
		g.quiet = make(chan struct{})
	}

	g.running++

	g.mu.Unlock()

	go func() {
		defer g.settle()

		complete()
	}()
}

func (g *HandoverGroup) Await(ctx context.Context) error {
	g.mu.Lock()
	quiet := g.quietLocked()
	g.mu.Unlock()

	select {
	case <-quiet:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *HandoverGroup) quietLocked() chan struct{} {
	if g.quiet == nil {
		g.quiet = make(chan struct{})

		if g.running == 0 {
			close(g.quiet)
		}
	}

	return g.quiet
}

func (g *HandoverGroup) settle() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.running--

	if g.running == 0 && g.quiet != nil {
		close(g.quiet)
	}
}
