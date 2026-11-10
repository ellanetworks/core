// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tracing

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/trace"
)

type UETrace struct {
	mu       sync.Mutex
	sessions *TraceSessions
	session  *TraceSession
}

func (u *UETrace) Activate(sessions *TraceSessions, subscriber string, plmn [3]byte) (bool, error) {
	if sessions == nil || subscriber == "" {
		return false, nil
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	if u.session != nil {
		return false, nil
	}

	sess, err := sessions.Acquire(subscriber, plmn)
	if err != nil {
		return false, err
	}

	u.sessions = sessions
	u.session = sess

	return true, nil
}

func (u *UETrace) Active() bool {
	u.mu.Lock()
	defer u.mu.Unlock()

	return u.session != nil
}

func (u *UETrace) NewRecording() (RANTraceID, bool) {
	u.mu.Lock()
	sess := u.session
	u.mu.Unlock()

	if sess == nil {
		return RANTraceID{}, false
	}

	return sess.NewRecording(), true
}

func (u *UETrace) TakeOver(from *UETrace) {
	if from == nil || from == u {
		return
	}

	from.mu.Lock()
	sessions, sess := from.sessions, from.session
	from.sessions, from.session = nil, nil
	from.mu.Unlock()

	if sess == nil {
		return
	}

	u.mu.Lock()

	adopted := u.session == nil
	if adopted {
		u.sessions, u.session = sessions, sess
	}

	u.mu.Unlock()

	if !adopted {
		sessions.Release(sess)
	}
}

func (u *UETrace) Release() {
	u.mu.Lock()
	sessions, sess := u.sessions, u.session
	u.sessions, u.session = nil, nil
	u.mu.Unlock()

	if sessions != nil {
		sessions.Release(sess)
	}
}

type ConnTrace struct {
	mu        sync.Mutex
	id        RANTraceID
	activated bool
}

func (c *ConnTrace) ID() (RANTraceID, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.id, !c.id.IsZero()
}

func (c *ConnTrace) Activated() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.activated
}

func (c *ConnTrace) Assign(id RANTraceID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.id.IsZero() {
		c.id = id
	}
}

func (c *ConnTrace) Pending() (RANTraceID, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.id, !c.id.IsZero() && !c.activated
}

func (c *ConnTrace) MarkActivated(id RANTraceID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.id == id {
		c.activated = true
	}
}

func (c *ConnTrace) Inherit(from *ConnTrace) {
	if from == nil || from == c {
		return
	}

	from.mu.Lock()
	id, activated := from.id, from.activated
	from.mu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.id = id
	c.activated = activated
}

func (c *ConnTrace) Fail(id RANTraceID) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.id.IsZero() || c.id != id {
		return false
	}

	c.activated = false

	return true
}

func (c *ConnTrace) Context(ctx context.Context) context.Context {
	if c == nil {
		return ctx
	}

	id, ok := c.ID()
	if !ok {
		return ctx
	}

	return WithPinnedTraceID(ctx, id)
}

func ContinueRANTrace(ctx context.Context, link trace.SpanContext) context.Context {
	if !link.IsValid() || !IsRANTraceID(link.TraceID()) {
		return ctx
	}

	return context.WithValue(ctx, pinnedTraceIDKey{}, link.TraceID())
}

func IsRANTraceID(id trace.TraceID) bool {
	return id.IsValid() && [8]byte(id[:8]) == [8]byte{}
}
