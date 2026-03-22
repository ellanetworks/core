// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const readBufSize uint32 = 131072

var sctpConfig sctp.SocketConfig = sctp.SocketConfig{
	InitMsg:   sctp.InitMsg{NumOstreams: 2, MaxInstreams: 5, MaxAttempts: 2, MaxInitTimeout: 2},
	RtoInfo:   &sctp.RtoInfo{SrtoAssocID: 0, SrtoInitial: 500, SrtoMax: 1500, StroMin: 100},
	AssocInfo: &sctp.AssocInfo{AsocMaxRxt: 4},
}

// Callbacks groups the functions the SCTP server calls into the upper layer.
// None of the callbacks should block for extended periods.
type Callbacks struct {
	// Dispatch is invoked for every complete NGAP message read from a connection.
	Dispatch func(ctx context.Context, conn *sctp.SCTPConn, msg []byte)
	// Notify is invoked for SCTP association/shutdown events.
	Notify func(conn *sctp.SCTPConn, notification sctp.Notification)
	// OnDisconnect is invoked when a connection is closing, before the socket is closed.
	OnDisconnect func(conn *sctp.SCTPConn)
}

// Server accepts SCTP connections and dispatches NGAP messages. Create one
// with NewServer, call ListenAndServe to start accepting, and Shutdown to
// stop cleanly.
type Server struct {
	cb         Callbacks
	listener   *sctp.SCTPListener
	conns      sync.Map
	wg         sync.WaitGroup
	acceptDone chan struct{}
}

func NewServer(cb Callbacks) *Server {
	return &Server{cb: cb}
}

func (s *Server) ListenAndServe(ctx context.Context, address string, port int) error {
	netAddr, err := net.ResolveIPAddr("ip", address)
	if err != nil {
		return fmt.Errorf("error resolving address '%s': %v", address, err)
	}

	addr := &sctp.SCTPAddr{
		IPAddrs: []net.IPAddr{*netAddr},
		Port:    port,
	}

	listener, err := sctpConfig.Listen("sctp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", addr, err)
	}

	s.listener = listener
	s.acceptDone = make(chan struct{}) // fresh channel each call for restart-safety

	logger.AmfLog.Info("NGAP server started", zap.String("address", addr.String()))

	go s.acceptLoop(ctx)

	return nil
}

func (s *Server) acceptLoop(ctx context.Context) {
	defer close(s.acceptDone)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// When the listener is closed (by Shutdown), Accept returns a
			// "use of closed file" error from the runtime poller, or EBADF.
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				logger.AmfLog.Debug("Accept loop exiting", zap.Error(err))
				return
			}

			logger.AmfLog.Error("Failed to accept", zap.Error(err))

			continue
		}

		// Store before wg.Add so Shutdown's Range sees every connection
		// that has a corresponding goroutine in wg.
		s.conns.Store(conn, struct{}{})
		s.wg.Add(1)

		go s.serveConn(ctx, conn)
	}
}

func (s *Server) serveConn(ctx context.Context, conn *sctp.SCTPConn) {
	defer s.wg.Done()
	defer s.conns.Delete(conn)
	defer func() {
		if s.cb.OnDisconnect != nil {
			s.cb.OnDisconnect(conn)
		}
	}()
	defer func() {
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			logger.AmfLog.Error("close connection error", zap.Error(err))
		}
	}()

	sctpEvents := sctp.SCTPEventDataIO | sctp.SCTPEventShutdown | sctp.SCTPEventAssociation
	if err := conn.SubscribeEvents(sctpEvents); err != nil {
		logger.AmfLog.Error("Failed to subscribe to SCTP events", zap.Error(err))
		return
	}

	if err := conn.SetReadBuffer(int(readBufSize)); err != nil {
		logger.AmfLog.Error("Set read buffer error", zap.Error(err))
		return
	}

	remoteAddr := conn.RemoteAddr()
	if remoteAddr == nil {
		logger.AmfLog.Error("Remote address is nil")
		return
	}

	logger.AmfLog.Info("New SCTP connection", zap.String("remote_address", remoteAddr.String()))

	buf := make([]byte, readBufSize)

	for {
		n, info, notification, err := conn.ReadMsg(buf)
		if err != nil {
			if err != io.EOF && !errors.Is(err, net.ErrClosed) {
				logger.AmfLog.Debug("ReadMsg terminated", zap.Error(err))
			}

			return
		}

		if notification != nil {
			if s.cb.Notify != nil {
				s.cb.Notify(conn, notification)
			}

			continue
		}

		if info == nil || networkToNativeEndianness32(info.PPID) != sctp.NGAPPPID {
			logger.AmfLog.Warn("Received SCTP PPID != 60, discard this packet")
			continue
		}

		msg := make([]byte, n)
		copy(msg, buf[:n])

		s.cb.Dispatch(ctx, conn, msg)
	}
}

func (s *Server) Shutdown(ctx context.Context) {
	if s.listener == nil {
		return
	}

	logger.AmfLog.Info("Signaling SCTP listener to stop")

	if err := s.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		logger.AmfLog.Error("could not close sctp listener", zap.Error(err))
	}

	// Wait for acceptLoop to exit. Close() unparks any goroutine blocked in
	// Accept via the runtime poller — no manual wakeup pipe needed.
	select {
	case <-s.acceptDone:
		logger.AmfLog.Info("Accept loop exited")
	case <-ctx.Done():
		logger.AmfLog.Warn("Timed out waiting for accept loop to exit")
	}

	logger.AmfLog.Info("Closing SCTP connections")

	// Close all connections. Each Close() sends SCTP EOF, shuts down the
	// socket, and wakes up any goroutine blocked in ReadMsg via the runtime
	// poller. No manual read timeout hack needed.
	s.conns.Range(func(key, _ any) bool {
		conn := key.(*sctp.SCTPConn)

		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			logger.AmfLog.Error("close connection error", zap.Error(err))
		}

		return true
	})

	logger.AmfLog.Info("All connections closed, waiting for serve goroutines")

	done := make(chan struct{})

	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.AmfLog.Info("SCTP server closed")
	case <-ctx.Done():
		logger.AmfLog.Warn("SCTP server shutdown timed out, some connections may not have closed cleanly")
	}
}

// networkToNativeEndianness32 converts a uint32 from network byte order to native byte order.
func networkToNativeEndianness32(value uint32) uint32 {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], value)

	return binary.NativeEndian.Uint32(b[:])
}
