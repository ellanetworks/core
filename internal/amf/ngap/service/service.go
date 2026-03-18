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

var (
	sctpListener *sctp.SCTPListener
	connections  sync.Map // key: int (fd), value: *connWorker
	wg           sync.WaitGroup
)

// connWorker serialises NGAP dispatch for one SCTP connection, matching the
// behaviour of the previous goroutine-per-connection model: different gNB
// connections dispatch concurrently; messages on the same connection are
// processed in order.
type connWorker struct {
	conn  *sctp.SCTPConn
	msgCh chan connMsg
}

type connMsg struct {
	ctx  context.Context
	data []byte
}

func newConnWorker(conn *sctp.SCTPConn) *connWorker {
	w := &connWorker{
		conn:  conn,
		msgCh: make(chan connMsg, 256),
	}

	wg.Add(1)

	go w.run()

	return w
}

func (w *connWorker) run() {
	defer wg.Done()

	for msg := range w.msgCh {
		ngap.Dispatch(msg.ctx, w.conn, msg.data)
	}
}

func (w *connWorker) enqueue(ctx context.Context, data []byte) {
	msg := make([]byte, len(data))
	copy(msg, data)

	w.msgCh <- connMsg{ctx: ctx, data: msg}
}

var sctpConfig sctp.SocketConfig = sctp.SocketConfig{
	InitMsg:   sctp.InitMsg{NumOstreams: 2, MaxInstreams: 5, MaxAttempts: 2, MaxInitTimeout: 2},
	RtoInfo:   &sctp.RtoInfo{SrtoAssocID: 0, SrtoInitial: 500, SrtoMax: 1500, StroMin: 100},
	AssocInfo: &sctp.AssocInfo{AsocMaxRxt: 4},
}

func Run(ctx context.Context, address string, port int) error {
	netAddr, err := net.ResolveIPAddr("ip", address)
	if err != nil {
		return fmt.Errorf("error resolving address '%s': %v", address, err)
	}

	addr := &sctp.SCTPAddr{
		IPAddrs: []net.IPAddr{*netAddr},
		Port:    port,
	}

	go listenAndServe(ctx, addr)

	return nil
}

func listenAndServe(ctx context.Context, addr *sctp.SCTPAddr) {
	listener, err := sctpConfig.Listen("sctp", addr)
	if err != nil {
		logger.AmfLog.Error("Failed to listen", zap.Error(err))
		return
	}

	sctpListener = listener

	logger.AmfLog.Info("NGAP server started", zap.String("address", addr.String()))

	// buf is reused across iterations; the reactor is the only reader and copies
	// data into fresh slices before enqueuing to worker goroutines.
	buf := make([]byte, readBufSize)
	events := make([]syscall.EpollEvent, 32)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := listener.Poll(events, 100)
		if err != nil {
			switch err {
			case syscall.EINTR:
				continue
			case syscall.EBADF, syscall.EINVAL:
				return // listener was closed by Stop()
			default:
				logger.AmfLog.Error("epoll wait error", zap.Error(err))
				continue
			}
		}

		for i := 0; i < n; i++ {
			fd := int(events[i].Fd)

			if fd == listener.ListenerFd() {
				accept(ctx, listener)
				continue
			}

			val, ok := connections.Load(fd)
			if !ok {
				continue
			}

			worker := val.(*connWorker)

			// On hangup, try to drain any last message before closing.
			if events[i].Events&(syscall.EPOLLRDHUP|syscall.EPOLLHUP|syscall.EPOLLERR) != 0 {
				if events[i].Events&syscall.EPOLLIN != 0 {
					readAndEnqueue(ctx, worker, buf)
				}

				closeConn(listener, worker)

				continue
			}

			if events[i].Events&syscall.EPOLLIN != 0 {
				if closed := readAndEnqueue(ctx, worker, buf); closed {
					closeConn(listener, worker)
				}
			}
		}
	}
}

// accept accepts a new connection and registers it with the epoll instance.
func accept(_ context.Context, listener *sctp.SCTPListener) {
	conn, err := listener.Accept()
	if err != nil {
		if err != syscall.EAGAIN && err != syscall.EINTR {
			logger.AmfLog.Error("Failed to accept", zap.Error(err))
		}

		return
	}

	sctpEvents := sctp.SCTPEventDataIO | sctp.SCTPEventShutdown | sctp.SCTPEventAssociation
	if err := conn.SubscribeEvents(sctpEvents); err != nil {
		logger.AmfLog.Error("Failed to subscribe to SCTP events", zap.Error(err))

		if err := conn.Close(); err != nil {
			logger.AmfLog.Error("close error", zap.Error(err))
		}

		return
	}

	if err := conn.SetReadBuffer(int(readBufSize)); err != nil {
		logger.AmfLog.Error("Set read buffer error", zap.Error(err))

		if err := conn.Close(); err != nil {
			logger.AmfLog.Error("close error", zap.Error(err))
		}

		return
	}

	remoteAddr := conn.RemoteAddr()
	if remoteAddr == nil {
		logger.AmfLog.Error("Remote address is nil")

		if err := conn.Close(); err != nil {
			logger.AmfLog.Error("close error", zap.Error(err))
		}

		return
	}

	if err := listener.AddConnToEpoll(conn.Fd()); err != nil {
		logger.AmfLog.Error("Failed to add connection to epoll", zap.Error(err))

		if err := conn.Close(); err != nil {
			logger.AmfLog.Error("close error", zap.Error(err))
		}

		return
	}

	worker := newConnWorker(conn)
	connections.Store(conn.Fd(), worker)
	logger.AmfLog.Info("New SCTP connection", zap.String("remote_address", remoteAddr.String()))
}

// readAndEnqueue reads one message from the connection and enqueues it for dispatch.
// Returns true if the connection should be closed.
func readAndEnqueue(ctx context.Context, worker *connWorker, buf []byte) bool {
	n, info, notification, err := worker.conn.SCTPRead(buf)
	if err != nil {
		switch err {
		case io.EOF, io.ErrUnexpectedEOF:
			return true
		case syscall.EAGAIN, syscall.EINTR:
			return false
		case syscall.EINVAL, syscall.EBADF:
			return true
		default:
			logger.AmfLog.Error("SCTPRead error", zap.Error(err))
			return true
		}
	}

	if notification != nil {
		ngap.HandleSCTPNotification(worker.conn, notification)
		return false
	}

	if info == nil || networkToNativeEndianness32(info.PPID) != sctp.NGAPPPID {
		logger.AmfLog.Warn("Received SCTP PPID != 60, discard this packet")
		return false
	}

	worker.enqueue(ctx, buf[:n])

	return false
}

func closeConn(listener *sctp.SCTPListener, worker *connWorker) {
	fd := worker.conn.Fd()
	_ = listener.RemoveConnFromEpoll(fd) // epfd may already be closed during Stop()
	connections.Delete(fd)
	close(worker.msgCh)

	if err := worker.conn.Close(); err != nil && err != syscall.EBADF {
		logger.AmfLog.Error("close connection error", zap.Error(err))
	}
}

func Stop() {
	if err := sctpListener.Close(); err != nil {
		logger.AmfLog.Error("could not close sctp listener", zap.Error(err))
	}

	connections.Range(func(key, value any) bool {
		worker := value.(*connWorker)
		close(worker.msgCh)

		if err := worker.conn.Close(); err != nil && err != syscall.EBADF {
			logger.AmfLog.Error("close connection error", zap.Error(err))
		}

		return true
	})

	wg.Wait()
	logger.AmfLog.Info("SCTP server closed")
}

// networkToNativeEndianness32 converts a uint32 from network byte order to native byte order.
func networkToNativeEndianness32(value uint32) uint32 {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], value)

	return binary.NativeEndian.Uint32(b[:])
}
