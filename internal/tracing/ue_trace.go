// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tracing

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/trace"
)

type UETrace struct {
	mu        sync.Mutex
	sessions  *TraceSessions
	session   *TraceSession
	recording bool
	current   RANTraceID
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

func (u *UETrace) StartRecording() (RANTraceID, bool, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.session == nil {
		return RANTraceID{}, false, false
	}

	if u.recording {
		return u.current, false, true
	}

	u.current = u.session.NewRecording()
	u.recording = true

	return u.current, true, true
}

func (u *UETrace) Recording() (RANTraceID, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()

	return u.current, u.recording
}

func (u *UETrace) StopRecording() {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.recording = false
}

func (u *UETrace) Release() {
	u.mu.Lock()
	sessions, sess := u.sessions, u.session
	u.sessions, u.session = nil, nil
	u.recording = false
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

func (c *ConnTrace) Activate(id RANTraceID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.activated {
		c.id = id
	}

	c.activated = true
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

func (c *ConnTrace) Fail() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.activated = false
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
