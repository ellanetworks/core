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

	writeTimeout = 5 * time.Second
)

var errWriteQueueFull = errors.New("sctp: outbound queue full")

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
			if err := c.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
				c.failAssociation("set write deadline", err)
				return
			}

			if _, err := c.writeMsgSync(qw.b, qw.info); err != nil {
				c.failAssociation("send", err)
				return
			}
		}
	}
}

// WriteMsg queues b for the association's writer goroutine: a nil error means
// queued, not sent. Conns without a writer send synchronously.
func (c *SCTPConn) WriteMsg(b []byte, info *SndRcvInfo) (int, error) {
	if c.writeCh == nil {
		return c.writeMsgSync(b, info)
	}

	qw := queuedWrite{b: append([]byte(nil), b...)}

	if info != nil {
		// Copied so a caller reusing its struct cannot mutate a queued write.
		cp := *info
		qw.info = &cp
	}

	// Without this, a send on a closed association could report success.
	select {
	case <-c.writerDone:
		return 0, net.ErrClosed
	default:
	}

	select {
	case c.writeCh <- qw:
		return len(b), nil
	case <-c.writerDone:
		return 0, net.ErrClosed
	default:
		c.failAssociation("queue full", errWriteQueueFull)

		return 0, errWriteQueueFull
	}
}

// failAssociation closes the conn, ending the read loop so the server runs its
// disconnect cleanup.
func (c *SCTPConn) failAssociation(op string, err error) {
	if c.writeLogger != nil {
		c.writeLogger.Warn("SCTP write failed; closing association", zap.String("op", op), zap.Error(err))
	}

	_ = c.Close()
}

func (c *SCTPConn) awaitWriter() {
	if c.writerExited == nil {
		return
	}

	_ = c.Close()
	<-c.writerExited
}
