// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"
)

const (
	tcpReadBufferSize = 64 * 1024
	minDialAttempt    = 2 * time.Second
)

var tcpLingerTimeout = 2 * time.Second

var errFrame = errors.New("diameter: unframeable TCP stream")

type tcpListener struct {
	ln *net.TCPListener
}

func NewTCPListener(ln *net.TCPListener) Listener {
	return &tcpListener{ln: ln}
}

func (l *tcpListener) Addr() net.Addr { return l.ln.Addr() }

func (l *tcpListener) Close() error { return l.ln.Close() }

func (l *tcpListener) accept() (transport, error) {
	tc, err := l.ln.AcceptTCP()
	if err != nil {
		return nil, err
	}

	return newTCPTransport(tc), nil
}

type tcpTransport struct {
	conn   *net.TCPConn
	r      *bufio.Reader
	remote netip.Addr
	closed atomic.Bool

	writeMu      sync.Mutex
	writeTimeout time.Duration
}

func newTCPTransport(tc *net.TCPConn) *tcpTransport {
	var remote netip.Addr
	if a, ok := tc.RemoteAddr().(*net.TCPAddr); ok {
		remote = a.AddrPort().Addr().Unmap()
	}

	_ = tc.SetNoDelay(false)

	return &tcpTransport{
		conn:         tc,
		r:            bufio.NewReaderSize(tc, tcpReadBufferSize),
		remote:       remote,
		writeTimeout: 2 * DefaultWatchdogInterval,
	}
}

func dialTCP(ctx context.Context, local []netip.Addr, remote []netip.Addr, port uint16) (transport, error) {
	var firstErr error

	for i, addr := range remote {
		d := net.Dialer{Deadline: attemptDeadline(ctx, len(remote)-i)}
		if l, ok := sameFamily(local, addr); ok {
			d.LocalAddr = net.TCPAddrFromAddrPort(netip.AddrPortFrom(l, 0))
		}

		c, err := d.DialContext(ctx, "tcp", netip.AddrPortFrom(addr, port).String())
		if err == nil {
			return newTCPTransport(c.(*net.TCPConn)), nil
		}

		if firstErr == nil {
			firstErr = err
		}

		if ctx.Err() != nil {
			break
		}
	}

	if firstErr == nil {
		firstErr = errors.New("diameter: no address to dial")
	}

	return nil, firstErr
}

func attemptDeadline(ctx context.Context, remaining int) time.Time {
	deadline, ok := ctx.Deadline()
	if !ok || remaining <= 1 {
		return time.Time{}
	}

	return time.Now().Add(max(time.Until(deadline)/time.Duration(remaining), minDialAttempt))
}

func sameFamily(addrs []netip.Addr, to netip.Addr) (netip.Addr, bool) {
	for _, a := range addrs {
		if a.Is4() == to.Is4() {
			return a, true
		}
	}

	return netip.Addr{}, false
}

func (t *tcpTransport) kind() Transport { return TransportTCP }

func (t *tcpTransport) remoteAddr() netip.Addr { return t.remote }

func (t *tcpTransport) setUnordered() {}

func (t *tcpTransport) setWriteTimeout(d time.Duration) {
	t.writeMu.Lock()
	t.writeTimeout = d
	t.writeMu.Unlock()
}

func (t *tcpTransport) readMessage(buf []byte) (int, error) {
	if t.closed.Load() {
		return 0, net.ErrClosed
	}

	n, err := readFrame(t.r, buf)
	if err != nil && t.closed.Load() {
		return 0, net.ErrClosed
	}

	return n, err
}

func readFrame(r io.Reader, buf []byte) (int, error) {
	if _, err := io.ReadFull(r, buf[:4]); err != nil {
		return 0, err
	}

	if buf[0] != version {
		return 0, fmt.Errorf("%w: version %d", errFrame, buf[0])
	}

	length := int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3])

	switch {
	case length < headerLen:
		return 0, fmt.Errorf("%w: message length %d is shorter than the header", errFrame, length)
	case length > len(buf):
		return 0, fmt.Errorf("%w: message length %d exceeds the %d-octet limit", errFrame, length, len(buf))
	}

	if _, err := io.ReadFull(r, buf[4:length]); err != nil {
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}

		return 0, err
	}

	return length, nil
}

func (t *tcpTransport) writeMessage(b []byte) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	if t.closed.Load() {
		return net.ErrClosed
	}

	if err := t.conn.SetWriteDeadline(time.Now().Add(t.writeTimeout)); err != nil {
		return err
	}

	if _, err := t.conn.Write(b); err != nil {
		_ = t.abort()

		return err
	}

	return nil
}

func (t *tcpTransport) close() error {
	if !t.closed.CompareAndSwap(false, true) {
		return net.ErrClosed
	}

	t.writeMu.Lock()
	err := t.conn.CloseWrite()
	t.writeMu.Unlock()

	if err == nil {
		_ = t.conn.SetReadDeadline(time.Now().Add(tcpLingerTimeout))
		_, _ = io.Copy(io.Discard, t.conn)
	}

	if cerr := t.conn.Close(); cerr != nil && !errors.Is(cerr, net.ErrClosed) {
		return cerr
	}

	return nil
}

func (t *tcpTransport) abort() error {
	t.closed.Store(true)
	_ = t.conn.SetLinger(0)

	return t.conn.Close()
}
