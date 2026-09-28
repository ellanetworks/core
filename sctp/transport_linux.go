// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"syscall"
	"unsafe"
)

const (
	sctpStatusPrimaryOffset = 24
	sctpPaddrAddressOffset  = 4
	sockaddrStorageSize     = 128
	sctpStatusSize          = sctpStatusPrimaryOffset + sctpPaddrAddressOffset + sockaddrStorageSize + 20
)

const MaxMessageSize = int(readBufSize)

type Dialer struct {
	LocalAddr *SCTPAddr
	InitMsg   InitMsg
	Logger    *slog.Logger
}

func (d *Dialer) Dial(ctx context.Context, raddr *SCTPAddr) (*SCTPConn, error) {
	logger := d.Logger
	if logger == nil {
		logger = slog.Default()
	}

	tune := func(fd int) error {
		if err := setRtoInfo(fd, *serverSocketConfig.rtoInfo); err != nil {
			return err
		}

		return setAssocInfo(fd, *serverSocketConfig.assocInfo)
	}

	conn, err := dial(ctx, "sctp", d.LocalAddr, raddr, d.InitMsg, tune)
	if err != nil {
		return nil, dialError("sctp", d.LocalAddr, raddr, err)
	}

	if err := conn.setReadBuffer(MaxMessageSize); err != nil {
		_ = conn.Abort()

		return nil, err
	}

	conn.startWriter(logger)

	return conn, nil
}

func (c *SCTPConn) PrepareAccepted(logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}

	if err := c.subscribeEvents(dialedEvents); err != nil {
		return err
	}

	if err := c.setReadBuffer(MaxMessageSize); err != nil {
		return err
	}

	if c.writeCh == nil {
		c.startWriter(logger)
	}

	return nil
}

func (c *SCTPConn) PrimaryPeerAddr() (netip.Addr, error) {
	var buf [sctpStatusSize]byte

	err := c.controlFd(func(fd int) error {
		optlen := uint32(len(buf))

		return getsockopt(fd, sctpOptStatus, unsafe.Pointer(&buf[0]), unsafe.Pointer(&optlen))
	})
	if err != nil {
		return netip.Addr{}, err
	}

	return parseSockaddrStorage(buf[sctpStatusPrimaryOffset+sctpPaddrAddressOffset:][:sockaddrStorageSize])
}

func parseSockaddrStorage(b []byte) (netip.Addr, error) {
	if len(b) < 24 {
		return netip.Addr{}, errors.New("sctp: short sockaddr")
	}

	switch family := binary.NativeEndian.Uint16(b); family {
	case syscall.AF_INET:
		return netip.AddrFrom4([4]byte(b[4:8])), nil
	case syscall.AF_INET6:
		return netip.AddrFrom16([16]byte(b[8:24])).Unmap(), nil
	default:
		return netip.Addr{}, fmt.Errorf("sctp: unknown address family %d", family)
	}
}
