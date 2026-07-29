// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"errors"
	"net"
	"time"

	"go.uber.org/zap"
)

const (
	// A healthy peer drains in microseconds; a backlog this deep means it has
	// stopped reading.
	writeQueueDepth = 256
)

// Shortened by tests to exercise the deadline path.
var writeTimeout = 5 * time.Second

// Bounds on a graceful close, so a stalled peer cannot hold up shutdown.
var (
	flushTimeout = 2 * time.Second
	drainTimeout = 500 * time.Millisecond
)

// ErrWriteQueueFull reports that the peer stopped draining and the association
// has been aborted.
var ErrWriteQueueFull = errors.New("sctp: outbound queue full")

type queuedWrite struct {
	b    []byte
	info *SndRcvInfo
}

func (c *SCTPConn) startWriter(logger *zap.Logger) {
	c.writeLogger = logger
	c.writeCh = make(chan queuedWrite, writeQueueDepth)
	c.writerExited = make(chan struct{})

	go c.writeLoop()
}

// flushWriter lets the writer send what is already queued, then exit. Bounded:
// a peer that stops draining must not hold up a close.
func (c *SCTPConn) flushWriter() {
	if c.writerExited == nil {
		close(c.writerFlush)
		return
	}

	close(c.writerFlush)

	select {
	case <-c.writerExited:
	case <-time.After(flushTimeout):
	}
}

func (c *SCTPConn) stopWriter() {
	c.writerStop.Do(func() { close(c.writerDone) })
}

func (c *SCTPConn) writeLoop() {
	defer close(c.writerExited)

	for {
		// Go selects randomly among ready cases; check for close first.
		select {
		case <-c.writerDone:
			return
		default:
		}

		select {
		case <-c.writerDone:
			return
		case qw := <-c.writeCh:
			if !c.sendQueued(qw) {
				return
			}
		case <-c.writerFlush:
			for {
				select {
				case qw := <-c.writeCh:
					if !c.sendQueued(qw) {
						return
					}
				default:
					return
				}
			}
		}
	}
}

// sendQueued reports whether the writer may continue.
func (c *SCTPConn) sendQueued(qw queuedWrite) bool {
	if err := c.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		c.failAssociation("set write deadline", err)
		return false
	}

	if _, err := c.writeMsgSync(qw.b, qw.info); err != nil {
		c.failAssociation("send", err)
		return false
	}

	return true
}

// WriteMsg queues b for the association's writer goroutine: a nil error means
// queued, not sent. Conns without a writer send synchronously.
func (c *SCTPConn) WriteMsg(b []byte, info *SndRcvInfo) (int, error) {
	if c.writeCh == nil {
		return c.writeMsgSync(b, info)
	}

	if c.closed.Load() {
		return 0, net.ErrClosed
	}

	qw := queuedWrite{b: append([]byte(nil), b...)}

	if info != nil {
		// Copied so a caller reusing its struct cannot mutate a queued write.
		cp := *info
		qw.info = &cp
	}

	// Checked before the enqueue, not alongside it: a select with both cases
	// ready picks at random and would report success on a closed association.
	select {
	case <-c.writerDone:
		return 0, net.ErrClosed
	default:
	}

	select {
	case c.writeCh <- qw:
		return len(b), nil
	default:
		c.failAssociation("queue full", ErrWriteQueueFull)

		return 0, ErrWriteQueueFull
	}
}

// failAssociation aborts the conn, ending the read loop so the server runs its
// disconnect cleanup. It runs on the writer goroutine, so it must not wait for
// the writer to finish.
func (c *SCTPConn) failAssociation(op string, err error) {
	if c.writeLogger != nil {
		c.writeLogger.Warn("SCTP write failed; aborting association", zap.String("op", op), zap.Error(err))
	}

	_ = c.Abort()
}

func (c *SCTPConn) awaitWriter() {
	if c.writerExited == nil {
		return
	}

	_ = c.Close()
	<-c.writerExited
}
