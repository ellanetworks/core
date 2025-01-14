// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"io"
	"net"
	"sync"
	"syscall"

	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/ngap"
)

type NGAPHandler struct {
	HandleMessage      func(conn net.Conn, msg []byte)
	HandleNotification func(conn net.Conn, notification sctp.Notification)
}

const readBufSize uint32 = 131072

// set default read timeout to 2 seconds
var readTimeout syscall.Timeval = syscall.Timeval{Sec: 2, Usec: 0}

var (
	sctpListener *sctp.SCTPListener
	connections  sync.Map
)

var sctpConfig sctp.SocketConfig = sctp.SocketConfig{
	InitMsg:   sctp.InitMsg{NumOstreams: 3, MaxInstreams: 5, MaxAttempts: 2, MaxInitTimeout: 2},
	RtoInfo:   &sctp.RtoInfo{SrtoAssocID: 0, SrtoInitial: 500, SrtoMax: 1500, StroMin: 100},
	AssocInfo: &sctp.AssocInfo{AsocMaxRxt: 4},
}

func Run(addr string, port uint16, handler NGAPHandler) {
	netAddr, err := net.ResolveIPAddr("ip", addr)
	if err != nil {
		logger.AmfLog.Errorf("Error resolving address '%s': %v\n", addr, err)
	}
	ips := []net.IPAddr{
		{
			IP: netAddr.IP,
		},
	}
	sctpAddr := &sctp.SCTPAddr{
		IPAddrs: ips,
		Port:    port,
	}
	go listenAndServe(sctpAddr, handler)
}

func listenAndServe(addr *sctp.SCTPAddr, handler NGAPHandler) {
	if listener, err := sctpConfig.Listen("sctp", addr); err != nil {
		logger.AmfLog.Errorf("Failed to listen: %+v", err)
		return
	} else {
		sctpListener = listener
	}

	logger.AmfLog.Infof("NGAP server started on %s", addr.String())

	for {
		newConn, err := sctpListener.AcceptSCTP()
		if err != nil {
			switch err {
			case syscall.EINTR, syscall.EAGAIN:
				logger.AmfLog.Debugf("AcceptSCTP: %+v", err)
			default:
				logger.AmfLog.Errorf("Failed to accept: %+v", err)
			}
			continue
		}

		var info *sctp.SndRcvInfo
		if infoTmp, err := newConn.GetDefaultSentParam(); err != nil {
			logger.AmfLog.Errorf("Get default sent param error: %+v, accept failed", err)
			if err = newConn.Close(); err != nil {
				logger.AmfLog.Errorf("Close error: %+v", err)
			}
			continue
		} else {
			info = infoTmp
			logger.AmfLog.Debugf("Get default sent param[value: %+v]", info)
		}

		info.PPID = ngap.PPID
		if err := newConn.SetDefaultSentParam(info); err != nil {
			logger.AmfLog.Errorf("Set default sent param error: %+v, accept failed", err)
			if err = newConn.Close(); err != nil {
				logger.AmfLog.Errorf("Close error: %+v", err)
			}
			continue
		} else {
			logger.AmfLog.Debugf("Set default sent param[value: %+v]", info)
		}

		events := sctp.SCTP_EVENT_DATA_IO | sctp.SCTP_EVENT_SHUTDOWN | sctp.SCTP_EVENT_ASSOCIATION
		if err := newConn.SubscribeEvents(events); err != nil {
			logger.AmfLog.Errorf("Failed to accept: %+v", err)
			if err = newConn.Close(); err != nil {
				logger.AmfLog.Errorf("Close error: %+v", err)
			}
			continue
		} else {
			logger.AmfLog.Debugln("Subscribe SCTP event[DATA_IO, SHUTDOWN_EVENT, ASSOCIATION_CHANGE]")
		}

		if err := newConn.SetReadBuffer(int(readBufSize)); err != nil {
			logger.AmfLog.Errorf("Set read buffer error: %+v, accept failed", err)
			if err = newConn.Close(); err != nil {
				logger.AmfLog.Errorf("Close error: %+v", err)
			}
			continue
		} else {
			logger.AmfLog.Debugf("Set read buffer to %d bytes", readBufSize)
		}

		if err := newConn.SetReadTimeout(readTimeout); err != nil {
			logger.AmfLog.Errorf("Set read timeout error: %+v, accept failed", err)
			if err = newConn.Close(); err != nil {
				logger.AmfLog.Errorf("Close error: %+v", err)
			}
			continue
		} else {
			logger.AmfLog.Debugf("Set read timeout: %+v", readTimeout)
		}

		logger.AmfLog.Infof("[AMF] SCTP Accept from: %s", newConn.RemoteAddr().String())
		connections.Store(newConn, newConn)

		go handleConnection(newConn, readBufSize, handler)
	}
}

func Stop() {
	logger.AmfLog.Infof("Close SCTP server...")
	if err := sctpListener.Close(); err != nil {
		logger.AmfLog.Error(err)
		logger.AmfLog.Infof("SCTP server may not close normally.")
	}

	connections.Range(func(key, value interface{}) bool {
		conn := value.(net.Conn)
		if err := conn.Close(); err != nil {
			logger.AmfLog.Error(err)
		}
		return true
	})

	logger.AmfLog.Infof("SCTP server closed")
}

func handleConnection(conn *sctp.SCTPConn, bufsize uint32, handler NGAPHandler) {
	defer func() {
		// if AMF call Stop(), then conn.Close() will return EBADF because conn has been closed inside Stop()
		if err := conn.Close(); err != nil && err != syscall.EBADF {
			logger.AmfLog.Errorf("close connection error: %+v", err)
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
				logger.AmfLog.Debugf("SCTPRead: %+v", err)
				continue
			default:
				logger.AmfLog.Errorf("Handle connection[addr: %+v] error: %+v", conn.RemoteAddr(), err)
				return
			}
		}

		if notification != nil {
			if handler.HandleNotification != nil {
				handler.HandleNotification(conn, notification)
			} else {
				logger.AmfLog.Warnf("Received sctp notification[type 0x%x] but not handled", notification.Type())
			}
		} else {
			if info == nil || info.PPID != ngap.PPID {
				logger.AmfLog.Warnln("Received SCTP PPID != 60, discard this packet")
				continue
			}

			handler.HandleMessage(conn, buf[:n])
		}
	}
}
