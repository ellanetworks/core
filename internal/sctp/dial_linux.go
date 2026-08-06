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
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// aLongTimeAgo unparks any goroutine waiting on the descriptor when installed
// as its deadline.
var aLongTimeAgo = time.Unix(1, 0)

// dialedEvents matches what serveConn subscribes to. PartialDelivery is
// required for correctness, not observability: without it the kernel abandons a
// partial message silently and the next message is read as its continuation.
const dialedEvents = sctpEventDataIO | sctpEventShutdown | sctpEventAssociation | sctpEventPartialDelivery

const assocChangeBufSize = 512

// getAddrsOld is struct sctp_getaddrs_old (include/uapi/linux/sctp.h). AddrNum
// holds the byte length of the packed sockaddr array, not a count of addresses.
type getAddrsOld struct {
	AssocID int32
	AddrNum int32
	Addrs   uintptr
}

// sctpConnect starts the association handshake towards every address in raddr.
// On a non-blocking socket it reports EINPROGRESS.
func sctpConnect(fd int, raddr *SCTPAddr) error {
	buf := raddr.toRawSockAddrBuf()
	if len(buf) == 0 {
		return syscall.EINVAL
	}

	param := getAddrsOld{
		AddrNum: int32(len(buf)),
		Addrs:   uintptr(unsafe.Pointer(&buf[0])),
	}

	// The kernel reads the length with get_user(int, optlen), so it must be a
	// 4-byte int rather than a uintptr.
	optlen := int32(unsafe.Sizeof(param))

	err := getsockopt(fd, sctpOptConnectX3, unsafe.Pointer(&param), unsafe.Pointer(&optlen))

	// param.Addrs is an integer to the collector, so buf is pinned by hand.
	runtime.KeepAlive(buf)

	return err
}

// Dial establishes an SCTP association with raddr, binding to laddr when it is
// non-nil and requesting a multihomed association when laddr names several
// addresses. ctx bounds the handshake; without a deadline it lasts as long as
// the kernel retries the INIT, which InitMsg.MaxAttempts and
// InitMsg.MaxInitTimeout govern.
func Dial(ctx context.Context, network string, laddr, raddr *SCTPAddr, options InitMsg) (*SCTPConn, error) {
	if raddr == nil || len(raddr.IPAddrs) == 0 {
		return nil, fmt.Errorf("sctp: dial requires a remote address")
	}

	af, ipv6only := favoriteAddrFamily(network, laddr, raddr, "dial")

	// SOCK_NONBLOCK makes the handshake asynchronous: the kernel reads O_NONBLOCK
	// off the socket's file to decide whether connectx waits.
	sock, err := syscall.Socket(
		af,
		syscall.SOCK_STREAM|syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC,
		syscall.IPPROTO_SCTP,
	)
	if err != nil {
		return nil, err
	}

	ownsSock := true

	defer func() {
		if ownsSock {
			if cerr := syscall.Close(sock); cerr != nil {
				logger.AmfLog.Warn("failed to close socket", zap.Error(cerr))
			}
		}
	}()

	if err := setDefaultSockopts(sock, af, ipv6only); err != nil {
		return nil, err
	}

	if err := setInitOpts(sock, options); err != nil {
		return nil, err
	}

	if err := setNoDelay(sock); err != nil {
		return nil, err
	}

	if laddr != nil {
		if err := sctpBind(sock, bindAddr(laddr, af), sctpBindxAddAddr); err != nil {
			return nil, err
		}
	}

	conn := newSCTPConn(sock)
	if conn == nil {
		return nil, fmt.Errorf("sctp: could not hand socket to the runtime poller")
	}

	ownsSock = false

	// Before connecting: the handshake's outcome is one of these events, and the
	// kernel only generates an event subscribed to at the time it occurs.
	if err := conn.subscribeEvents(dialedEvents); err != nil {
		_ = conn.Abort()

		return nil, fmt.Errorf("subscribe events: %w", err)
	}

	if err := conn.connect(ctx, raddr); err != nil {
		// Abort: there is no association to shut down gracefully, and Close would
		// spend the drain timeout establishing that.
		_ = conn.Abort()

		return nil, fmt.Errorf("dial %s: %w", raddr.String(), err)
	}

	return conn, nil
}

// bindAddr substitutes the family's wildcard for an address with no IPs, which
// bindx cannot express as an empty list. The copy keeps a caller reusing its
// SCTPAddr across dials from seeing it mutated.
func bindAddr(laddr *SCTPAddr, af int) *SCTPAddr {
	if len(laddr.IPAddrs) > 0 {
		return laddr
	}

	wildcard := net.IPv4zero
	if af == syscall.AF_INET6 {
		wildcard = net.IPv6zero
	}

	return &SCTPAddr{IPAddrs: []net.IPAddr{{IP: wildcard}}, Port: laddr.Port}
}

// connect runs the association handshake to completion.
func (c *SCTPConn) connect(ctx context.Context, raddr *SCTPAddr) error {
	err := c.controlFd(func(fd int) error { return sctpConnect(fd, raddr) })
	if err != nil && !errors.Is(err, syscall.EINPROGRESS) {
		return err
	}

	return c.awaitAssocChange(ctx)
}

// awaitAssocChange waits for the kernel to report how the handshake ended.
//
// Write readiness cannot be used as the completion signal: sctp_poll marks a
// socket writable as soon as its send buffer has room, which holds throughout
// the handshake (net/sctp/socket.c). The outcome arrives instead as an
// SCTP_ASSOC_CHANGE notification on the receive queue — SCTP_COMM_UP on
// success, SCTP_CANT_STR_ASSOC when the INIT is aborted or runs out of attempts
// — which is delivered through sk_data_ready.
func (c *SCTPConn) awaitAssocChange(ctx context.Context) error {
	if deadline, ok := ctx.Deadline(); ok {
		if err := c.setReadDeadline(deadline); err != nil {
			return err
		}

		// Cleared so the deadline cannot expire a later read on the association.
		defer func() { _ = c.setReadDeadline(time.Time{}) }()
	}

	if done := ctx.Done(); done != nil {
		stop := make(chan struct{})
		watcherDone := make(chan struct{})

		go func() {
			defer close(watcherDone)

			select {
			case <-done:
				_ = c.setReadDeadline(aLongTimeAgo)
			case <-stop:
			}
		}()

		// Registered after the deadline reset so it runs first, and waits for the
		// watcher so no deadline can be installed after that reset.
		defer func() {
			close(stop)
			<-watcherDone
		}()
	}

	buf := make([]byte, assocChangeBufSize)

	for {
		delivery, err := c.readMsgOnce(buf)
		if err != nil {
			// A cancelled context unparks the read through the deadline above.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}

			return err
		}

		change, ok := delivery.notification.(*SCTPAssocChangeEvent)
		if !ok {
			continue
		}

		if change.State() == SCTPCommUp {
			return nil
		}

		return c.assocFailure(change)
	}
}

// assocFailure turns a non-COMM_UP association change into an error, preferring
// the socket's errno: ECONNREFUSED when the peer ABORTs the INIT, ETIMEDOUT
// when the attempts are exhausted.
func (c *SCTPConn) assocFailure(change *SCTPAssocChangeEvent) error {
	var code int

	if err := c.controlFd(func(fd int) error {
		var err error

		code, err = syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_ERROR)

		return err
	}); err == nil && code != 0 {
		return syscall.Errno(code)
	}

	return fmt.Errorf("sctp: association not established (state %d, error %d)", change.State(), change.Error())
}
