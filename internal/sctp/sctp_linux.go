// SPDX-FileCopyrightText: Ella Networks Inc.
//go:build linux && !386

// Copyright 2019 Wataru Ishida. All rights reserved.
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
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
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func setsockopt(fd int, optname uintptr, optval unsafe.Pointer, optlen uintptr) error {
	_, _, errno := syscall.Syscall6(syscall.SYS_SETSOCKOPT,
		uintptr(fd),
		solSCTP,
		optname,
		uintptr(optval),
		optlen,
		0)
	if errno != 0 {
		return errno
	}

	return nil
}

func getsockopt(fd int, optname uintptr, optval, optlen unsafe.Pointer) error {
	_, _, errno := syscall.Syscall6(syscall.SYS_GETSOCKOPT,
		uintptr(fd),
		solSCTP,
		optname,
		uintptr(optval),
		uintptr(optlen),
		0)
	if errno != 0 {
		return errno
	}

	return nil
}

// listenRawConn is a minimal syscall.RawConn used only during listener socket
// setup (before the socket is wrapped in os.File). It supports only Control;
// Read and Write return an error rather than panic, since they are never part of
// listener setup but must not crash the process if a future path reaches them.
type listenRawConn struct {
	sockfd int
}

func (r listenRawConn) Control(f func(fd uintptr)) error {
	f(uintptr(r.sockfd))
	return nil
}

func (r listenRawConn) Read(func(fd uintptr) (done bool)) error {
	return fmt.Errorf("sctp: Read not supported on a listener control connection")
}

func (r listenRawConn) Write(func(fd uintptr) (done bool)) error {
	return fmt.Errorf("sctp: Write not supported on a listener control connection")
}

// retryableIO reports an error that means "nothing happened, go round again":
// EAGAIN once the poller reports readiness, and EINTR when a signal arrives
// mid-syscall, which acceptWouldBlock treats the same way.
func retryableIO(err error) bool {
	return errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EINTR)
}

// writeMsgSync sends one message. Accepted associations funnel through the
// writer goroutine; dialled ones call it from the caller's goroutine, which is
// safe because RawConn.Write serialises on the descriptor's write lock.
func (c *SCTPConn) writeMsgSync(b []byte, info *SndRcvInfo) (int, error) {
	if c.rc == nil {
		return 0, syscall.EBADF
	}

	var cbuf []byte

	if info != nil {
		cmsgBuf := toBuf(info)
		hdr := &syscall.Cmsghdr{
			Level: syscall.IPPROTO_SCTP,
			Type:  sctpCMsgSndRcv,
		}
		// The kernel requires exactly CMSG_LEN; CmsgSpace rounds up and would be
		// rejected for any payload size that is not already alignment-sized.
		hdr.SetLen(syscall.CmsgLen(len(cmsgBuf)))
		cbuf = append(toBuf(hdr), cmsgBuf...)
	}

	var n int

	var err error

	werr := c.rc.Write(func(fd uintptr) bool {
		n, err = syscall.SendmsgN(int(fd), b, cbuf, nil, 0)
		return !retryableIO(err)
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
			case sctpCMsgSndRcv:
				if len(m.Data) < int(unsafe.Sizeof(SndRcvInfo{})) {
					return nil, fmt.Errorf("sctp: short SNDRCV cmsg (%d bytes)", len(m.Data))
				}

				// Copy the struct out of the syscall control buffer so the
				// returned pointer does not alias storage the caller reuses.
				info := *(*SndRcvInfo)(unsafe.Pointer(&m.Data[0]))

				return &info, nil
			}
		}
	}

	return nil, nil
}

func parseNotification(b []byte) Notification {
	if len(b) < 2 {
		return nil
	}

	snType := SCTPNotificationType(binary.NativeEndian.Uint16(b[:2]))

	switch snType {
	case SCTPShutdownEvent:
		if len(b) < 12 {
			return nil
		}

		notification := SCTPShutdownEventNotification{
			sseType:    binary.NativeEndian.Uint16(b[:2]),
			sseFlags:   binary.NativeEndian.Uint16(b[2:4]),
			sseLength:  binary.NativeEndian.Uint32(b[4:8]),
			sseAssocID: SCTPAssocID(binary.NativeEndian.Uint32(b[8:])),
		}

		return &notification
	case SCTPAssocChange:
		if len(b) < 20 {
			return nil
		}

		notification := SCTPAssocChangeEvent{
			sacType:            binary.NativeEndian.Uint16(b[:2]),
			sacFlags:           binary.NativeEndian.Uint16(b[2:4]),
			sacLength:          binary.NativeEndian.Uint32(b[4:8]),
			sacState:           SCTPState(binary.NativeEndian.Uint16(b[8:10])),
			sacError:           binary.NativeEndian.Uint16(b[10:12]),
			sacOutboundStreams: binary.NativeEndian.Uint16(b[12:14]),
			sacInboundStreams:  binary.NativeEndian.Uint16(b[14:16]),
			sacAssocID:         SCTPAssocID(binary.NativeEndian.Uint32(b[16:20])),
			sacInfo:            append([]uint8(nil), b[20:]...),
		}

		return &notification
	case SCTPPartialDeliveryEvent:
		if len(b) < 12 {
			return nil
		}

		notification := SCTPPartialDeliveryEventNotification{
			pdapiType:       binary.NativeEndian.Uint16(b[:2]),
			pdapiFlags:      binary.NativeEndian.Uint16(b[2:4]),
			pdapiLength:     binary.NativeEndian.Uint32(b[4:8]),
			pdapiIndication: binary.NativeEndian.Uint32(b[8:12]),
		}

		return &notification
	default:
		return nil
	}
}

// delivery is one datagram handed up by recvmsg.
type delivery struct {
	n            int
	info         *SndRcvInfo
	notification Notification
	// isNotification distinguishes an event the caller must not treat as
	// payload from one parseNotification did not recognise.
	isNotification bool
	eor            bool
}

// readMsgOnce receives one delivery. eor reports whether it completes a message;
// the kernel clears MSG_EOR and requeues the remainder when a message does not
// fit the supplied buffer (net/sctp/socket.c sctp_recvmsg).
func (c *SCTPConn) readMsgOnce(b []byte) (delivery, error) {
	if c.rc == nil {
		return delivery{}, syscall.EBADF
	}

	var oob [254]byte

	var n, oobn, recvflags int

	var err error

	rerr := c.rc.Read(func(fd uintptr) bool {
		n, oobn, recvflags, _, err = syscall.Recvmsg(int(fd), b, oob[:], 0)
		return !retryableIO(err)
	})
	if rerr != nil {
		return delivery{}, rerr
	}

	if err != nil {
		return delivery{n: n}, err
	}

	if n == 0 && oobn == 0 {
		return delivery{}, io.EOF
	}

	eor := recvflags&syscall.MSG_EOR > 0

	if recvflags&msgNotification > 0 {
		return delivery{n: n, notification: parseNotification(b[:n]), isNotification: true, eor: eor}, nil
	}

	if recvflags&syscall.MSG_CTRUNC > 0 {
		return delivery{n: n}, fmt.Errorf("sctp: ancillary data truncated")
	}

	var info *SndRcvInfo

	if oobn > 0 {
		info, err = parseSndRcvInfo(oob[:oobn])
	}

	return delivery{n: n, info: info, eor: eor}, err
}

// readMsg receives one complete SCTP message into b, reassembling a message the
// kernel split across deliveries. A message that does not fit in b returns
// errMessageTooLarge, since silently dispatching a fragment would present it to
// the caller as a whole message.
func (c *SCTPConn) readMsg(b []byte) (int, *SndRcvInfo, Notification, error) {
	return reassemble(c.readMsgOnce, b)
}

// ReadMsg reads one complete message into b and returns its SCTP receive
// metadata. Events are consumed rather than surfaced; one that ends the
// association reports io.EOF. Server instead dispatches them via readMsg.
func (c *SCTPConn) ReadMsg(b []byte) (int, *SndRcvInfo, error) {
	for {
		n, info, notification, err := c.readMsg(b)
		if err != nil {
			return n, info, err
		}

		if notification == nil {
			return n, info, nil
		}

		switch event := notification.(type) {
		case *SCTPShutdownEventNotification:
			return 0, nil, io.EOF
		case *SCTPAssocChangeEvent:
			switch event.State() {
			// SCTPRestart belongs here: the peer re-INITed the same 5-tuple and
			// the kernel swapped the TCB underneath, so every stream, TSN and
			// piece of application state on the old association is gone
			// (net/sctp/sm_statefuns.c sctp_sf_do_dupcook_a). Continuing to read
			// would hand the caller a different association's traffic.
			case SCTPCommLost, SCTPShutdownComp, SCTPCantStrAssoc, SCTPRestart:
				return 0, nil, io.EOF
			}
		}
	}
}

func reassemble(read func([]byte) (delivery, error), b []byte) (int, *SndRcvInfo, Notification, error) {
	total := 0

	for {
		d, err := read(b[total:])
		if err != nil {
			return total + d.n, nil, nil, err
		}

		if d.isNotification {
			// Every subscribed event type is decoded, so an event that did not
			// parse — or that the kernel had to split — is not a shape this
			// association should be producing.
			if d.notification == nil || !d.eor {
				return 0, nil, nil, errUnrecognizedDelivery
			}

			// The kernel abandons a partially delivered message without ever
			// setting MSG_EOR and then splices the messages that arrived
			// meanwhile onto the queue (net/sctp/ulpqueue.c sctp_ulpq_abort_pd),
			// so the prefix read so far has to go.
			if pd, ok := d.notification.(*SCTPPartialDeliveryEventNotification); ok && pd.Aborted() {
				total = 0
				continue
			}

			// Any other event during reassembly means the kernel's ordering
			// guarantee no longer holds and the stream cannot be trusted.
			if total > 0 {
				return total, nil, nil, errUnexpectedNotification
			}

			return 0, nil, d.notification, nil
		}

		// A delivery that neither carries payload nor completes a message would
		// leave the loop with no way to make progress.
		if d.n == 0 && !d.eor {
			return total, nil, nil, errUnrecognizedDelivery
		}

		total += d.n

		if d.eor {
			return total, d.info, nil, nil
		}

		if total >= len(b) {
			return total, d.info, nil, errMessageTooLarge
		}
	}
}

// Close ends the association gracefully: queued sends are flushed, the SCTP
// shutdown handshake is started, and the receive queue is drained before the
// descriptor closes. The drain is required — the kernel turns the shutdown into
// an ABORT if any inbound data is still unread (net/sctp/socket.c sctp_close).
// Close is safe for concurrent use; the second and subsequent calls return
// net.ErrClosed.
func (c *SCTPConn) Close() error {
	if c == nil || c.file == nil {
		return net.ErrClosed
	}

	if !c.closed.CompareAndSwap(false, true) {
		return net.ErrClosed
	}

	c.flushWriter()

	// Control() holds a reference to the fd, preventing the actual close(2)
	// until it returns.
	_ = c.rc.Control(func(fd uintptr) {
		_ = syscall.Shutdown(int(fd), syscall.SHUT_RDWR)
	})

	// Bounded: the kernel drops newly arriving data once both shutdown bits are
	// set (net/sctp/ulpqueue.c), so only the pre-shutdown backlog is left.
	c.drainReceiveQueue()

	c.stopWriter()

	// The runtime poller evicts all waiters first, unparking any goroutines
	// blocked in readMsg/WriteMsg, before the fd is closed.
	return c.file.Close()
}

// Abort ends the association immediately with an SCTP ABORT, discarding queued
// sends. It is the right close for a peer that has stopped functioning, and it
// never waits for the writer, so the writer goroutine itself may call it.
func (c *SCTPConn) Abort() error {
	if c == nil || c.file == nil {
		return net.ErrClosed
	}

	if !c.closed.CompareAndSwap(false, true) {
		return net.ErrClosed
	}

	c.stopWriter()

	// SO_LINGER with a zero timeout makes close(2) emit an ABORT
	// (net/sctp/socket.c sctp_close).
	_ = c.controlFd(func(fd int) error {
		return syscall.SetsockoptLinger(fd, syscall.SOL_SOCKET, syscall.SO_LINGER, &syscall.Linger{Onoff: 1, Linger: 0})
	})

	return c.file.Close()
}

// drainReceiveQueue reads until the queue is empty or the deadline expires, so a
// peer still sending cannot hold up the close.
func (c *SCTPConn) drainReceiveQueue() {
	if err := c.setReadDeadline(time.Now().Add(drainTimeout)); err != nil {
		return
	}

	buf := make([]byte, drainBufSize)

	for {
		d, err := c.readMsgOnce(buf)
		if err != nil || d.n == 0 {
			return
		}
	}
}

func (c *SCTPConn) setReadBuffer(bytes int) error {
	return c.controlFd(func(fd int) error {
		return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, bytes)
	})
}

// listenSCTPExtConfig starts an SCTP listener on the specified address/port
// with the given SCTP options. The listener integrates with Go's runtime
// poller via os.NewFile, enabling safe concurrent Accept/Close without manual
// epoll or wakeup pipes.
func listenSCTPExtConfig(network string, laddr *SCTPAddr, options InitMsg, rtoInfo *rtoInfo, assocInfo *assocInfo, control func(network, address string, c syscall.RawConn) error) (*sctpListener, error) {
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

	// Inherited by accepted associations, like the options above.
	if err = setNoDelay(sock); err != nil {
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

		if err = sctpBind(sock, laddr, sctpBindxAddAddr); err != nil {
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
		err = fmt.Errorf("os.NewFile returned nil for fd %d", sock)

		return nil, err
	}

	// f owns the fd from here: clear sock so the deferred cleanup cannot close
	// the same descriptor a second time.
	sock = -1

	rc, err := f.SyscallConn()
	if err != nil {
		_ = f.Close()

		return nil, fmt.Errorf("SyscallConn: %w", err)
	}

	return &sctpListener{file: f, rc: rc}, nil
}

// Accept waits for an incoming SCTP connection. It uses Go's runtime poller:
// the goroutine parks efficiently until a connection is ready. When Close is
// called, the poller wakes Accept which returns an error wrapping
// net.ErrClosed, mirroring the behaviour of Go's net.Listener.
func (ln *sctpListener) Accept() (*SCTPConn, error) {
	var newFd int

	var err error

	rerr := ln.rc.Read(func(fd uintptr) bool {
		newFd, _, err = syscall.Accept4(int(fd), syscall.SOCK_CLOEXEC|syscall.SOCK_NONBLOCK)
		if acceptWouldBlock(err) {
			return false // not ready; tell poller to park and retry
		}

		return true
	})
	if rerr != nil {
		return nil, ln.acceptErr(rerr)
	}

	if err != nil {
		return nil, ln.acceptErr(err)
	}

	// newSCTPConn owns newFd whether or not it succeeds.
	conn := newSCTPConn(newFd)
	if conn == nil {
		return nil, fmt.Errorf("failed to wrap accepted fd %d", newFd)
	}

	return conn, nil
}

// SCTP answers a would-block accept with EINTR when a signal is pending:
// sctp_wait_for_accept tests signal_pending before the !timeo case
// (net/sctp/socket.c).
func acceptWouldBlock(err error) bool {
	return err == syscall.EAGAIN || err == syscall.EINTR
}

// acceptErr reports a closed listener as net.ErrClosed: the poller surfaces the
// close as a bare "use of closed file" that matches no exported sentinel.
func (ln *sctpListener) acceptErr(err error) error {
	if ln.closed.Load() {
		return net.ErrClosed
	}

	return err
}

// Close closes the listener and unblocks any concurrent Accept call. The
// runtime poller safely wakes all parked goroutines before the file descriptor
// is closed, avoiding the race that existed with manual epoll.
func (ln *sctpListener) Close() error {
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
