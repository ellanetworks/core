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
	"os"
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

// ErrMissingAddress is returned by Dial when raddr names no address to connect
// to. It is a sentinel rather than a formatted error because the alternative is
// silently connecting to the wildcard address.
var ErrMissingAddress = errors.New("sctp: dial requires a remote address")

// getAddrsOld is struct sctp_getaddrs_old (include/uapi/linux/sctp.h). AddrNum
// holds the byte length of the packed sockaddr array, not a count of addresses.
//
// Addrs is a real pointer rather than a uintptr so the garbage collector tracks
// the buffer it refers to; a uintptr would be outside the unsafe.Pointer rules
// and would need pinning by hand. Width and offsets are unchanged.
type getAddrsOld struct {
	AssocID int32
	AddrNum int32
	Addrs   unsafe.Pointer
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
		Addrs:   unsafe.Pointer(&buf[0]),
	}

	// The kernel reads the length with get_user(int, optlen), so it must be a
	// 4-byte int rather than a uintptr.
	optlen := int32(unsafe.Sizeof(param))

	return getsockopt(fd, sctpOptConnectX3, unsafe.Pointer(&param), unsafe.Pointer(&optlen))
}

// validNetwork reports the network name to use, or an error for one this
// package does not speak. An empty name means "sctp", matching ResolveSCTPAddr.
func validNetwork(network string) (string, error) {
	switch network {
	case "":
		return "sctp", nil
	case "sctp", "sctp4", "sctp6":
		return network, nil
	default:
		return "", net.UnknownNetworkError(network)
	}
}

// dialError gives every dial failure the shape the net package uses, so callers
// can classify it with errors.Is or net.Error.Timeout.
func dialError(network string, laddr, raddr *SCTPAddr, err error) error {
	opErr := &net.OpError{Op: "dial", Net: network, Err: err}

	// Assigned only when non-nil: a typed-nil in the net.Addr interface would
	// reach SCTPAddr.String and panic when OpError formats itself.
	if laddr != nil {
		opErr.Source = laddr
	}

	if raddr != nil {
		opErr.Addr = raddr
	}

	return opErr
}

// Dial establishes an SCTP association with raddr, binding to laddr when it is
// non-nil and requesting a multihomed association when laddr names several
// addresses. ctx bounds the handshake; without a deadline it lasts as long as
// the kernel retries the INIT, which InitMsg.MaxAttempts and
// InitMsg.MaxInitTimeout govern.
func Dial(ctx context.Context, network string, laddr, raddr *SCTPAddr, options InitMsg) (*SCTPConn, error) {
	resolved, err := validNetwork(network)
	if err != nil {
		// Reported against the name the caller passed, not the resolved one.
		return nil, dialError(network, laddr, raddr, err)
	}

	conn, err := dial(ctx, resolved, laddr, raddr, options)
	if err != nil {
		return nil, dialError(resolved, laddr, raddr, err)
	}

	return conn, nil
}

func dial(ctx context.Context, network string, laddr, raddr *SCTPAddr, options InitMsg) (*SCTPConn, error) {
	if raddr == nil || len(raddr.IPAddrs) == 0 {
		return nil, ErrMissingAddress
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

	// newSCTPConn owns the descriptor from here whether or not it succeeds.
	conn := newSCTPConn(sock)
	ownsSock = false

	if conn == nil {
		return nil, fmt.Errorf("sctp: could not hand socket to the runtime poller")
	}

	// Before connecting: the handshake's outcome is one of these events, and the
	// kernel only generates an event subscribed to at the time it occurs.
	if err := conn.subscribeEvents(dialedEvents); err != nil {
		_ = conn.Abort()

		return nil, err
	}

	if err := conn.connect(ctx, raddr); err != nil {
		// Abort: there is no association to shut down gracefully, and Close would
		// spend the drain timeout establishing that.
		_ = conn.Abort()

		return nil, err
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
// The outcome arrives as an SCTP_ASSOC_CHANGE notification on the receive queue:
// SCTP_COMM_UP on success, SCTP_CANT_STR_ASSOC when the INIT is aborted or runs
// out of attempts (net/sctp/sm_sideeffect.c sctp_cmd_init_failed). Write
// readiness would also signal completion — sctp_poll suppresses EPOLLOUT while
// the socket is CLOSED, which covers the whole handshake — but it cannot say
// why a handshake failed, and the notification can.
func (c *SCTPConn) awaitAssocChange(ctx context.Context) error {
	// Cleared unconditionally. The watcher below installs aLongTimeAgo for any
	// cancellable context, not only one carrying a deadline, and a deadline left
	// behind on a conn that dialled successfully would fail every later read.
	defer func() { _ = c.setReadDeadline(time.Time{}) }()

	if deadline, ok := ctx.Deadline(); ok {
		if err := c.setReadDeadline(deadline); err != nil {
			return err
		}
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

		// Registered after the reset so it runs first, and waits for the watcher
		// so no deadline can be installed after that reset.
		defer func() {
			close(stop)
			<-watcherDone
		}()
	}

	buf := make([]byte, assocChangeBufSize)

	for {
		delivery, err := c.readMsgOnce(buf)
		if err != nil {
			return dialCtxErr(ctx, err)
		}

		change, ok := delivery.notification.(*SCTPAssocChangeEvent)
		if !ok {
			continue
		}

		if change.State() == SCTPCommUp {
			// A cancellation landing as the association comes up still means the
			// caller no longer wants it (net/fd_unix.go, Go issue 16523).
			return ctx.Err()
		}

		return c.assocFailure(change)
	}
}

// dialCtxErr reports why the handshake read ended. The read deadline is only
// ever installed from ctx, so its expiry means ctx is done or about to be: the
// poller's timer and the context's fire at the same instant and either can win,
// which would otherwise make the returned error nondeterministic.
func dialCtxErr(ctx context.Context, err error) error {
	if ctx.Done() != nil && errors.Is(err, os.ErrDeadlineExceeded) {
		<-ctx.Done()
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}

	return err
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
