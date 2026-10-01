// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
)

type Transport int

const (
	TransportSCTP Transport = iota
	TransportTCP
)

func (t Transport) String() string {
	switch t {
	case TransportSCTP:
		return "sctp"
	case TransportTCP:
		return "tcp"
	default:
		return fmt.Sprintf("transport(%d)", int(t))
	}
}

const DefaultPort uint16 = 3868

const maxMessageSize = 128 * 1024

var errConnectionSetup = errors.New("diameter: accepted connection lost during setup")

type Listener interface {
	Addr() net.Addr
	Close() error
	accept() (transport, error)
}

type transport interface {
	kind() Transport
	readMessage(buf []byte) (int, error)
	writeMessage(b []byte) error
	setUnordered()
	remoteAddr() netip.Addr
	close() error
	abort() error
}
