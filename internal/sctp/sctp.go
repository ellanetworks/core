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
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// drainBufSize is the scratch size for discarding queued data on close.
const drainBufSize = 2048

const (
	solSCTP = 132

	sctpBindxAddAddr = 0x01
	sctpBindxRemAddr = 0x02

	msgNotification = 0x8000

	sctpPartialDeliveryAborted = 0
)

const (
	sctpOptRtoInfo = iota
	sctpOptAssocInfo
	sctpOptInitMsg
	sctpOptNoDelay
	sctpOptAutoClose
	sctpOptSetPeerPrimaryAddr
	sctpOptPrimaryAddr
	sctpOptAdaptationLayer
	sctpOptDisableFragments
	sctpOptPeerAddrParams
	sctpOptDefaultSentParam
	sctpOptEvents
	sctpOptIWantMappedV4Addr
	sctpOptMaxSeg
	sctpOptStatus
	sctpOptGetPeerAddrInfo
	sctpOptDelayedAckTime

	sctpOptBindxAdd      = 100
	sctpOptBindxRem      = 101
	sctpOptGetPeerAddrs  = 108
	sctpOptGetLocalAddrs = 109
	sctpOptConnectX3     = 111
)

const (
	sctpEventDataIO = 1 << iota
	sctpEventAssociation
	sctpEventAddress
	sctpEventSendFailure
	sctpEventSendPeerError
	sctpEventShutdown
	sctpEventPartialDelivery
	sctpEventAdaptationLayer
	sctpEventAuthentication
	sctpEventSenderDry
)

type (
	SCTPNotificationType int
	SCTPAssocID          int32
)

const (
	SCTPSnTypeBase = SCTPNotificationType(iota + (1 << 15))
	SCTPAssocChange
	SCTPPeerAddrChange
	SCTPSendFailed
	SCTPRemoteError
	SCTPShutdownEvent
	SCTPPartialDeliveryEvent
	SCTPAdaptationIndication
	SCTPAuthenticationIndication
	SCTPSenderDryEvent
)

type eventSubscribe struct {
	DataIO          uint8
	Association     uint8
	Address         uint8
	SendFailure     uint8
	PeerError       uint8
	Shutdown        uint8
	PartialDelivery uint8
	AdaptationLayer uint8
	Authentication  uint8
	SenderDry       uint8
}

const (
	sctpCMsgInit = iota
	sctpCMsgSndRcv
)

type InitMsg struct {
	NumOstreams    uint16
	MaxInstreams   uint16
	MaxAttempts    uint16
	MaxInitTimeout uint16
}

// Retransmission Timeout Parameters defined in RFC 6458 8.1
type rtoInfo struct {
	SrtoAssocID int32
	SrtoInitial uint32
	SrtoMax     uint32
	SrtoMin     uint32
}

// Association Parameters defined in RFC 6458 8.1
type assocInfo struct {
	AssocID SCTPAssocID
	// maximum retransmission attempts to make for the association
	AsocMaxRxt uint16
	// number of destination addresses that the peer has
	NumberPeerDestinations uint16
	// current value of the peer's rwnd (reported in the last selective acknowledgment (SACK)) minus any outstanding data
	PeerRwnd uint32
	// the last reported rwnd that was sent to the peer
	LocalRwnd uint32
	// the association's cookie life value used when issuing cookies
	CookieLife uint32
}

type SndRcvInfo struct {
	Stream uint16
	SSN    uint16
	Flags  uint16
	// Matches the alignment padding in the kernel's struct sctp_sndrcvinfo;
	// binary.Write emits it, so removing it shifts every field below.
	_       uint16
	PPID    uint32
	Context uint32
	TTL     uint32
	TSN     uint32
	CumTSN  uint32
	AssocID int32
}

type SCTPState uint16

const (
	SCTPCommUp = SCTPState(iota)
	SCTPCommLost
	SCTPRestart
	SCTPShutdownComp
	SCTPCantStrAssoc
)

// toBuf serialises a fixed-size struct or scalar to its native-endian bytes for
// a syscall buffer. binary.Write only errors on a variable-size type, which the
// fixed-layout syscall structs passed here never are; the Warn is a backstop.
func toBuf(v any) []byte {
	var buf bytes.Buffer

	err := binary.Write(&buf, binary.NativeEndian, v)
	if err != nil {
		logger.AmfLog.Warn("failed to write binary", zap.Error(err))
	}

	return buf.Bytes()
}

// htons converts a host-order uint16 to network order (big-endian); ntohs is the
// inverse, which for a two-byte swap is the same operation.
func htons(h uint16) uint16 {
	var b [2]byte

	binary.BigEndian.PutUint16(b[:], h)

	return binary.NativeEndian.Uint16(b[:])
}

var ntohs = htons

// setInitOpts sets options for an SCTP association initialization
// see https://tools.ietf.org/html/rfc4960#page-25
func setInitOpts(fd int, options InitMsg) error {
	optlen := unsafe.Sizeof(options)
	err := setsockopt(fd, sctpOptInitMsg, unsafe.Pointer(&options), optlen)

	return err
}

func setRtoInfo(fd int, info rtoInfo) error {
	optlen := unsafe.Sizeof(info)
	err := setsockopt(fd, sctpOptRtoInfo, unsafe.Pointer(&info), optlen)

	return err
}

func setAssocInfo(fd int, info assocInfo) error {
	optlen := unsafe.Sizeof(info)
	err := setsockopt(fd, sctpOptAssocInfo, unsafe.Pointer(&info), optlen)

	return err
}

// setNoDelay disables the Nagle-like transmit delay (RFC 6458 §8.1.4), which
// otherwise holds the second of two back-to-back small PDUs until the peer's
// delayed SACK.
func setNoDelay(fd int) error {
	on := int32(1)

	return setsockopt(fd, sctpOptNoDelay, unsafe.Pointer(&on), unsafe.Sizeof(on))
}

// errMessageTooLarge reports a message larger than the supplied buffer.
var errMessageTooLarge = errors.New("sctp: message larger than read buffer")

// errUnexpectedNotification reports an event delivered between the fragments of
// a message, which the kernel does not do for a well-behaved association.
var errUnexpectedNotification = errors.New("sctp: notification during message reassembly")

// errUnrecognizedDelivery reports a delivery whose shape does not match anything
// the subscribed event set can produce.
var errUnrecognizedDelivery = errors.New("sctp: unrecognized delivery")

type SCTPAddr struct {
	IPAddrs []net.IPAddr
	Port    int
}

func (a *SCTPAddr) toRawSockAddrBuf() []byte {
	p := htons(uint16(a.Port))
	if len(a.IPAddrs) == 0 { // if a.IPAddrs list is empty - fall back to IPv4 zero addr
		s := syscall.RawSockaddrInet4{
			Family: syscall.AF_INET,
			Port:   p,
		}
		copy(s.Addr[:], net.IPv4zero)

		return toBuf(s)
	}

	buf := []byte{}

	for _, ip := range a.IPAddrs {
		ipBytes := ip.IP
		if len(ipBytes) == 0 {
			ipBytes = net.IPv4zero
		}

		if ip4 := ipBytes.To4(); ip4 != nil {
			s := syscall.RawSockaddrInet4{
				Family: syscall.AF_INET,
				Port:   p,
			}
			copy(s.Addr[:], ip4)
			buf = append(buf, toBuf(s)...)
		} else {
			var scopeid uint32

			ifi, err := net.InterfaceByName(ip.Zone)
			if err == nil {
				scopeid = uint32(ifi.Index)
			}

			s := syscall.RawSockaddrInet6{
				Family:   syscall.AF_INET6,
				Port:     p,
				Scope_id: scopeid,
			}
			copy(s.Addr[:], ipBytes)
			buf = append(buf, toBuf(s)...)
		}
	}

	return buf
}

func (a *SCTPAddr) String() string {
	var b bytes.Buffer

	for n, i := range a.IPAddrs {
		if i.IP.To4() != nil {
			b.WriteString(i.String())
		} else if i.IP.To16() != nil {
			b.WriteRune('[')
			b.WriteString(i.String())
			b.WriteRune(']')
		}

		if n < len(a.IPAddrs)-1 {
			b.WriteRune('/')
		}
	}

	b.WriteRune(':')
	b.WriteString(strconv.Itoa(a.Port))

	return b.String()
}

func (a *SCTPAddr) Network() string { return "sctp" }

// ResolveSCTPAddr parses "host:port", or "host1/host2/...:port" for a
// multihomed association — the inverse of SCTPAddr.String. Only the final
// element carries the port.
func ResolveSCTPAddr(network, address string) (*SCTPAddr, error) {
	var tcpNet string

	// SCTP shares TCP's address syntax, so parsing is delegated to the resolver.
	switch network {
	case "", "sctp":
		tcpNet = "tcp"
	case "sctp4":
		tcpNet = "tcp4"
	case "sctp6":
		tcpNet = "tcp6"
	default:
		return nil, fmt.Errorf("sctp: unknown network %q", network)
	}

	elems := strings.Split(address, "/")
	last := len(elems) - 1

	ipAddrs := make([]net.IPAddr, 0, len(elems))

	// The leading elements are bare hosts; ResolveTCPAddr requires a port.
	for _, host := range elems[:last] {
		addr, err := net.ResolveTCPAddr(tcpNet, host+":0")
		if err != nil {
			return nil, err
		}

		ipAddrs = append(ipAddrs, net.IPAddr{IP: addr.IP, Zone: addr.Zone})
	}

	addr, err := net.ResolveTCPAddr(tcpNet, elems[last])
	if err != nil {
		return nil, err
	}

	// An omitted host means the wildcard, which SCTPAddr represents as an empty
	// list rather than an entry with a nil IP.
	if addr.IP != nil {
		ipAddrs = append(ipAddrs, net.IPAddr{IP: addr.IP, Zone: addr.Zone})
	}

	return &SCTPAddr{IPAddrs: ipAddrs, Port: addr.Port}, nil
}

func sctpBind(fd int, addr *SCTPAddr, flags int) error {
	var option uintptr

	switch flags {
	case sctpBindxAddAddr:
		option = sctpOptBindxAdd
	case sctpBindxRemAddr:
		option = sctpOptBindxRem
	default:
		return syscall.EINVAL
	}

	buf := addr.toRawSockAddrBuf()
	err := setsockopt(fd, option, unsafe.Pointer(&buf[0]), uintptr(len(buf)))

	return err
}

type SCTPConn struct {
	file   *os.File
	rc     syscall.RawConn
	closed atomic.Bool

	// Cached: the address sets are fixed after establishment (inbound ASCONF is
	// discarded under the kernel default net.sctp.addip_enable=0).
	localAddr  atomic.Pointer[SCTPAddr]
	remoteAddr atomic.Pointer[SCTPAddr]

	// writerDone always exists so Close can signal a writer started later; the
	// rest stays nil on dialled conns, which write synchronously.
	writerDone   chan struct{}
	writerStop   sync.Once
	writerFlush  chan struct{}
	writeCh      chan queuedWrite
	writerExited chan struct{}
	writeLogger  *zap.Logger
}

// controlFd runs fn with the raw file descriptor held by the runtime poller.
// The fd is guaranteed valid for the duration of fn.
func (c *SCTPConn) controlFd(fn func(fd int) error) error {
	if c.rc == nil {
		return syscall.EBADF
	}

	var err error

	cerr := c.rc.Control(func(f uintptr) {
		err = fn(int(f))
	})
	if cerr != nil {
		return cerr
	}

	return err
}

// newSCTPConn wraps an existing SCTP socket file descriptor. The fd is set
// to non-blocking mode and registered with Go's runtime poller, enabling
// deadline support and safe concurrent Close. newSCTPConn takes ownership
// of fd; callers must not close it separately.
func newSCTPConn(fd int) *SCTPConn {
	// Set non-blocking before os.NewFile so the runtime poller manages the fd.
	_ = syscall.SetNonblock(fd, true)

	f := os.NewFile(uintptr(fd), "sctp")
	if f == nil {
		return nil
	}

	rc, err := f.SyscallConn()
	if err != nil {
		_ = f.Close()
		return nil
	}

	return &SCTPConn{file: f, rc: rc, writerDone: make(chan struct{}), writerFlush: make(chan struct{})}
}

func (c *SCTPConn) subscribeEvents(flags int) error {
	var d, a, ad, sf, p, sh, pa, ada, au, se uint8
	if flags&sctpEventDataIO > 0 {
		d = 1
	}

	if flags&sctpEventAssociation > 0 {
		a = 1
	}

	if flags&sctpEventAddress > 0 {
		ad = 1
	}

	if flags&sctpEventSendFailure > 0 {
		sf = 1
	}

	if flags&sctpEventSendPeerError > 0 {
		p = 1
	}

	if flags&sctpEventShutdown > 0 {
		sh = 1
	}

	if flags&sctpEventPartialDelivery > 0 {
		pa = 1
	}

	if flags&sctpEventAdaptationLayer > 0 {
		ada = 1
	}

	if flags&sctpEventAuthentication > 0 {
		au = 1
	}

	if flags&sctpEventSenderDry > 0 {
		se = 1
	}

	param := eventSubscribe{
		DataIO:          d,
		Association:     a,
		Address:         ad,
		SendFailure:     sf,
		PeerError:       p,
		Shutdown:        sh,
		PartialDelivery: pa,
		AdaptationLayer: ada,
		Authentication:  au,
		SenderDry:       se,
	}
	optlen := unsafe.Sizeof(param)

	return c.controlFd(func(fd int) error {
		return setsockopt(fd, sctpOptEvents, unsafe.Pointer(&param), optlen)
	})
}

func resolveFromRawAddr(ptr unsafe.Pointer, n int) (*SCTPAddr, error) {
	addr := &SCTPAddr{
		IPAddrs: make([]net.IPAddr, n),
	}

	switch family := (*(*syscall.RawSockaddrAny)(ptr)).Addr.Family; family {
	case syscall.AF_INET:
		addr.Port = int(ntohs((*(*syscall.RawSockaddrInet4)(ptr)).Port))

		size := unsafe.Sizeof(syscall.RawSockaddrInet4{})
		for i := 0; i < n; i++ {
			a := *(*syscall.RawSockaddrInet4)(unsafe.Pointer(
				uintptr(ptr) + size*uintptr(i)))
			addr.IPAddrs[i] = net.IPAddr{IP: a.Addr[:]}
		}
	case syscall.AF_INET6:
		addr.Port = int(ntohs((*(*syscall.RawSockaddrInet6)(ptr)).Port))

		size := unsafe.Sizeof(syscall.RawSockaddrInet6{})
		for i := 0; i < n; i++ {
			a := *(*syscall.RawSockaddrInet6)(unsafe.Pointer(
				uintptr(ptr) + size*uintptr(i)))

			var zone string

			ifi, err := net.InterfaceByIndex(int(a.Scope_id))
			if err == nil {
				zone = ifi.Name
			}

			addr.IPAddrs[i] = net.IPAddr{IP: a.Addr[:], Zone: zone}
		}
	default:
		return nil, fmt.Errorf("unknown address family: %d", family)
	}

	return addr, nil
}

func sctpGetAddrs(fd, id, optname int) (*SCTPAddr, error) {
	type getaddrs struct {
		assocID int32
		addrNum uint32
		addrs   [4096]byte
	}

	param := getaddrs{
		assocID: int32(id),
	}
	optlen := unsafe.Sizeof(param)

	err := getsockopt(fd, uintptr(optname), unsafe.Pointer(&param), unsafe.Pointer(&optlen))
	if err != nil {
		return nil, err
	}

	return resolveFromRawAddr(unsafe.Pointer(&param.addrs), int(param.addrNum))
}

func (c *SCTPConn) getAddrs(optname int) *SCTPAddr {
	var addr *SCTPAddr

	_ = c.controlFd(func(fd int) error {
		var err error

		addr, err = sctpGetAddrs(fd, 0, optname)

		return err
	})

	return addr
}

func (c *SCTPConn) LocalAddr() net.Addr {
	if addr := c.localAddr.Load(); addr != nil {
		return addr
	}

	if addr := c.getAddrs(sctpOptGetLocalAddrs); addr != nil {
		c.localAddr.Store(addr)

		return addr
	}

	return nil
}

func (c *SCTPConn) RemoteAddr() net.Addr {
	if addr := c.remoteAddr.Load(); addr != nil {
		return addr
	}

	if addr := c.getAddrs(sctpOptGetPeerAddrs); addr != nil {
		c.remoteAddr.Store(addr)

		return addr
	}

	return nil
}

func (c *SCTPConn) setDeadline(t time.Time) error {
	if c.file == nil {
		return syscall.EBADF
	}

	return c.file.SetDeadline(t)
}

func (c *SCTPConn) setWriteDeadline(t time.Time) error {
	if c.file == nil {
		return syscall.EBADF
	}

	return c.file.SetWriteDeadline(t)
}

func (c *SCTPConn) setReadDeadline(t time.Time) error {
	if c.file == nil {
		return syscall.EBADF
	}

	return c.file.SetReadDeadline(t)
}

type sctpListener struct {
	file   *os.File
	rc     syscall.RawConn
	closed atomic.Bool
}

// socketConfig contains options for the SCTP socket.
type socketConfig struct {
	// If Control is not nil it is called after the socket is created but before
	// it is bound or connected.
	Control func(network, address string, c syscall.RawConn) error

	// InitMsg is the options to send in the initial SCTP message
	InitMsg InitMsg

	// rtoInfo
	rtoInfo *rtoInfo

	// assocInfo (RFC 6458)
	assocInfo *assocInfo
}

func (cfg *socketConfig) Listen(net string, laddr *SCTPAddr) (*sctpListener, error) {
	return listenSCTPExtConfig(net, laddr, cfg.InitMsg, cfg.rtoInfo, cfg.assocInfo, cfg.Control)
}
