// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

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

// s1apPPID is the SCTP payload protocol identifier for S1AP (TS 36.412).
const s1apPPID uint32 = 18

const readBufSize uint32 = 131072

// The SCTP server mirrors internal/amf/ngap/service: the MME keeps its own copy
// so it does not depend on AMF internals. A shared internal/sctp server can
// replace both once 4G bring-up stabilises.

var sctpConfig = sctp.SocketConfig{
	InitMsg:   sctp.InitMsg{NumOstreams: 2, MaxInstreams: 5, MaxAttempts: 2, MaxInitTimeout: 2},
	RtoInfo:   &sctp.RtoInfo{SrtoAssocID: 0, SrtoInitial: 500, SrtoMax: 1500, StroMin: 100},
	AssocInfo: &sctp.AssocInfo{AsocMaxRxt: 4},
}

// callbacks groups the functions the SCTP server calls into the upper layer.
// None of the callbacks should block for extended periods.
type callbacks struct {
	// dispatch is invoked for every complete S1AP message read from a connection.
	dispatch func(ctx context.Context, conn *sctp.SCTPConn, msg []byte)
	// onDisconnect is invoked when a connection is closing, before the socket closes.
	onDisconnect func(conn *sctp.SCTPConn)
}

// server accepts SCTP connections on S1-MME and dispatches S1AP messages.
type server struct {
	cb         callbacks
	listener   *sctp.SCTPListener
	conns      sync.Map
	wg         sync.WaitGroup
	acceptDone chan struct{}
}

func newServer(cb callbacks) *server {
	return &server{cb: cb}
}

func (s *server) listenAndServe(ctx context.Context, address string, port int) error {
	netAddr, err := net.ResolveIPAddr("ip", address)
	if err != nil {
		return fmt.Errorf("error resolving address %q: %w", address, err)
	}

	laddr := &sctp.SCTPAddr{IPAddrs: []net.IPAddr{*netAddr}, Port: port}

	listener, err := sctpConfig.Listen("sctp", laddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", laddr.String(), err)
	}

	s.listener = listener
	s.acceptDone = make(chan struct{})

	logger.MmeLog.Info("S1-MME server started", zap.String("address", laddr.String()))

	go s.acceptLoop(ctx)

	return nil
}

func (s *server) acceptLoop(ctx context.Context) {
	defer close(s.acceptDone)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				logger.MmeLog.Debug("Accept loop exiting", zap.Error(err))
				return
			}

			logger.MmeLog.Error("Failed to accept", zap.Error(err))

			continue
		}

		s.conns.Store(conn, struct{}{})
		s.wg.Add(1)

		go s.serveConn(ctx, conn)
	}
}

func (s *server) serveConn(ctx context.Context, conn *sctp.SCTPConn) {
	defer s.wg.Done()
	defer s.conns.Delete(conn)

	defer func() {
		if s.cb.onDisconnect != nil {
			s.cb.onDisconnect(conn)
		}
	}()

	defer func() {
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			logger.MmeLog.Warn("close connection error", zap.Error(err))
		}
	}()

	sctpEvents := sctp.SCTPEventDataIO | sctp.SCTPEventShutdown | sctp.SCTPEventAssociation
	if err := conn.SubscribeEvents(sctpEvents); err != nil {
		logger.MmeLog.Error("Failed to subscribe to SCTP events", zap.Error(err))
		return
	}

	if err := conn.SetReadBuffer(int(readBufSize)); err != nil {
		logger.MmeLog.Error("Set read buffer error", zap.Error(err))
		return
	}

	remoteAddr := conn.RemoteAddr()
	if remoteAddr == nil {
		logger.MmeLog.Error("Remote address is nil")
		return
	}

	logger.MmeLog.Info("New SCTP connection", zap.String("remote_address", remoteAddr.String()))

	buf := make([]byte, readBufSize)

	for {
		n, info, notification, err := conn.ReadMsg(buf)
		if err != nil {
			if err != io.EOF && !errors.Is(err, net.ErrClosed) {
				logger.MmeLog.Debug("ReadMsg terminated", zap.Error(err))
			}

			return
		}

		if notification != nil {
			continue
		}

		if info == nil || networkToNativeEndianness32(info.PPID) != s1apPPID {
			logger.MmeLog.Warn("Received SCTP message with PPID != 18, discarding")
			continue
		}

		msg := make([]byte, n)
		copy(msg, buf[:n])
		s.cb.dispatch(ctx, conn, msg)
	}
}

func (s *server) shutdown(ctx context.Context) {
	if s.listener == nil {
		return
	}

	if err := s.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		logger.MmeLog.Warn("could not close sctp listener", zap.Error(err))
	}

	select {
	case <-s.acceptDone:
	case <-ctx.Done():
		logger.MmeLog.Warn("Timed out waiting for accept loop to exit")
	}

	s.conns.Range(func(key, _ any) bool {
		conn := key.(*sctp.SCTPConn)
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			logger.MmeLog.Warn("close connection error", zap.Error(err))
		}

		return true
	})

	done := make(chan struct{})

	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.MmeLog.Info("S1-MME server closed")
	case <-ctx.Done():
		logger.MmeLog.Warn("S1-MME server shutdown timed out")
	}
}

// networkToNativeEndianness32 converts a uint32 from network to native byte order.
func networkToNativeEndianness32(value uint32) uint32 {
	var b [4]byte

	binary.BigEndian.PutUint32(b[:], value)

	return binary.NativeEndian.Uint32(b[:])
}
