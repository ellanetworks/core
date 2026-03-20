// Copyright 2026 Ella Networks
//go:build linux && !386

// Copyright 2019 Wataru Ishida. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sctp

import (
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
	"unsafe"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func setsockopt(fd int, optname, optval, optlen uintptr) error {
	_, _, errno := syscall.Syscall6(syscall.SYS_SETSOCKOPT,
		uintptr(fd),
		SolSCTP,
		optname,
		optval,
		optlen,
		0)
	if errno != 0 {
		return errno
	}

	return nil
}

func getsockopt(fd int, optname, optval, optlen uintptr) error {
	_, _, errno := syscall.Syscall6(syscall.SYS_GETSOCKOPT,
		uintptr(fd),
		SolSCTP,
		optname,
		optval,
		optlen,
		0)
	if errno != 0 {
		return errno
	}

	return nil
}

// listenRawConn is a minimal syscall.RawConn used only during listener socket
// setup (before the socket is wrapped in os.File). Read and Write panic
// because they should never be called during setup.
type listenRawConn struct {
	sockfd int
}

func (r listenRawConn) Control(f func(fd uintptr)) error {
	f(uintptr(r.sockfd))
	return nil
}

func (r listenRawConn) Read(func(fd uintptr) (done bool)) error {
	panic("listenRawConn: Read not supported")
}

func (r listenRawConn) Write(func(fd uintptr) (done bool)) error {
	panic("listenRawConn: Write not supported")
}

// WriteMsg sends data with optional SCTP ancillary info. Uses Go's runtime
// poller: the goroutine parks efficiently when the socket is not ready for
// writing, rather than pinning an OS thread.
func (c *SCTPConn) WriteMsg(b []byte, info *SndRcvInfo) (int, error) {
	if c.rc == nil {
		return 0, syscall.EBADF
	}

	var cbuf []byte

	if info != nil {
		cmsgBuf := toBuf(info)
		hdr := &syscall.Cmsghdr{
			Level: syscall.IPPROTO_SCTP,
			Type:  SCTPCMsgSndRcv,
		}
		hdr.SetLen(syscall.CmsgSpace(len(cmsgBuf)))
		cbuf = append(toBuf(hdr), cmsgBuf...)
	}

	var n int

	var err error

	werr := c.rc.Write(func(fd uintptr) bool {
		n, err = syscall.SendmsgN(int(fd), b, cbuf, nil, 0)
		return err != syscall.EAGAIN
	})
	if werr != nil {
		return 0, werr
	}

	return n, err
}

func parseSndRcvInfo(b []byte) (*SndRcvInfo, error) {
	msgs, err := syscall.ParseSocketControlMessage(b)
	if err != nil {
		return nil, err
	}

	for _, m := range msgs {
		if m.Header.Level == syscall.IPPROTO_SCTP {
			switch m.Header.Type {
			case SCTPCMsgSndRcv:
				return (*SndRcvInfo)(unsafe.Pointer(&m.Data[0])), nil
			}
		}
	}

	return nil, nil
}

func parseNotification(b []byte) Notification {
	snType := SCTPNotificationType(nativeEndian.Uint16(b[:2]))

	switch snType {
	case SCTPShutdownEvent:
		notification := SCTPShutdownEventNotification{
			sseType:    nativeEndian.Uint16(b[:2]),
			sseFlags:   nativeEndian.Uint16(b[2:4]),
			sseLength:  nativeEndian.Uint32(b[4:8]),
			sseAssocID: SCTPAssocID(nativeEndian.Uint32(b[8:])),
		}

		return &notification
	case SCTPAssocChange:
		notification := SCTPAssocChangeEvent{
			sacType:            nativeEndian.Uint16(b[:2]),
			sacFlags:           nativeEndian.Uint16(b[2:4]),
			sacLength:          nativeEndian.Uint32(b[4:8]),
			sacState:           SCTPState(nativeEndian.Uint16(b[8:10])),
			sacError:           nativeEndian.Uint16(b[10:12]),
			sacOutboundStreams: nativeEndian.Uint16(b[12:14]),
			sacInboundStreams:  nativeEndian.Uint16(b[14:16]),
			sacAssocID:         SCTPAssocID(nativeEndian.Uint32(b[16:20])),
			sacInfo:            b[20:],
		}

		return &notification
	default:
		return nil
	}
}

// ReadMsg receives an SCTP message and returns the data, optional SndRcvInfo,
// and any notification. Uses Go's runtime poller: the goroutine parks
// efficiently when the socket has no data, rather than blocking an OS thread.
func (c *SCTPConn) ReadMsg(b []byte) (int, *SndRcvInfo, Notification, error) {
	if c.rc == nil {
		return 0, nil, nil, syscall.EBADF
	}

	var oob [254]byte

	var n, oobn, recvflags int

	var err error

	rerr := c.rc.Read(func(fd uintptr) bool {
		n, oobn, recvflags, _, err = syscall.Recvmsg(int(fd), b, oob[:], 0)
		return err != syscall.EAGAIN
	})
	if rerr != nil {
		return 0, nil, nil, rerr
	}

	if err != nil {
		return n, nil, nil, err
	}

	if n == 0 && oobn == 0 {
		return 0, nil, nil, io.EOF
	}

	if recvflags&MsgNotification > 0 {
		notification := parseNotification(b[:n])
		return n, nil, notification, nil
	}

	var info *SndRcvInfo

	if oobn > 0 {
		info, err = parseSndRcvInfo(oob[:oobn])
	}

	return n, info, nil, err
}

// Close sends a graceful SCTP EOF to the peer and closes the underlying file
// descriptor. Any goroutine blocked in ReadMsg or WriteMsg is safely unparked
// by the runtime poller. Close is safe for concurrent use; the second and
// subsequent calls return syscall.EBADF.
func (c *SCTPConn) Close() error {
	if c == nil || c.file == nil {
		return net.ErrClosed
	}

	if !c.closed.CompareAndSwap(false, true) {
		return net.ErrClosed
	}

	// Send SCTP EOF to notify the peer of graceful shutdown. Control() holds
	// a reference to the fd, preventing the actual close(2) until it returns.
	_ = c.rc.Control(func(fd uintptr) {
		info := &SndRcvInfo{Flags: SCTPEof}
		cmsgBuf := toBuf(info)
		hdr := &syscall.Cmsghdr{
			Level: syscall.IPPROTO_SCTP,
			Type:  SCTPCMsgSndRcv,
		}
		hdr.SetLen(syscall.CmsgSpace(len(cmsgBuf)))
		cbuf := append(toBuf(hdr), cmsgBuf...)
		_, _ = syscall.SendmsgN(int(fd), nil, cbuf, nil, 0)
		_ = syscall.Shutdown(int(fd), syscall.SHUT_RDWR)
	})

	// Close the file. The runtime poller evicts all waiters first, unparking
	// any goroutines blocked in ReadMsg/WriteMsg, before the fd is closed.
	return c.file.Close()
}

func (c *SCTPConn) SetWriteBuffer(bytes int) error {
	return c.controlFd(func(fd int) error {
		return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_SNDBUF, bytes)
	})
}

func (c *SCTPConn) GetWriteBuffer() (int, error) {
	var val int

	err := c.controlFd(func(fd int) error {
		var e error

		val, e = syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_SNDBUF)

		return e
	})

	return val, err
}

func (c *SCTPConn) SetReadBuffer(bytes int) error {
	return c.controlFd(func(fd int) error {
		return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, bytes)
	})
}

func (c *SCTPConn) GetReadBuffer() (int, error) {
	var val int

	err := c.controlFd(func(fd int) error {
		var e error

		val, e = syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF)

		return e
	})

	return val, err
}

func (c *SCTPConn) GetRtoInfo() (*RtoInfo, error) {
	var info *RtoInfo

	err := c.controlFd(func(fd int) error {
		var e error

		info, e = getRtoInfo(fd)

		return e
	})

	return info, err
}

func (c *SCTPConn) SetRtoInfo(rtoInfo RtoInfo) error {
	return c.controlFd(func(fd int) error {
		return setRtoInfo(fd, rtoInfo)
	})
}

func (c *SCTPConn) GetAssocInfo() (*AssocInfo, error) {
	var info *AssocInfo

	err := c.controlFd(func(fd int) error {
		var e error

		info, e = getAssocInfo(fd)

		return e
	})

	return info, err
}

func (c *SCTPConn) SetAssocInfo(info AssocInfo) error {
	return c.controlFd(func(fd int) error {
		return setAssocInfo(fd, info)
	})
}

// listenSCTPExtConfig starts an SCTP listener on the specified address/port
// with the given SCTP options. The listener integrates with Go's runtime
// poller via os.NewFile, enabling safe concurrent Accept/Close without manual
// epoll or wakeup pipes.
func listenSCTPExtConfig(network string, laddr *SCTPAddr, options InitMsg, rtoInfo *RtoInfo, assocInfo *AssocInfo, control func(network, address string, c syscall.RawConn) error) (*SCTPListener, error) {
	af, ipv6only := favoriteAddrFamily(network, laddr, nil, "listen")

	sock, err := syscall.Socket(
		af,
		syscall.SOCK_STREAM|syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC,
		syscall.IPPROTO_SCTP,
	)
	if err != nil {
		return nil, err
	}

	// close socket on error
	defer func() {
		if err != nil {
			if cerr := syscall.Close(sock); cerr != nil {
				logger.AmfLog.Warn("failed to close socket", zap.Error(cerr))
			}
		}
	}()

	if err = setDefaultSockopts(sock, af, ipv6only); err != nil {
		return nil, err
	}

	if err = syscall.SetsockoptInt(sock, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return nil, err
	}

	if control != nil {
		rc := listenRawConn{sockfd: sock}
		if err = control(network, laddr.String(), rc); err != nil {
			return nil, err
		}
	}

	if rtoInfo != nil {
		if err = setRtoInfo(sock, *rtoInfo); err != nil {
			return nil, err
		}
	}

	if assocInfo != nil {
		if err = setAssocInfo(sock, *assocInfo); err != nil {
			return nil, err
		}
	}

	if err = setInitOpts(sock, options); err != nil {
		return nil, err
	}

	if laddr != nil {
		if len(laddr.IPAddrs) == 0 {
			switch af {
			case syscall.AF_INET:
				laddr.IPAddrs = append(laddr.IPAddrs, net.IPAddr{IP: net.IPv4zero})
			case syscall.AF_INET6:
				laddr.IPAddrs = append(laddr.IPAddrs, net.IPAddr{IP: net.IPv6zero})
			}
		}

		if err = SCTPBind(sock, laddr, SCTPBindxAddAddr); err != nil {
			return nil, err
		}
	}

	if err = syscall.Listen(sock, syscall.SOMAXCONN); err != nil {
		return nil, err
	}

	// Wrap the listener socket in os.File. Because the socket was created with
	// SOCK_NONBLOCK, os.NewFile detects the non-blocking flag and registers the
	// fd with Go's runtime poller. This enables Accept to park the goroutine
	// efficiently and Close to safely wake it up.
	f := os.NewFile(uintptr(sock), "sctp-listener")
	if f == nil {
		return nil, fmt.Errorf("os.NewFile returned nil for fd %d", sock)
	}

	rc, err := f.SyscallConn()
	if err != nil {
		_ = f.Close()

		return nil, fmt.Errorf("SyscallConn: %w", err)
	}

	// The fd is now owned by f; prevent the deferred cleanup from closing it.
	sock = -1

	return &SCTPListener{file: f, rc: rc}, nil
}

// Accept waits for an incoming SCTP connection. It uses Go's runtime poller:
// the goroutine parks efficiently until a connection is ready. When Close is
// called, the poller wakes Accept which returns an error wrapping
// net.ErrClosed, mirroring the behaviour of Go's net.Listener.
func (ln *SCTPListener) Accept() (*SCTPConn, error) {
	var newFd int

	var err error

	rerr := ln.rc.Read(func(fd uintptr) bool {
		newFd, _, err = syscall.Accept4(int(fd), syscall.SOCK_CLOEXEC|syscall.SOCK_NONBLOCK)
		if err == syscall.EAGAIN {
			return false // not ready; tell poller to park and retry
		}

		return true
	})
	if rerr != nil {
		return nil, rerr
	}

	if err != nil {
		return nil, err
	}

	conn := NewSCTPConn(newFd)
	if conn == nil {
		_ = syscall.Close(newFd)
		return nil, fmt.Errorf("failed to wrap accepted fd %d", newFd)
	}

	return conn, nil
}

// Close closes the listener and unblocks any concurrent Accept call. The
// runtime poller safely wakes all parked goroutines before the file descriptor
// is closed, avoiding the race that existed with manual epoll.
func (ln *SCTPListener) Close() error {
	if !ln.closed.CompareAndSwap(false, true) {
		return net.ErrClosed
	}
	// Shutdown the socket so any in-flight associations are cleanly aborted,
	// then close the file which unparks Accept via the runtime poller.
	_ = ln.rc.Control(func(fd uintptr) {
		_ = syscall.Shutdown(int(fd), syscall.SHUT_RDWR)
	})

	return ln.file.Close()
}
