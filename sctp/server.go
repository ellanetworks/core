// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sctp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"runtime/debug"
	"sync"
	"syscall"
	"time"
)

const readBufSize uint32 = 131072

// RTO and association limits are the RFC 4960 §15 values, set explicitly to not
// depend on host net.sctp.* sysctls. MaxAttempts and MaxInitTimeout apply only to
// an initiating socket, so they are inert here.
var serverSocketConfig = socketConfig{
	InitMsg:   InitMsg{NumOstreams: 2, MaxInstreams: 5, MaxAttempts: 2, MaxInitTimeout: 2},
	rtoInfo:   &rtoInfo{SrtoAssocID: 0, SrtoInitial: 3000, SrtoMax: 60000, SrtoMin: 1000},
	assocInfo: &assocInfo{AsocMaxRxt: 10},
}

// Config parameterizes a Server for one RAN-facing signalling interface.
type Config struct {
	// PPID is the SCTP payload protocol identifier accepted on inbound messages:
	// 18 for S1AP, 60 for NGAP (TS 36.412 §7, TS 38.412 §7). The wire byte order
	// is big-endian.
	PPID uint32
	// Name labels the interface in log messages, e.g. "NGAP" or "S1-MME".
	Name string
	// Logger receives the server's lifecycle and per-connection logs.
	Logger *slog.Logger
}

// Callbacks groups the functions the SCTP server calls into the upper layer.
// Dispatch runs on the association's read goroutine: serial within an
// association, concurrent across associations, and blocking it stalls that
// association's reads.
type Callbacks struct {
	// Dispatch is invoked for every complete message read from a connection.
	Dispatch  func(ctx context.Context, conn *SCTPConn, msg []byte)
	OnConnect func(conn *SCTPConn)
	// Notify is invoked for SCTP association/shutdown events.
	Notify func(conn *SCTPConn, notification Notification)
	// OnDisconnect is invoked once per connection, after its socket is closed.
	OnDisconnect func(conn *SCTPConn)
}

// Server accepts SCTP connections and dispatches application-layer messages.
// Create one with NewServer, call Serve with a listener to start accepting, and
// Shutdown to stop cleanly.
type Server struct {
	cfg        Config
	cb         Callbacks
	listener   *Listener
	conns      sync.Map
	wg         sync.WaitGroup
	acceptDone chan struct{}
}

func NewServer(cfg Config, cb Callbacks) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &Server{cfg: cfg, cb: cb}
}

type ListenConfig struct {
	Control func(network, address string, c syscall.RawConn) error
}

func (lc *ListenConfig) Listen(ctx context.Context, laddr *SCTPAddr) (*Listener, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cfg := serverSocketConfig
	cfg.Control = lc.Control

	ln, err := cfg.Listen("sctp", laddr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", laddr, err)
	}

	ln.reqAddr = &SCTPAddr{IPAddrs: laddr.IPAddrs, Port: ln.laddr.Port}

	return ln, nil
}

func (s *Server) Serve(ctx context.Context, ln *Listener) {
	s.listener = ln
	s.acceptDone = make(chan struct{})

	addr := ln.laddr
	if ln.reqAddr != nil {
		addr = ln.reqAddr
	}

	s.cfg.Logger.Info("SCTP server started", slog.String("listener", s.cfg.Name), slog.String("address", addr.String()))

	go s.acceptLoop(ctx)
}

func (s *Server) acceptLoop(ctx context.Context) {
	defer close(s.acceptDone)

	backoff := time.Duration(0)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				s.cfg.Logger.Debug("Accept loop exiting", slog.Any("error", err))
				return
			}

			// A persistent failure (typically fd exhaustion) leaves the pending
			// association queued, so retrying immediately spins on the same error.
			backoff = nextAcceptBackoff(backoff)

			s.cfg.Logger.Error("Failed to accept", slog.Any("error", err), slog.Duration("retry_in", backoff))

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}

			continue
		}

		backoff = 0

		conn.startWriter(s.cfg.Logger)

		s.conns.Store(conn, struct{}{})
		s.wg.Add(1)

		go s.serveConn(ctx, conn)
	}
}

func (s *Server) serveConn(ctx context.Context, conn *SCTPConn) {
	defer s.wg.Done()
	defer s.conns.Delete(conn)

	defer func() {
		if s.cb.OnDisconnect != nil {
			s.cb.OnDisconnect(conn)
		}
	}()

	defer func() {
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			s.cfg.Logger.Warn("close connection error", slog.Any("error", err))
		}
	}()

	defer conn.awaitWriter()

	// PartialDelivery is required for correctness, not observability: without the
	// subscription the kernel abandons a partial message silently and the next
	// message is read as its continuation.
	sctpEvents := sctpEventDataIO | sctpEventShutdown | sctpEventAssociation | sctpEventPartialDelivery
	if err := conn.subscribeEvents(sctpEvents); err != nil {
		s.cfg.Logger.Error("Failed to subscribe to SCTP events", slog.Any("error", err))
		return
	}

	if err := conn.setReadBuffer(int(readBufSize)); err != nil {
		s.cfg.Logger.Error("Set read buffer error", slog.Any("error", err))
		return
	}

	remoteAddr := conn.RemoteAddr()
	if remoteAddr == nil {
		s.cfg.Logger.Error("Remote address is nil")
		return
	}

	s.cfg.Logger.Info("New SCTP connection", slog.String("remote_address", remoteAddr.String()))

	if s.cb.OnConnect != nil {
		s.cb.OnConnect(conn)
	}

	buf := make([]byte, readBufSize)
	discarded := 0

	defer func() {
		if discarded > 0 {
			s.cfg.Logger.Warn("discarded messages with an unexpected PPID",
				slog.Uint64("expected", uint64(s.cfg.PPID)), slog.Int("count", discarded))
		}
	}()

	for {
		n, info, notification, err := conn.readMsg(buf)
		if err != nil {
			// Anything the framing layer rejected leaves the association's message
			// boundaries in doubt, so it cannot be handed back to the peer intact.
			if errors.Is(err, errMessageTooLarge) ||
				errors.Is(err, errUnexpectedNotification) ||
				errors.Is(err, errUnrecognizedDelivery) {
				s.cfg.Logger.Warn("aborting association on unusable delivery",
					slog.Any("error", err), slog.Int("read_buffer", len(buf)))

				_ = conn.Abort()

				return
			}

			if err != io.EOF && !errors.Is(err, net.ErrClosed) {
				s.cfg.Logger.Debug("readMsg terminated", slog.Any("error", err))
			}

			return
		}

		if notification != nil {
			if s.cb.Notify != nil {
				s.cb.Notify(conn, notification)
			}

			continue
		}

		// Counted rather than logged per message: the peer controls the rate.
		if info == nil || PPIDWireOrder(info.PPID) != s.cfg.PPID {
			discarded++

			continue
		}

		msg := make([]byte, n)
		copy(msg, buf[:n])

		s.dispatch(ctx, conn, msg)
	}
}

// nextAcceptBackoff grows the accept retry delay up to a ceiling.
func nextAcceptBackoff(current time.Duration) time.Duration {
	const (
		initial = 5 * time.Millisecond
		ceiling = time.Second
	)

	if current == 0 {
		return initial
	}

	if next := current * 2; next < ceiling {
		return next
	}

	return ceiling
}

// dispatch isolates a panic in message handling to the association that caused
// it, so one malformed PDU cannot take down every other radio.
func (s *Server) dispatch(ctx context.Context, conn *SCTPConn, msg []byte) {
	defer func() {
		if r := recover(); r != nil {
			s.cfg.Logger.Error("panic handling message; aborting association",
				slog.Any("panic", r), slog.String("stack", string(debug.Stack())))

			_ = conn.Abort()
		}
	}()

	s.cb.Dispatch(ctx, conn, msg)
}

func (s *Server) Shutdown(ctx context.Context) {
	if s.listener == nil {
		return
	}

	if err := s.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		s.cfg.Logger.Warn("could not close sctp listener", slog.Any("error", err))
	}

	select {
	case <-s.acceptDone:
	case <-ctx.Done():
		s.cfg.Logger.Warn("Timed out waiting for accept loop to exit")
	}

	// Closed concurrently: each graceful close can take seconds against a peer
	// that has stopped draining, and they would otherwise serialise.
	var closing sync.WaitGroup

	s.conns.Range(func(key, _ any) bool {
		conn := key.(*SCTPConn)

		closing.Add(1)

		go func() {
			defer closing.Done()

			if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				s.cfg.Logger.Warn("close connection error", slog.Any("error", err))
			}
		}()

		return true
	})

	done := make(chan struct{})

	go func() {
		closing.Wait()
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.cfg.Logger.Info("SCTP server closed")
	case <-ctx.Done():
		s.cfg.Logger.Warn("SCTP server shutdown timed out")
	}
}

// PPIDWireOrder converts an SCTP Payload Protocol Identifier between host order
// and the big-endian wire order the socket layer writes verbatim (TS 36.412 §7,
// TS 38.412 §7). The conversion is symmetric: the same call encodes a PPID for
// sending and decodes a received one.
func PPIDWireOrder(ppid uint32) uint32 {
	var b [4]byte

	binary.BigEndian.PutUint32(b[:], ppid)

	return binary.NativeEndian.Uint32(b[:])
}
