// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"syscall"

	"github.com/ellanetworks/core/internal/amf/ngap"
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

// Server accepts SCTP connections and dispatches NGAP messages. Create one
// with NewServer, call ListenAndServe to start accepting, and Shutdown to
// stop cleanly.
type Server struct {
	listener *sctp.SCTPListener
	conns    sync.Map
	wg       sync.WaitGroup
}

func NewServer() *Server {
	return &Server{}
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

	logger.AmfLog.Info("NGAP server started", zap.String("address", addr.String()))

	go s.acceptLoop(ctx)

	return nil
}

func (s *Server) acceptLoop(ctx context.Context) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			switch err {
			case syscall.EINVAL, syscall.EBADF:
				return // listener closed by Shutdown()
			default:
				if ctx.Err() != nil {
					return
				}

				logger.AmfLog.Error("Failed to accept", zap.Error(err))

				continue
			}
		}

		// Store before wg.Add so Shutdown's Range sees every connection
		// that has a corresponding goroutine in wg.
		s.conns.Store(conn.Fd(), conn)
		s.wg.Add(1)

		go s.serveConn(ctx, conn)
	}
}

func (s *Server) serveConn(ctx context.Context, conn *sctp.SCTPConn) {
	defer s.wg.Done()
	defer s.conns.Delete(conn.Fd())
	defer func() {
		if err := conn.Close(); err != nil && err != syscall.EBADF {
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
		n, info, notification, err := conn.SCTPRead(buf)
		if err != nil {
			switch err {
			case io.EOF, io.ErrUnexpectedEOF, syscall.EBADF, syscall.EINVAL:
				return
			case syscall.EAGAIN, syscall.EINTR:
				continue
			default:
				logger.AmfLog.Error("SCTPRead error", zap.Error(err))
				return
			}
		}

		if notification != nil {
			ngap.HandleSCTPNotification(conn, notification)
			continue
		}

		if info == nil || networkToNativeEndianness32(info.PPID) != sctp.NGAPPPID {
			logger.AmfLog.Warn("Received SCTP PPID != 60, discard this packet")
			continue
		}

		msg := make([]byte, n)
		copy(msg, buf[:n])

		ngap.Dispatch(ctx, conn, msg)
	}
}

func (s *Server) Shutdown() {
	if err := s.listener.Close(); err != nil {
		logger.AmfLog.Error("could not close sctp listener", zap.Error(err))
	}

	s.conns.Range(func(_, value any) bool {
		conn := value.(*sctp.SCTPConn)

		if err := conn.Close(); err != nil && err != syscall.EBADF {
			logger.AmfLog.Error("close connection error", zap.Error(err))
		}

		return true
	})

	s.wg.Wait()
	logger.AmfLog.Info("SCTP server closed")
}

// networkToNativeEndianness32 converts a uint32 from network byte order to native byte order.
func networkToNativeEndianness32(value uint32) uint32 {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], value)

	return binary.NativeEndian.Uint32(b[:])
}

// Package-level server instance and shim functions to preserve the existing API.
var server = NewServer()

func Run(ctx context.Context, address string, port int) error {
	return server.ListenAndServe(ctx, address, port)
}

func Stop() {
	server.Shutdown()
}
