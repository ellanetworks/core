// Copyright 2024 Ella Networks
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
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	NGAPPPID uint32 = 60
)

const (
	SolSCTP = 132

	SCTPBindxAddAddr = 0x01
	SCTPBindxRemAddr = 0x02

	MsgNotification = 0x8000
)

const (
	SCTPRtoInfo = iota
	SCTPAssocInfo
	SCTPInitMsg
	SCTPNoDelay
	SCTPAutoClose
	SCTPSetPeerPrimaryAddr
	SCTPPrimaryAddr
	SCTPAdaptationLayer
	SCTPDisableFragments
	SCTPPeerAddrParams
	SCTPDefaultSentParam
	SCTPEvents
	SCTPIWantMappedV4Addr
	SCTPMaxSeg
	SCTPStatus
	SCTPGetPeerAddrInfo
	SCTPDelayedAckTime
	SCTPDelayedAck  = SCTPDelayedAckTime
	SCTPDelayedSack = SCTPDelayedAckTime

	SCTPSockOptBindxAdd  = 100
	SCTPSockOptBindxRem  = 101
	SCTPSockOptPeelOff   = 102
	SCTPGetPeerAddrs     = 108
	SCTPGetLocalAddrs    = 109
	SCTPSockOptConnectx  = 110
	SCTPSockOptConnectx3 = 111
)

const (
	SCTPEventDataIO = 1 << iota
	SCTPEventAssociation
	SCTPEventAddress
	SCTPEventSendFailure
	SCTPEventSendPeerError
	SCTPEventShutdown
	SCTPEventPartialDelivery
	SCTPEventAdaptationLayer
	SCTPEventAuthentication
	SCTPEventSenderDry

	SCTPEventAll = SCTPEventDataIO | SCTPEventAssociation | SCTPEventAddress | SCTPEventSendFailure | SCTPEventSendPeerError | SCTPEventShutdown | SCTPEventPartialDelivery | SCTPEventAdaptationLayer | SCTPEventAuthentication | SCTPEventSenderDry
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

type NotificationHandler func([]byte) error

type EventSubscribe struct {
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
	SCTPCMsgInit = iota
	SCTPCMsgSndRcv
	SCTPCMsgSndInfo
	SCTPCMsgRcvInfo
	SCTPCMsgNxtInfo
)

const (
	SCTPUnordered = 1 << iota
	SCTPAddrOver
	SCTPAbort
	SCTPSackImmediately
	SCTPEof
)

type InitMsg struct {
	NumOstreams    uint16
	MaxInstreams   uint16
	MaxAttempts    uint16
	MaxInitTimeout uint16
}

// Retransmission Timeout Parameters defined in RFC 6458 8.1
type RtoInfo struct {
	SrtoAssocID int32
	SrtoInitial uint32
	SrtoMax     uint32
	StroMin     uint32
}

// Association Parameters defined in RFC 6458 8.1
type AssocInfo struct {
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
	Stream  uint16
	SSN     uint16
	Flags   uint16
	_       uint16
	PPID    uint32
	Context uint32
	TTL     uint32
	TSN     uint32
	CumTSN  uint32
	AssocID int32
}

type SndInfo struct {
	SID     uint16
	Flags   uint16
	PPID    uint32
	Context uint32
	AssocID int32
}

type GetAddrsOld struct {
	AssocID int32
	AddrNum int32
	Addrs   uintptr
}

type NotificationHeader struct {
	Type   uint16
	Flags  uint16
	Length uint32
}

type SCTPState uint16

const (
	SCTPCommUp = SCTPState(iota)
	SCTPCommLost
	SCTPRestart
	SCTPShutdownComp
	SCTPCantStrAssoc
)

var nativeEndian binary.ByteOrder

func init() {
	i := uint16(1)
	if *(*byte)(unsafe.Pointer(&i)) == 0 {
		nativeEndian = binary.BigEndian
	} else {
		nativeEndian = binary.LittleEndian
	}
}

func toBuf(v any) []byte {
	var buf bytes.Buffer
	err := binary.Write(&buf, nativeEndian, v)
	if err != nil {
		logger.AmfLog.Warn("failed to write binary", zap.Error(err))
	}
	return buf.Bytes()
}

func htons(h uint16) uint16 {
	if nativeEndian == binary.LittleEndian {
		return (h << 8 & 0xff00) | (h >> 8 & 0xff)
	}
	return h
}

var ntohs = htons

// setInitOpts sets options for an SCTP association initialization
// see https://tools.ietf.org/html/rfc4960#page-25
func setInitOpts(fd int, options InitMsg) error {
	optlen := unsafe.Sizeof(options)
	err := setsockopt(fd, SCTPInitMsg, uintptr(unsafe.Pointer(&options)), optlen)
	return err
}

func getRtoInfo(fd int) (*RtoInfo, error) {
	rtoInfo := RtoInfo{}
	rtolen := unsafe.Sizeof(rtoInfo)
	err := getsockopt(fd, SCTPRtoInfo, uintptr(unsafe.Pointer(&rtoInfo)), uintptr(unsafe.Pointer(&rtolen)))
	if err != nil {
		return nil, err
	}

	return &rtoInfo, err
}

func setRtoInfo(fd int, rtoInfo RtoInfo) error {
	rtolen := unsafe.Sizeof(rtoInfo)
	err := setsockopt(fd, SCTPRtoInfo, uintptr(unsafe.Pointer(&rtoInfo)), rtolen)
	return err
}

func getAssocInfo(fd int) (*AssocInfo, error) {
	info := AssocInfo{}
	optlen := unsafe.Sizeof(info)
	err := getsockopt(fd, SCTPAssocInfo, uintptr(unsafe.Pointer(&info)), uintptr(unsafe.Pointer(&optlen)))
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func setAssocInfo(fd int, info AssocInfo) error {
	optlen := unsafe.Sizeof(info)
	err := setsockopt(fd, SCTPAssocInfo, uintptr(unsafe.Pointer(&info)), optlen)
	return err
}

type SCTPAddr struct {
	IPAddrs []net.IPAddr
	Port    int
}

func (a *SCTPAddr) ToRawSockAddrBuf() []byte {
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

func SCTPBind(fd int, addr *SCTPAddr, flags int) error {
	var option uintptr
	switch flags {
	case SCTPBindxAddAddr:
		option = SCTPSockOptBindxAdd
	case SCTPBindxRemAddr:
		option = SCTPSockOptBindxRem
	default:
		return syscall.EINVAL
	}

	buf := addr.ToRawSockAddrBuf()
	err := setsockopt(fd, option, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return err
}

type SCTPConn struct {
	_fd                 int32
	notificationHandler NotificationHandler
}

func (c *SCTPConn) fd() int {
	return int(atomic.LoadInt32(&c._fd))
}

func NewSCTPConn(fd int, handler NotificationHandler) *SCTPConn {
	conn := &SCTPConn{
		_fd:                 int32(fd),
		notificationHandler: handler,
	}
	return conn
}

func (c *SCTPConn) SubscribeEvents(flags int) error {
	var d, a, ad, sf, p, sh, pa, ada, au, se uint8
	if flags&SCTPEventDataIO > 0 {
		d = 1
	}
	if flags&SCTPEventAssociation > 0 {
		a = 1
	}
	if flags&SCTPEventAddress > 0 {
		ad = 1
	}
	if flags&SCTPEventSendFailure > 0 {
		sf = 1
	}
	if flags&SCTPEventSendPeerError > 0 {
		p = 1
	}
	if flags&SCTPEventShutdown > 0 {
		sh = 1
	}
	if flags&SCTPEventPartialDelivery > 0 {
		pa = 1
	}
	if flags&SCTPEventAdaptationLayer > 0 {
		ada = 1
	}
	if flags&SCTPEventAuthentication > 0 {
		au = 1
	}
	if flags&SCTPEventSenderDry > 0 {
		se = 1
	}
	param := EventSubscribe{
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
	err := setsockopt(c.fd(), SCTPEvents, uintptr(unsafe.Pointer(&param)), optlen)
	return err
}

func (c *SCTPConn) SubscribedEvents() (int, error) {
	param := EventSubscribe{}
	optlen := unsafe.Sizeof(param)
	err := getsockopt(c.fd(), SCTPEvents, uintptr(unsafe.Pointer(&param)), uintptr(unsafe.Pointer(&optlen)))
	if err != nil {
		return 0, err
	}
	var flags int
	if param.DataIO > 0 {
		flags |= SCTPEventDataIO
	}
	if param.Association > 0 {
		flags |= SCTPEventAssociation
	}
	if param.Address > 0 {
		flags |= SCTPEventAddress
	}
	if param.SendFailure > 0 {
		flags |= SCTPEventSendFailure
	}
	if param.PeerError > 0 {
		flags |= SCTPEventSendPeerError
	}
	if param.Shutdown > 0 {
		flags |= SCTPEventShutdown
	}
	if param.PartialDelivery > 0 {
		flags |= SCTPEventPartialDelivery
	}
	if param.AdaptationLayer > 0 {
		flags |= SCTPEventAdaptationLayer
	}
	if param.Authentication > 0 {
		flags |= SCTPEventAuthentication
	}
	if param.SenderDry > 0 {
		flags |= SCTPEventSenderDry
	}
	return flags, nil
}

func (c *SCTPConn) SetDefaultSentParam(info *SndRcvInfo) error {
	optlen := unsafe.Sizeof(*info)
	err := setsockopt(c.fd(), SCTPDefaultSentParam, uintptr(unsafe.Pointer(info)), optlen)
	return err
}

func (c *SCTPConn) GetDefaultSentParam() (*SndRcvInfo, error) {
	info := &SndRcvInfo{}
	optlen := unsafe.Sizeof(*info)
	err := getsockopt(c.fd(), SCTPDefaultSentParam, uintptr(unsafe.Pointer(info)), uintptr(unsafe.Pointer(&optlen)))
	return info, err
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
		addr.Port = int(ntohs((*(*syscall.RawSockaddrInet4)(ptr)).Port))
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
	err := getsockopt(fd, uintptr(optname), uintptr(unsafe.Pointer(&param)), uintptr(unsafe.Pointer(&optlen)))
	if err != nil {
		return nil, err
	}
	return resolveFromRawAddr(unsafe.Pointer(&param.addrs), int(param.addrNum))
}

func (c *SCTPConn) LocalAddr() net.Addr {
	addr, err := sctpGetAddrs(c.fd(), 0, SCTPGetLocalAddrs)
	if err != nil {
		return nil
	}
	return addr
}

func (c *SCTPConn) RemoteAddr() net.Addr {
	addr, err := sctpGetAddrs(c.fd(), 0, SCTPGetPeerAddrs)
	if err != nil {
		return nil
	}
	return addr
}

func (c *SCTPConn) SetDeadline(t time.Time) error {
	return syscall.EOPNOTSUPP
}

func (c *SCTPConn) SetReadDeadline(t time.Time) error {
	return syscall.EOPNOTSUPP
}

func (c *SCTPConn) SetWriteDeadline(t time.Time) error {
	return syscall.EOPNOTSUPP
}

type SCTPListener struct {
	fd   int
	epfd int // fd for epoll
}

// SocketConfig contains options for the SCTP socket.
type SocketConfig struct {
	// If Control is not nil it is called after the socket is created but before
	// it is bound or connected.
	Control func(network, address string, c syscall.RawConn) error

	// InitMsg is the options to send in the initial SCTP message
	InitMsg InitMsg

	// RtoInfo
	RtoInfo *RtoInfo

	// AssocInfo (RFC 6458)
	AssocInfo *AssocInfo
}

func (cfg *SocketConfig) Listen(net string, laddr *SCTPAddr) (*SCTPListener, error) {
	return listenSCTPExtConfig(net, laddr, cfg.InitMsg, cfg.RtoInfo, cfg.AssocInfo, cfg.Control)
}
