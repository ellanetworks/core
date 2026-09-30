// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"sync/atomic"

	"github.com/ellanetworks/core/sctp"
)

const ppidUnspecified uint32 = 0

var sctpInit = sctp.InitMsg{NumOstreams: 2, MaxInstreams: 5, MaxAttempts: 4, MaxInitTimeout: 3000}

type sctpListener struct {
	ln     *sctp.Listener
	logger *slog.Logger
}

func NewSCTPListener(ln *sctp.Listener, logger *slog.Logger) Listener {
	if logger == nil {
		logger = slog.Default()
	}

	return &sctpListener{ln: ln, logger: logger}
}

func (l *sctpListener) Addr() net.Addr { return l.ln.Addr() }

func (l *sctpListener) Close() error { return l.ln.Close() }

func (l *sctpListener) accept() (transport, error) {
	sc, err := l.ln.Accept()
	if err != nil {
		return nil, err
	}

	if err := sc.PrepareAccepted(l.logger); err != nil {
		_ = sc.Abort()

		return nil, fmt.Errorf("%w: %w", errConnectionSetup, err)
	}

	t, err := newSCTPTransport(sc)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errConnectionSetup, err)
	}

	return t, nil
}

type sctpTransport struct {
	sc        *sctp.SCTPConn
	remote    netip.Addr
	unordered atomic.Bool
	closed    atomic.Bool
}

func newSCTPTransport(sc *sctp.SCTPConn) (*sctpTransport, error) {
	remote, err := sc.PrimaryPeerAddr()
	if err != nil {
		_ = sc.Abort()

		return nil, err
	}

	return &sctpTransport{sc: sc, remote: remote}, nil
}

func dialSCTP(ctx context.Context, local []netip.Addr, remote []netip.Addr, port uint16, logger *slog.Logger) (transport, error) {
	d := sctp.Dialer{InitMsg: sctpInit, Logger: logger}

	if len(local) > 0 {
		d.LocalAddr = &sctp.SCTPAddr{IPAddrs: ipAddrs(local)}
	}

	sc, err := d.Dial(ctx, &sctp.SCTPAddr{IPAddrs: ipAddrs(remote), Port: int(port)})
	if err != nil {
		return nil, err
	}

	return newSCTPTransport(sc)
}

func ipAddrs(addrs []netip.Addr) []net.IPAddr {
	out := make([]net.IPAddr, 0, len(addrs))

	for _, a := range addrs {
		out = append(out, net.IPAddr{IP: a.AsSlice(), Zone: a.Zone()})
	}

	return out
}

func (t *sctpTransport) kind() Transport { return TransportSCTP }

func (t *sctpTransport) remoteAddr() netip.Addr { return t.remote }

func (t *sctpTransport) setUnordered() { t.unordered.Store(true) }

func (t *sctpTransport) readMessage(buf []byte) (int, error) {
	for {
		n, info, err := t.sc.ReadMsg(buf)
		if err != nil && t.closed.Load() {
			return 0, net.ErrClosed
		}

		if err != nil {
			return 0, err
		}

		if info != nil {
			if ppid := sctp.PPIDWireOrder(info.PPID); ppid != PPID && ppid != ppidUnspecified {
				continue
			}
		}

		return n, nil
	}
}

func (t *sctpTransport) writeMessage(b []byte) error {
	info := sctp.SndRcvInfo{PPID: sctp.PPIDWireOrder(PPID)}
	if t.unordered.Load() {
		info.Flags = sctp.SCTPUnordered
	}

	_, err := t.sc.WriteMsg(b, &info)

	return err
}

func (t *sctpTransport) close() error {
	t.closed.Store(true)
	return t.sc.Close()
}

func (t *sctpTransport) abort() error {
	t.closed.Store(true)
	return t.sc.Abort()
}
