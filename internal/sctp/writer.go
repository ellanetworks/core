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
	// writeQueueDepth bounds the per-association outbound backlog. A healthy peer
	// drains in microseconds, so a backlog this deep means the peer has stopped
	// reading and the association is failed.
	writeQueueDepth = 256

	// writeTimeout bounds one blocking send. Expiry means the peer's receive
	// window has stayed closed this long, so the association is failed.
	writeTimeout = 5 * time.Second
)

var errWriteQueueFull = errors.New("sctp: outbound queue full")

type queuedWrite struct {
	b    []byte
	info *SndRcvInfo
}

// startWriter runs the dedicated writer goroutine for an accepted association.
// WriteMsg then enqueues onto a bounded channel drained here in FIFO order, so no
// application goroutine blocks on a slow peer and per-association ordering holds.
// A conn already closed exits the loop at once, since Close has signalled
// writerDone.
func (c *SCTPConn) startWriter(logger *zap.Logger) {
	c.writeLogger = logger
	c.writeCh = make(chan queuedWrite, writeQueueDepth)
	c.writerExited = make(chan struct{})

	go c.writeLoop()
}

// stopWriter signals the writer goroutine to exit, once however many times Close
// is called.
func (c *SCTPConn) stopWriter() {
	c.writerStop.Do(func() { close(c.writerDone) })
}

func (c *SCTPConn) writeLoop() {
	defer close(c.writerExited)

	for {
		// Checked first so a close is observed even when a queued write is also
		// ready, which would otherwise send on a closed fd.
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

// WriteMsg queues b for transmission by the association's writer goroutine and
// never blocks on a slow peer: a full queue means the peer has stopped reading,
// so the association is failed and the send reported as an error. A conn without
// a writer (dialled or test) sends synchronously.
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

	// Checked first so a send on a closed association is not reported as
	// success when the queue also has space.
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

// failAssociation closes the connection, which ends the read loop and runs the
// server's disconnect cleanup (RAN-loss teardown of the peer's UEs).
func (c *SCTPConn) failAssociation(op string, err error) {
	if c.writeLogger != nil {
		c.writeLogger.Warn("SCTP write failed; closing association", zap.String("op", op), zap.Error(err))
	}

	_ = c.Close()
}

// awaitWriter closes the connection and waits for the writer goroutine to exit,
// so the server's shutdown wait covers it. No-op for conns without a writer.
func (c *SCTPConn) awaitWriter() {
	if c.writerExited == nil {
		return
	}

	_ = c.Close()
	<-c.writerExited
}
