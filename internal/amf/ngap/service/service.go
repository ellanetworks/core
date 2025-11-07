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

	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/ngap"
	"go.uber.org/zap"
)

type NGAPHandler struct {
	HandleMessage      func(ctx context.Context, conn *sctp.SCTPConn, msg []byte)
	HandleNotification func(conn *sctp.SCTPConn, notification sctp.Notification)
}

const readBufSize uint32 = 131072

// set default read timeout to 2 seconds
var readTimeout syscall.Timeval = syscall.Timeval{Sec: 2, Usec: 0}

var (
	sctpListener *sctp.SCTPListener
	connections  sync.Map
)

var sctpConfig sctp.SocketConfig = sctp.SocketConfig{
	InitMsg:   sctp.InitMsg{NumOstreams: 2, MaxInstreams: 5, MaxAttempts: 2, MaxInitTimeout: 2},
	RtoInfo:   &sctp.RtoInfo{SrtoAssocID: 0, SrtoInitial: 500, SrtoMax: 1500, StroMin: 100},
	AssocInfo: &sctp.AssocInfo{AsocMaxRxt: 4},
}

func Run(address string, port int, handler NGAPHandler) error {
	ips := []net.IPAddr{}
	netAddr, err := net.ResolveIPAddr("ip", address)
	if err != nil {
		return fmt.Errorf("error resolving address '%s': %v", address, err)
	}
	ips = append(ips, *netAddr)

	addr := &sctp.SCTPAddr{
		IPAddrs: ips,
		Port:    port,
	}

	go listenAndServe(addr, handler)
	return nil
}

func listenAndServe(addr *sctp.SCTPAddr, handler NGAPHandler) {
	listener, err := sctpConfig.Listen("sctp", addr)
	if err != nil {
		logger.AmfLog.Error("Failed to listen", zap.Error(err))
		return
	}
	sctpListener = listener
	logger.AmfLog.Info("NGAP server started", zap.String("address", addr.String()))
	for {
		newConn, err := sctpListener.AcceptSCTP()
		if err != nil {
			switch err {
			case syscall.EINTR, syscall.EAGAIN:
				logger.AmfLog.Debug("AcceptSCTP", zap.Error(err))
			default:
				logger.AmfLog.Error("Failed to accept", zap.Error(err))
			}
			continue
		}

		events := sctp.SCTPEventDataIO | sctp.SCTPEventShutdown | sctp.SCTPEventAssociation
		if err := newConn.SubscribeEvents(events); err != nil {
			logger.AmfLog.Error("Failed to accept", zap.Error(err))
			if err = newConn.Close(); err != nil {
				logger.AmfLog.Error("Close error", zap.Error(err))
			}
			continue
		} else {
			logger.AmfLog.Debug("Subscribe SCTP event[DATA_IO, SHUTDOWN_EVENT, ASSOCIATION_CHANGE]")
		}

		if err := newConn.SetReadBuffer(int(readBufSize)); err != nil {
			logger.AmfLog.Error("Set read buffer error", zap.Error(err))
			if err = newConn.Close(); err != nil {
				logger.AmfLog.Error("Close error", zap.Error(err))
			}
			continue
		} else {
			logger.AmfLog.Debug("Set read buffer", zap.Any("size", readBufSize))
		}

		if err := newConn.SetReadTimeout(readTimeout); err != nil {
			logger.AmfLog.Error("Set read timeout error", zap.Error(err))
			if err = newConn.Close(); err != nil {
				logger.AmfLog.Error("Close error", zap.Error(err))
			}
			continue
		} else {
			logger.AmfLog.Debug("Set read timeout", zap.Any("timeout", readTimeout))
		}

		remoteAddress := newConn.RemoteAddr()
		if remoteAddress == nil {
			logger.AmfLog.Error("Remote address is nil")
			if err = newConn.Close(); err != nil {
				logger.AmfLog.Error("Close error", zap.Error(err))
			}
			continue
		}

		logger.AmfLog.Info("New SCTP connection", zap.String("remote_address", remoteAddress.String()))
		connections.Store(newConn, newConn)

		go handleConnection(newConn, readBufSize, handler)
	}
}

func Stop() {
	if err := sctpListener.Close(); err != nil {
		logger.AmfLog.Error("could not close sctp listener", zap.Error(err))
	}

	connections.Range(func(key, value any) bool {
		conn := value.(*sctp.SCTPConn)
		if err := conn.Close(); err != nil {
			logger.AmfLog.Error("close connection error", zap.Error(err))
		}
		return true
	})

	logger.AmfLog.Info("SCTP server closed")
}

func handleConnection(conn *sctp.SCTPConn, bufsize uint32, handler NGAPHandler) {
	defer func() {
		// if AMF call Stop(), then conn.Close() will return EBADF because conn has been closed inside Stop()
		if err := conn.Close(); err != nil && err != syscall.EBADF {
			logger.AmfLog.Error("close connection error", zap.Error(err))
		}
		connections.Delete(conn)
	}()

	for {
		buf := make([]byte, bufsize)

		n, info, notification, err := conn.SCTPRead(buf)
		if err != nil {
			switch err {
			case io.EOF, io.ErrUnexpectedEOF:
				return
			case syscall.EAGAIN:
				continue
			case syscall.EINTR:
				logger.AmfLog.Debug("SCTPRead", zap.Error(err))
				continue
			case syscall.EINVAL:
				logger.AmfLog.Error("Couldn't handle remotely closed connection", zap.Error(err))
				return
			default:
				logger.AmfLog.Error("Handle connection error", zap.Error(err))
				return
			}
		}

		if notification != nil {
			if handler.HandleNotification != nil {
				handler.HandleNotification(conn, notification)
			} else {
				logger.AmfLog.Warn("Received sctp notification but not handled", zap.Any("type", notification.Type()))
			}
		} else {
			if info == nil || networkToNativeEndianness32(info.PPID) != ngap.PPID {
				logger.AmfLog.Warn("Received SCTP PPID != 60, discard this packet")
				continue
			}

			handler.HandleMessage(context.Background(), conn, buf[:n])
		}
	}
}

// Takes a uint32 in Network Byte Order and returns
// in in Native Byte Order
func networkToNativeEndianness32(value uint32) uint32 {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], value)
	return binary.NativeEndian.Uint32(b[:])
}
