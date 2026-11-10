// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tracing

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"net/netip"
	"sync"

	"go.opentelemetry.io/otel/trace"
)

const (
	traceIDValueBits = 18
	traceIDValueMask = 1<<traceIDValueBits - 1
	traceIDPrefixMax = 1<<(24-traceIDValueBits) - 1
)

var ErrTraceIDsExhausted = errors.New("tracing: no free Trace ID")

type RANTraceID [8]byte

func (id RANTraceID) OTelTraceID() trace.TraceID {
	var t trace.TraceID

	copy(t[8:], id[:])

	return t
}

func (id RANTraceID) IsZero() bool {
	return id == RANTraceID{}
}

type TraceConfig struct {
	TCEAddress netip.Addr
}

type TraceSessions struct {
	cfg    TraceConfig
	prefix func() int

	mu    sync.Mutex
	next  uint32
	inUse map[uint32]struct{}
	bySub map[string]*TraceSession
}

func NewTraceSessions(cfg TraceConfig, prefix func() int) *TraceSessions {
	return &TraceSessions{
		cfg:    cfg,
		prefix: prefix,
		next:   randomUint32() & traceIDValueMask,
		inUse:  make(map[uint32]struct{}),
		bySub:  make(map[string]*TraceSession),
	}
}

func (s *TraceSessions) Config() TraceConfig {
	return s.cfg
}

func (s *TraceSessions) Acquire(subscriber string, plmn [3]byte) (*TraceSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sess, ok := s.bySub[subscriber]; ok {
		sess.refs++

		return sess, nil
	}

	value, ok := s.allocateLocked()
	if !ok {
		return nil, ErrTraceIDsExhausted
	}

	traceID := uint32(s.prefix()&traceIDPrefixMax)<<traceIDValueBits | value

	sess := &TraceSession{
		subscriber: subscriber,
		value:      value,
		refs:       1,
		trsr:       uint16(randomUint32()),
	}

	copy(sess.traceReference[:3], plmn[:])
	sess.traceReference[3] = byte(traceID >> 16)
	sess.traceReference[4] = byte(traceID >> 8)
	sess.traceReference[5] = byte(traceID)

	s.inUse[value] = struct{}{}
	s.bySub[subscriber] = sess

	return sess, nil
}

func (s *TraceSessions) Release(sess *TraceSession) {
	if sess == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.bySub[sess.subscriber] != sess {
		return
	}

	sess.refs--
	if sess.refs > 0 {
		return
	}

	delete(s.bySub, sess.subscriber)
	delete(s.inUse, sess.value)
}

func (s *TraceSessions) Active() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.bySub)
}

func (s *TraceSessions) allocateLocked() (uint32, bool) {
	for range traceIDValueMask + 1 {
		v := s.next
		s.next = (s.next + 1) & traceIDValueMask

		if _, used := s.inUse[v]; !used {
			return v, true
		}
	}

	return 0, false
}

type TraceSession struct {
	subscriber     string
	value          uint32
	traceReference [6]byte
	refs           int

	mu   sync.Mutex
	trsr uint16
}

func (t *TraceSession) TraceReference() [6]byte {
	return t.traceReference
}

func (t *TraceSession) NewRecording() RANTraceID {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.trsr++

	var id RANTraceID

	copy(id[:6], t.traceReference[:])
	binary.BigEndian.PutUint16(id[6:], t.trsr)

	return id
}

func randomUint32() uint32 {
	var b [4]byte

	_, _ = rand.Read(b[:])

	return binary.BigEndian.Uint32(b[:])
}
