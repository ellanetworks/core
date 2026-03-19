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
	"io"
	"net"
	"sync/atomic"
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

type rawConn struct {
	sockfd int
}

func (r rawConn) Control(f func(fd uintptr)) error {
	f(uintptr(r.sockfd))
	return nil
}

func (r rawConn) Read(f func(fd uintptr) (done bool)) error {
	panic("not implemented")
}

func (r rawConn) Write(f func(fd uintptr) (done bool)) error {
	panic("not implemented")
}

func (c *SCTPConn) SCTPWrite(b []byte, info *SndRcvInfo) (int, error) {
	var cbuf []byte

	if info != nil {
		cmsgBuf := toBuf(info)
		hdr := &syscall.Cmsghdr{
			Level: syscall.IPPROTO_SCTP,
			Type:  SCTPCMsgSndRcv,
		}

		// bitwidth of hdr.Len is platform-specific,
		// so we use hdr.SetLen() rather than directly setting hdr.Len
		hdr.SetLen(syscall.CmsgSpace(len(cmsgBuf)))
		cbuf = append(toBuf(hdr), cmsgBuf...)
	}

	return syscall.SendmsgN(c.fd(), b, cbuf, nil, 0)
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

// SCTPRead use syscall.Recvmsg to receive SCTP message and return sctp sndrcvinfo/notification if need
func (c *SCTPConn) SCTPRead(b []byte) (int, *SndRcvInfo, Notification, error) {
	var oob [254]byte

	n, oobn, recvflags, _, err := syscall.Recvmsg(c.fd(), b, oob[:], 0)
	if err != nil {
		return n, nil, nil, err
	}

	if n == 0 && oobn == 0 {
		return 0, nil, nil, io.EOF
	}

	if recvflags&MsgNotification > 0 {
		notification := parseNotification(b[:n])
		return n, nil, notification, nil
	} else {
		var info *SndRcvInfo
		if oobn > 0 {
			info, err = parseSndRcvInfo(oob[:oobn])
		}

		return n, info, nil, err
	}
}

func (c *SCTPConn) Close() error {
	if c == nil {
		return syscall.EBADF
	}

	fd := atomic.SwapInt32(&c._fd, -1)
	if fd <= 0 {
		return syscall.EBADF
	}

	// Send SCTP EOF using the saved fd directly. c.SCTPWrite() calls c.fd()
	// which reads the atomic _fd (now -1 after the swap above), so it would
	// silently fail with EBADF and the peer would never receive the EOF.
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

	return syscall.Close(int(fd))
}

func (c *SCTPConn) SetWriteBuffer(bytes int) error {
	return syscall.SetsockoptInt(c.fd(), syscall.SOL_SOCKET, syscall.SO_SNDBUF, bytes)
}

func (c *SCTPConn) GetWriteBuffer() (int, error) {
	return syscall.GetsockoptInt(c.fd(), syscall.SOL_SOCKET, syscall.SO_SNDBUF)
}

func (c *SCTPConn) SetReadBuffer(bytes int) error {
	return syscall.SetsockoptInt(c.fd(), syscall.SOL_SOCKET, syscall.SO_RCVBUF, bytes)
}

func (c *SCTPConn) GetReadBuffer() (int, error) {
	return syscall.GetsockoptInt(c.fd(), syscall.SOL_SOCKET, syscall.SO_RCVBUF)
}

func (c *SCTPConn) SetWriteTimeout(tv syscall.Timeval) error {
	return syscall.SetsockoptTimeval(c.fd(), syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, &tv)
}

func (c *SCTPConn) SetReadTimeout(tv syscall.Timeval) error {
	return syscall.SetsockoptTimeval(c.fd(), syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)
}

func (c *SCTPConn) SetNonBlock(nonBlock bool) error {
	return syscall.SetNonblock(c.fd(), nonBlock)
}

func (c *SCTPConn) GetRtoInfo() (*RtoInfo, error) {
	return getRtoInfo(c.fd())
}

func (c *SCTPConn) SetRtoInfo(rtoInfo RtoInfo) error {
	return setRtoInfo(c.fd(), rtoInfo)
}

func (c *SCTPConn) GetAssocInfo() (*AssocInfo, error) {
	return getAssocInfo(c.fd())
}

func (c *SCTPConn) SetAssocInfo(info AssocInfo) error {
	return setAssocInfo(c.fd(), info)
}

// listenSCTPExtConfig starts a listener on the specified address/port with the
// given SCTP options. The listening socket is blocking; Accept will block until
// a connection arrives.
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
			err := syscall.Close(sock)
			if err != nil {
				logger.AmfLog.Warn("failed to close socket", zap.Error(err))
			}
		}
	}()

	if err = setDefaultSockopts(sock, af, ipv6only); err != nil {
		return nil, err
	}

	// enable REUSEADDR option
	if err = syscall.SetsockoptInt(sock, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return nil, err
	}

	if control != nil {
		rc := rawConn{sockfd: sock}
		if err = control(network, laddr.String(), rc); err != nil {
			return nil, err
		}
	}

	// RTO
	if rtoInfo != nil {
		err = setRtoInfo(sock, *rtoInfo)
		if err != nil {
			return nil, err
		}
	}

	// set default association parameters (RFC 6458 8.1.2)
	if assocInfo != nil {
		err = setAssocInfo(sock, *assocInfo)
		if err != nil {
			return nil, err
		}
	}

	err = setInitOpts(sock, options)
	if err != nil {
		return nil, err
	}

	if laddr != nil {
		// If IP address and/or port was not provided so far, let's use the unspecified IPv4 or IPv6 address
		if len(laddr.IPAddrs) == 0 {
			switch af {
			case syscall.AF_INET:
				laddr.IPAddrs = append(laddr.IPAddrs, net.IPAddr{IP: net.IPv4zero})
			case syscall.AF_INET6:
				laddr.IPAddrs = append(laddr.IPAddrs, net.IPAddr{IP: net.IPv6zero})
			}
		}

		err := SCTPBind(sock, laddr, SCTPBindxAddAddr)
		if err != nil {
			return nil, err
		}
	}

	err = syscall.Listen(sock, syscall.SOMAXCONN)
	if err != nil {
		return nil, err
	}

	epfd, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		return nil, err
	}

	var wfds [2]int
	if err = syscall.Pipe2(wfds[:], syscall.O_NONBLOCK|syscall.O_CLOEXEC); err != nil {
		_ = syscall.Close(epfd)
		return nil, err
	}

	wakeR, wakeW := wfds[0], wfds[1]

	// Register the listener socket: fires when a connection is waiting.
	if err = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, sock, &syscall.EpollEvent{Events: syscall.EPOLLIN}); err != nil {
		_ = syscall.Close(epfd)
		_ = syscall.Close(wakeR)
		_ = syscall.Close(wakeW)

		return nil, err
	}

	// Register the wakeup pipe: fires when Close writes to wakeW.
	if err = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, wakeR, &syscall.EpollEvent{Events: syscall.EPOLLIN}); err != nil {
		_ = syscall.Close(epfd)
		_ = syscall.Close(wakeR)
		_ = syscall.Close(wakeW)

		return nil, err
	}

	return &SCTPListener{fd: sock, epfd: epfd, wakeR: wakeR, wakeW: wakeW}, nil
}

// Accept waits for an incoming connection using epoll and a wakeup pipe.
// It returns promptly with syscall.EBADF when Close is called, mirroring the
// behaviour of Go's net.Listener and avoiding the OS-level race where closing
// a file descriptor from another goroutine does not reliably unblock a
// concurrent blocking accept(2) call.
func (ln *SCTPListener) Accept() (*SCTPConn, error) {
	var (
		events  [2]syscall.EpollEvent
		oneByte [1]byte
	)

	for {
		n, err := syscall.EpollWait(ln.epfd, events[:], -1)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			// epfd was closed by Close() (EBADF) or another unexpected error.
			return nil, err
		}

		_ = n

		// Non-blocking drain of the wakeup pipe. A successful read (err == nil)
		// or any error other than EAGAIN means Close() has been called.
		_, pipeErr := syscall.Read(ln.wakeR, oneByte[:])
		if pipeErr != syscall.EAGAIN {
			return nil, syscall.EBADF
		}

		// Wakeup pipe was empty — a connection must be ready on the listener.
		fd, _, err := syscall.Accept4(ln.fd, syscall.SOCK_CLOEXEC)
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EINTR {
				// Spurious wakeup; loop back to EpollWait.
				continue
			}

			return nil, err
		}

		return NewSCTPConn(fd, nil), nil
	}
}

func (ln *SCTPListener) Close() error {
	// Write to the wakeup pipe first so that any goroutine blocked in
	// EpollWait sees wakeR become readable and returns immediately.
	_, _ = syscall.Write(ln.wakeW, []byte{1})

	_ = syscall.Shutdown(ln.fd, syscall.SHUT_RDWR)
	listenerErr := syscall.Close(ln.fd)

	// Close the epoll fd and both pipe ends. Any concurrent EpollWait will
	// return EBADF, which Accept treats identically to the wakeup-pipe signal.
	_ = syscall.Close(ln.epfd)
	_ = syscall.Close(ln.wakeR)
	_ = syscall.Close(ln.wakeW)

	return listenerErr
}
