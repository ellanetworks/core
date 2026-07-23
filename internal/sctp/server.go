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
	"net"
	"sync"

	"github.com/ellanetworks/core/internal/netutil"
	"go.uber.org/zap"
)

const readBufSize uint32 = 131072

var errNoInterfaceAddrs = errors.New("no IP addresses found")

// The 5 s heartbeat keeps middlebox flows alive; the RTO and retransmit count
// tolerate transient loss on idle or lossy links before aborting.
var serverSocketConfig = SocketConfig{
	InitMsg:        InitMsg{NumOstreams: 2, MaxInstreams: 5, MaxAttempts: 2, MaxInitTimeout: 2},
	RtoInfo:        &RtoInfo{SrtoAssocID: 0, SrtoInitial: 3000, SrtoMax: 5000, SrtoMin: 1000},
	AssocInfo:      &AssocInfo{AsocMaxRxt: 10},
	PeerAddrParams: &PeerAddrParams{HBIntervalMs: 5000},
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
	Logger *zap.Logger
}

// Callbacks groups the functions the SCTP server calls into the upper layer.
// None of the callbacks should block for extended periods.
type Callbacks struct {
	// Dispatch is invoked for every complete message read from a connection.
	Dispatch func(ctx context.Context, conn *SCTPConn, msg []byte)
	// Notify is invoked for SCTP association/shutdown events.
	Notify func(conn *SCTPConn, notification Notification)
	// OnDisconnect is invoked when a connection is closing, before the socket is closed.
	OnDisconnect func(conn *SCTPConn)
}

// Server accepts SCTP connections and dispatches application-layer messages.
// Create one with NewServer, call ListenAndServe to start accepting, and
// Shutdown to stop cleanly.
type Server struct {
	cfg        Config
	cb         Callbacks
	listener   *SCTPListener
	conns      sync.Map
	wg         sync.WaitGroup
	acceptDone chan struct{}
}

func NewServer(cfg Config, cb Callbacks) *Server {
	return &Server{cfg: cfg, cb: cb}
}

func (s *Server) ListenAndServe(ctx context.Context, address string, port int, interfaceName string) error {
	var (
		laddr   *SCTPAddr
		addrStr string
	)

	// A bind can transiently fail while a shared N2/N3 interface flaps; retry
	// resolve and listen together.
	bind := func() error {
		if interfaceName != "" {
			iface, err := net.InterfaceByName(interfaceName)
			if err != nil {
				return fmt.Errorf("failed to get interface %s: %w", interfaceName, err)
			}

			addrs, err := iface.Addrs()
			if err != nil {
				return fmt.Errorf("failed to get interface addresses: %w", err)
			}

			var ipAddrs []net.IPAddr

			for _, addr := range addrs {
				ipNet, ok := addr.(*net.IPNet)
				if !ok {
					continue
				}

				ip := ipNet.IP
				if ip.IsLoopback() {
					continue
				}

				if ip.IsLinkLocalUnicast() {
					continue
				}

				ipAddrs = append(ipAddrs, net.IPAddr{IP: ip})
			}

			if len(ipAddrs) == 0 {
				return fmt.Errorf("%w on interface %s", errNoInterfaceAddrs, interfaceName)
			}

			laddr = &SCTPAddr{IPAddrs: ipAddrs, Port: port}
			addrStr = laddr.String()
		} else {
			netAddr, err := net.ResolveIPAddr("ip", address)
			if err != nil {
				return fmt.Errorf("error resolving address %q: %w", address, err)
			}

			laddr = &SCTPAddr{IPAddrs: []net.IPAddr{*netAddr}, Port: port}
			addrStr = laddr.String()
		}

		return nil
	}

	isTransient := func(err error) bool {
		return errors.Is(err, errNoInterfaceAddrs) || netutil.IsAddrNotAvailable(err)
	}

	var listener *SCTPListener

	err := netutil.Retry(ctx, netutil.BindTimeout, netutil.BindInterval, isTransient, func() error {
		if err := bind(); err != nil {
			return err
		}

		l, err := serverSocketConfig.Listen("sctp", laddr)
		if err != nil {
			return fmt.Errorf("failed to listen on %s: %w", addrStr, err)
		}

		listener = l

		return nil
	})
	if err != nil {
		return err
	}

	s.listener = listener
	s.acceptDone = make(chan struct{})

	logFields := []zap.Field{zap.String("interface", s.cfg.Name), zap.String("address", addrStr)}
	if interfaceName != "" {
		logFields = append(logFields, zap.String("interface_name", interfaceName))
	}

	s.cfg.Logger.Info("SCTP server started", logFields...)

	go s.acceptLoop(ctx)

	return nil
}

func (s *Server) acceptLoop(ctx context.Context) {
	defer close(s.acceptDone)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				s.cfg.Logger.Debug("Accept loop exiting", zap.Error(err))
				return
			}

			s.cfg.Logger.Error("Failed to accept", zap.Error(err))

			continue
		}

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
			s.cfg.Logger.Warn("close connection error", zap.Error(err))
		}
	}()

	sctpEvents := SCTPEventDataIO | SCTPEventShutdown | SCTPEventAssociation
	if err := conn.SubscribeEvents(sctpEvents); err != nil {
		s.cfg.Logger.Error("Failed to subscribe to SCTP events", zap.Error(err))
		return
	}

	if err := conn.SetReadBuffer(int(readBufSize)); err != nil {
		s.cfg.Logger.Error("Set read buffer error", zap.Error(err))
		return
	}

	remoteAddr := conn.RemoteAddr()
	if remoteAddr == nil {
		s.cfg.Logger.Error("Remote address is nil")
		return
	}

	s.cfg.Logger.Info("New SCTP connection", zap.String("remote_address", remoteAddr.String()))

	buf := make([]byte, readBufSize)

	for {
		n, info, notification, err := conn.ReadMsg(buf)
		if err != nil {
			if err != io.EOF && !errors.Is(err, net.ErrClosed) {
				s.cfg.Logger.Debug("ReadMsg terminated", zap.Error(err))
			}

			return
		}

		if notification != nil {
			if s.cb.Notify != nil {
				s.cb.Notify(conn, notification)
			}

			continue
		}

		if info == nil || PPIDWireOrder(info.PPID) != s.cfg.PPID {
			s.cfg.Logger.Warn("Received SCTP message with unexpected PPID, discarding",
				zap.Uint32("expected", s.cfg.PPID))

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

	if err := s.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		s.cfg.Logger.Warn("could not close sctp listener", zap.Error(err))
	}

	select {
	case <-s.acceptDone:
	case <-ctx.Done():
		s.cfg.Logger.Warn("Timed out waiting for accept loop to exit")
	}

	s.conns.Range(func(key, _ any) bool {
		conn := key.(*SCTPConn)
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			s.cfg.Logger.Warn("close connection error", zap.Error(err))
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
