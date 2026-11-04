// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"sync"
	"time"
)

const CrossCancelWait = time.Minute

type CrossCancelGuard struct {
	mu      sync.Mutex
	wait    time.Duration
	pending map[string]*crossCancel
}

type crossCancel struct {
	partnerID string
	timer     *time.Timer
}

func (g *CrossCancelGuard) SetWait(d time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.wait = d
}

func (g *CrossCancelGuard) Arm(imsi, partnerID string, purge func()) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.pending == nil {
		g.pending = make(map[string]*crossCancel)
	}

	if old := g.pending[imsi]; old != nil {
		old.timer.Stop()
	}

	wait := g.wait
	if wait <= 0 {
		wait = CrossCancelWait
	}

	c := &crossCancel{partnerID: partnerID}
	c.timer = time.AfterFunc(wait, func() {
		g.mu.Lock()

		current := g.pending[imsi] == c
		if current {
			delete(g.pending, imsi)
		}

		g.mu.Unlock()

		if current {
			purge()
		}
	})
	g.pending[imsi] = c
}

func (g *CrossCancelGuard) Partner(imsi string) (string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	c, ok := g.pending[imsi]
	if !ok {
		return "", false
	}

	return c.partnerID, true
}

func (g *CrossCancelGuard) Disarm(imsi string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	c, ok := g.pending[imsi]
	if !ok {
		return false
	}

	c.timer.Stop()
	delete(g.pending, imsi)

	return true
}
