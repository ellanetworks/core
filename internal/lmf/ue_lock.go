// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"sync"

	"github.com/ellanetworks/core/etsi"
)

type ueLocks struct {
	mu    sync.Mutex
	locks map[string]*ueLock
}

type ueLock struct {
	sem  chan struct{}
	refs int
}

func (l *ueLocks) acquire(ctx context.Context, supi string) (func(), error) {
	l.mu.Lock()

	if l.locks == nil {
		l.locks = make(map[string]*ueLock)
	}

	lock, ok := l.locks[supi]
	if !ok {
		lock = &ueLock{sem: make(chan struct{}, 1)}
		l.locks[supi] = lock
	}

	lock.refs++
	l.mu.Unlock()

	release := func() {
		l.mu.Lock()
		defer l.mu.Unlock()

		lock.refs--
		if lock.refs == 0 {
			delete(l.locks, supi)
		}
	}

	select {
	case lock.sem <- struct{}{}:
		return func() {
			<-lock.sem
			release()
		}, nil
	case <-ctx.Done():
		release()

		return nil, ctx.Err()
	}
}

func (l *LMF) withUE(ctx context.Context, supi etsi.SUPI, fn func() error) error {
	unlock, err := l.ueLocks.acquire(ctx, supi.String())
	if err != nil {
		return err
	}

	defer unlock()

	return fn()
}
