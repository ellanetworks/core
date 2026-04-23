package gnb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/tester/air"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap"
	"github.com/ishidawataru/sctp"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

const (
	SCTPReadBufferSize = 65535
)

// ErrNoActivePeer indicates no N2 peer is currently usable. Returned by
// SendToRan when every configured peer has failed.
var ErrNoActivePeer = errors.New("gnb: no active N2 peer")

type GnodeB struct {
	GnbID             string
	MCC               string
	MNC               string
	SST               int32
	SD                string
	Slices            []SliceOpt // Additional slices beyond SST/SD
	TAC               string
	DNN               string
	Name              string
	UEPool            map[int64]air.DownlinkSender // RANUENGAPID -> UE
	NGAPIDs           map[int64]int64              // RANUENGAPID -> AMFUENGAPID
	N3Conn            *net.UDPConn
	tunnels           map[uint32]*Tunnel // local TEID -> Tunnel
	lastGeneratedTEID uint32
	receivedFrames    map[int]map[int][]SCTPFrame // pduType -> msgType -> frames
	mu                sync.Mutex
	cond              *sync.Cond
	N3Address         netip.Addr
	PDUSessions       map[int64]map[int64]*PDUSessionInformation // RANUENGAPID -> PDUSessionID -> PDUSessionInformation
	UEAmbr            map[int64]*UEAmbrInformation               // RANUENGAPID -> UE AMBR

	// N2 peer management. Ordered list of Ella Core N2 endpoints; the gNB
	// maintains exactly one active SCTP association at a time, starting
	// with index 0 and falling through on read/dial/NG-Setup failure.
	// Guarded by n2Mu.
	n2Local     *sctp.SCTPAddr
	n2Mu        sync.RWMutex
	n2Peers     []*n2Peer
	n2Active    int // index into n2Peers; -1 when no active peer
	n2Shutdown  bool
	n2Change    chan struct{} // closed on every active-peer transition
	n2SetupOpts NGSetupRequestOpts
}

// n2Peer is one ordered N2 endpoint.
type n2Peer struct {
	address string
	conn    *sctp.SCTPConn
	state   n2PeerState
}

type n2PeerState uint8

const (
	n2StatePending n2PeerState = iota
	n2StateActive
	n2StateFailed
)

func (g *GnodeB) StorePDUSession(ranUeId int64, pduSessionInfo *PDUSessionInformation) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.PDUSessions == nil {
		g.PDUSessions = make(map[int64]map[int64]*PDUSessionInformation)
	}

	if g.PDUSessions[ranUeId] == nil {
		g.PDUSessions[ranUeId] = make(map[int64]*PDUSessionInformation)
	}

	g.PDUSessions[ranUeId][pduSessionInfo.PDUSessionID] = pduSessionInfo
	g.cond.Broadcast()
}

type UEAmbrInformation struct {
	UplinkBps   int64
	DownlinkBps int64
}

func (g *GnodeB) StoreUEAmbr(ranUeId int64, ambr *UEAmbrInformation) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.UEAmbr == nil {
		g.UEAmbr = make(map[int64]*UEAmbrInformation)
	}

	g.UEAmbr[ranUeId] = ambr
}

func (g *GnodeB) GetUEAmbr(ranUeId int64) *UEAmbrInformation {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.UEAmbr == nil {
		return nil
	}

	return g.UEAmbr[ranUeId]
}

func (g *GnodeB) GetPDUSession(ranUeId int64, pduSessionID int64) *PDUSessionInformation {
	g.mu.Lock()
	defer g.mu.Unlock()

	sessions := g.PDUSessions[ranUeId]
	if sessions == nil {
		return nil
	}

	return sessions[pduSessionID]
}

// GetPDUSessions returns all PDU sessions for a given RAN UE.
func (g *GnodeB) GetPDUSessions(ranUeId int64) map[int64]*PDUSessionInformation {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.PDUSessions[ranUeId]
}

func (g *GnodeB) WaitForPDUSession(ranUeId int64, pduSessionID int64, timeout time.Duration) (*PDUSessionInformation, error) {
	deadline := time.Now().Add(timeout)

	timer := time.AfterFunc(timeout, func() {
		g.cond.Broadcast()
	})
	defer timer.Stop()

	g.mu.Lock()
	defer g.mu.Unlock()

	for {
		if sessions, ok := g.PDUSessions[ranUeId]; ok {
			if pduSession, ok := sessions[pduSessionID]; ok {
				return pduSession, nil
			}
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for PDU session %d for RAN UE ID %d", pduSessionID, ranUeId)
		}

		g.cond.Wait()
	}
}

func (g *GnodeB) GetAMFUENGAPID(ranUeId int64) int64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.NGAPIDs[ranUeId]
}

func (g *GnodeB) UpdateNGAPIDs(ranUeId int64, amfUeId int64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.NGAPIDs == nil {
		g.NGAPIDs = make(map[int64]int64)
	}

	g.NGAPIDs[ranUeId] = amfUeId
}

func (g *GnodeB) LoadUE(ranUeId int64) (air.DownlinkSender, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	ue, ok := g.UEPool[ranUeId]
	if !ok {
		return nil, fmt.Errorf("UE is not found in GNB UE POOL with RAN UE ID %d", ranUeId)
	}

	return ue, nil
}

func (g *GnodeB) WaitForMessage(pduType int, msgType int, timeout time.Duration) (SCTPFrame, error) {
	deadline := time.Now().Add(timeout)

	timer := time.AfterFunc(timeout, func() {
		g.cond.Broadcast()
	})
	defer timer.Stop()

	g.mu.Lock()
	defer g.mu.Unlock()

	for {
		msgTypeMap, ok := g.receivedFrames[pduType]
		if ok {
			frames, ok := msgTypeMap[msgType]
			if ok && len(frames) > 0 {
				frame := frames[0]

				if len(frames) == 1 {
					delete(msgTypeMap, msgType)
				} else {
					msgTypeMap[msgType] = frames[1:]
				}

				g.receivedFrames[pduType] = msgTypeMap

				return frame, nil
			}
		}

		if time.Now().After(deadline) {
			return SCTPFrame{}, fmt.Errorf("timeout waiting for NGAP message %v", getMessageName(pduType, msgType))
		}

		g.cond.Wait()
	}
}

type SCTPFrame struct {
	Data []byte
	Info *sctp.SndRcvInfo
}

// NewGnodeB constructs a gNB with a single pre-dialed N2 conn. Used by
// ng-eNB scenarios that dial their own SCTP elsewhere. Starts the
// receiver on the given conn.
func NewGnodeB(
	gnbID string,
	mcc string,
	mnc string,
	sst int32,
	sd string,
	dnn string,
	tac string,
	name string,
	n2Conn *sctp.SCTPConn,
	n3Conn *net.UDPConn,
	n3Address netip.Addr,
) *GnodeB {
	g := &GnodeB{
		GnbID:     gnbID,
		MCC:       mcc,
		MNC:       mnc,
		SST:       sst,
		SD:        sd,
		DNN:       dnn,
		TAC:       tac,
		Name:      name,
		N3Conn:    n3Conn,
		tunnels:   make(map[uint32]*Tunnel),
		N3Address: n3Address,
		n2Peers: []*n2Peer{{
			address: "pre-dialed",
			conn:    n2Conn,
			state:   n2StateActive,
		}},
		n2Active: 0,
		n2Change: make(chan struct{}),
	}
	g.cond = sync.NewCond(&g.mu)

	go g.runReceiver(0, n2Conn)

	return g
}

type StartOpts struct {
	GnbID string
	MCC   string
	MNC   string
	SST   int32
	SD    string
	// Slices lists additional slices beyond SST/SD advertised in NG Setup.
	Slices []SliceOpt
	DNN    string
	TAC    string
	Name   string
	// CoreN2Addresses is the ordered list of Ella Core N2 endpoints. The
	// gNB uses the first as primary; on failure, it falls through to the
	// next in order. A single-entry list matches the pre-multi-peer
	// behaviour.
	CoreN2Addresses []string
	GnbN2Address    string
	GnbN3Address    string
}

// Start builds a gNB and establishes one active N2 SCTP association.
// Addresses in CoreN2Addresses are tried in order; the first one where
// dial + NG Setup send succeed becomes active. On all-fail, Start
// returns an error.
func Start(opts *StartOpts) (*GnodeB, error) {
	if len(opts.CoreN2Addresses) == 0 {
		return nil, fmt.Errorf("at least one CoreN2Address required")
	}

	local := &sctp.SCTPAddr{
		IPAddrs: []net.IPAddr{
			{IP: net.ParseIP(opts.GnbN2Address)},
		},
	}

	peers := make([]*n2Peer, len(opts.CoreN2Addresses))
	for i, a := range opts.CoreN2Addresses {
		peers[i] = &n2Peer{address: a, state: n2StatePending}
	}

	var (
		n3Conn         *net.UDPConn
		gnbN3IPAddress netip.Addr
	)

	if opts.GnbN3Address != "" {
		laddr := &net.UDPAddr{
			IP:   net.ParseIP(opts.GnbN3Address),
			Port: 2152,
		}

		var err error

		n3Conn, err = net.ListenUDP("udp", laddr)
		if err != nil {
			return nil, fmt.Errorf("could not listen on GTP-U UDP address %s: %v", opts.GnbN3Address, err)
		}

		gnbN3IPAddress, err = netip.ParseAddr(opts.GnbN3Address)
		if err != nil {
			return nil, fmt.Errorf("could not parse gNB N3 IP address: %v", err)
		}
	}

	g := &GnodeB{
		GnbID:     opts.GnbID,
		MCC:       opts.MCC,
		MNC:       opts.MNC,
		SST:       opts.SST,
		SD:        opts.SD,
		Slices:    opts.Slices,
		DNN:       opts.DNN,
		TAC:       opts.TAC,
		Name:      opts.Name,
		N3Conn:    n3Conn,
		tunnels:   make(map[uint32]*Tunnel),
		N3Address: gnbN3IPAddress,
		n2Local:   local,
		n2Peers:   peers,
		n2Active:  -1,
		n2Change:  make(chan struct{}),
		n2SetupOpts: NGSetupRequestOpts{
			GnbID:  opts.GnbID,
			Mcc:    opts.MCC,
			Mnc:    opts.MNC,
			Sst:    opts.SST,
			Tac:    opts.TAC,
			Name:   opts.Name,
			Slices: opts.Slices,
		},
	}
	g.cond = sync.NewCond(&g.mu)

	if n3Conn != nil {
		go g.GTPReader()
	}

	g.n2Mu.Lock()
	defer g.n2Mu.Unlock()

	var lastErr error

	for idx := 0; idx < len(peers); idx++ {
		if err := g.n2DialAndActivateLocked(idx); err != nil {
			lastErr = err
			continue
		}

		return g, nil
	}

	if n3Conn != nil {
		_ = n3Conn.Close()
	}

	return nil, fmt.Errorf("no N2 peer reachable: %w", lastErr)
}

// n2DialAndActivateLocked dials the peer at peers[idx], on success marks it
// active, starts its receiver goroutine, and sends NG Setup Request.
// Must be called with g.n2Mu write-held.
//
// On failure, marks the peer failed and returns the error; the caller can
// continue iterating.
func (g *GnodeB) n2DialAndActivateLocked(idx int) error {
	peer := g.n2Peers[idx]

	rem, err := sctp.ResolveSCTPAddr("sctp", peer.address)
	if err != nil {
		peer.state = n2StateFailed
		return fmt.Errorf("resolve %s: %w", peer.address, err)
	}

	conn, err := sctp.DialSCTPExt(
		"sctp", g.n2Local, rem,
		sctp.InitMsg{NumOstreams: 2, MaxInstreams: 2},
	)
	if err != nil {
		peer.state = n2StateFailed
		return fmt.Errorf("dial %s: %w", peer.address, err)
	}

	if err := conn.SubscribeEvents(sctp.SCTP_EVENT_DATA_IO); err != nil {
		_ = conn.Close()
		peer.state = n2StateFailed

		return fmt.Errorf("subscribe SCTP events on %s: %w", peer.address, err)
	}

	peer.conn = conn
	peer.state = n2StateActive
	g.n2Active = idx

	go g.runReceiver(idx, conn)

	if err := g.sendNGSetupOnConn(conn); err != nil {
		_ = conn.Close()
		peer.conn = nil
		peer.state = n2StateFailed
		g.n2Active = -1

		return fmt.Errorf("NGSetupRequest on %s: %w", peer.address, err)
	}

	close(g.n2Change)
	g.n2Change = make(chan struct{})

	logger.GnbLogger.Info(
		"gnb: active N2 peer set",
		zap.String("address", peer.address),
		zap.Int("index", idx),
	)

	return nil
}

// sendNGSetupOnConn builds and writes NGSetupRequest directly to the given
// conn. Called from the locked startup / promotion path so it cannot take
// n2Mu; goes straight to writeToConn.
func (g *GnodeB) sendNGSetupOnConn(conn *sctp.SCTPConn) error {
	pdu, err := BuildNGSetupRequest(&g.n2SetupOpts)
	if err != nil {
		return fmt.Errorf("build NGSetupRequest: %w", err)
	}

	bytes, err := ngap.Encoder(pdu)
	if err != nil {
		return fmt.Errorf("encode NGSetupRequest: %w", err)
	}

	return writeToConn(conn, bytes, NGAPProcedureNGSetupRequest)
}

// runReceiver reads SCTP frames on conn and dispatches via HandleFrame.
// On read error, triggers promotion of the next peer and exits.
func (g *GnodeB) runReceiver(idx int, conn *sctp.SCTPConn) {
	buf := make([]byte, SCTPReadBufferSize)

	for {
		n, info, err := conn.SCTPRead(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				logger.GnbLogger.Debug("SCTP peer closed (EOF)", zap.Int("peer", idx))
			} else {
				logger.GnbLogger.Warn("SCTP read error", zap.Int("peer", idx), zap.Error(err))
			}

			g.promoteNextFromReceiver(idx, conn)

			return
		}

		if n == 0 {
			// On Linux, SCTPRead returning n=0 with no error is the kernel's
			// signal that the peer has shut down the association (SHUTDOWN
			// chunk received and drained). Treat it the same as io.EOF:
			// promote the next peer and exit the receive loop. Dropping this
			// into the error path — rather than continuing — is what makes
			// graceful peer close drive failover sub-second, instead of
			// waiting for kernel SCTP heartbeat timeouts (minutes).
			logger.GnbLogger.Debug("SCTP peer shutdown (zero-byte read)", zap.Int("peer", idx))
			g.promoteNextFromReceiver(idx, conn)

			return
		}

		cp := append([]byte(nil), buf[:n]...) // copy to isolate from buffer reuse

		sctpFrame := SCTPFrame{
			Data: cp,
			Info: info,
		}

		go func(f SCTPFrame) {
			if err := HandleFrame(g, f); err != nil {
				logger.GnbLogger.Error("could not handle SCTP frame", zap.Error(err))
			}
		}(sctpFrame)
	}
}

// promoteNextFromReceiver is called by a receiver goroutine when its SCTP
// read errors. It advances the active peer to the next candidate in order.
//
// If the current active peer has already been advanced by another caller
// (e.g. Close), this is a no-op.
func (g *GnodeB) promoteNextFromReceiver(failedIdx int, failedConn *sctp.SCTPConn) {
	g.n2Mu.Lock()
	defer g.n2Mu.Unlock()

	if g.n2Shutdown {
		return
	}

	if g.n2Active != failedIdx {
		return
	}

	peer := g.n2Peers[failedIdx]
	peer.state = n2StateFailed

	if peer.conn != nil && peer.conn == failedConn {
		_ = peer.conn.Close()
	}

	peer.conn = nil
	g.n2Active = -1

	for idx := failedIdx + 1; idx < len(g.n2Peers); idx++ {
		if err := g.n2DialAndActivateLocked(idx); err != nil {
			logger.GnbLogger.Warn(
				"gnb failover: peer unreachable",
				zap.String("address", g.n2Peers[idx].address),
				zap.Error(err),
			)

			continue
		}

		return
	}

	close(g.n2Change)
	g.n2Change = make(chan struct{})

	logger.GnbLogger.Error("gnb failover: no remaining N2 peers")
}

// ActivePeerAddress returns the current active peer's address, or the empty
// string when no peer is active.
func (g *GnodeB) ActivePeerAddress() string {
	g.n2Mu.RLock()
	defer g.n2Mu.RUnlock()

	if g.n2Active < 0 || g.n2Active >= len(g.n2Peers) {
		return ""
	}

	return g.n2Peers[g.n2Active].address
}

// WaitForActivePeerChange blocks until the active peer transitions (either
// to a new peer or to no-active), or ctx is cancelled. Returns the new
// active peer's address (empty when no active peer).
func (g *GnodeB) WaitForActivePeerChange(ctx context.Context) (string, error) {
	g.n2Mu.RLock()
	ch := g.n2Change
	g.n2Mu.RUnlock()

	select {
	case <-ch:
		return g.ActivePeerAddress(), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// TriggerFailover closes the current active peer's SCTP conn, causing the
// receiver to detect the error and promote the next peer. Intended for
// tests that want to force a failover without killing the remote.
func (g *GnodeB) TriggerFailover() {
	g.n2Mu.RLock()

	var conn *sctp.SCTPConn

	if g.n2Active >= 0 && g.n2Active < len(g.n2Peers) {
		conn = g.n2Peers[g.n2Active].conn
	}

	g.n2Mu.RUnlock()

	if conn != nil {
		_ = conn.Close()
	}
}

func (g *GnodeB) GenerateTEID() uint32 {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.lastGeneratedTEID++

	return g.lastGeneratedTEID
}

func (g *GnodeB) AddUE(ranUENGAPID int64, ue air.DownlinkSender) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.UEPool == nil {
		g.UEPool = make(map[int64]air.DownlinkSender)
	}

	g.UEPool[ranUENGAPID] = ue
}

func (g *GnodeB) Close() {
	g.mu.Lock()

	tunnelsToClose := make(map[uint32]*Tunnel, len(g.tunnels))
	for teid, t := range g.tunnels {
		tunnelsToClose[teid] = t
	}
	g.mu.Unlock()

	for _, t := range tunnelsToClose {
		if err := t.tunIF.Close(); err != nil {
			logger.GnbLogger.Error("error closing TUN interface", zap.String("if", t.Name), zap.Error(err))
		}

		link, err := netlink.LinkByName(t.Name)
		if err == nil {
			if err = netlink.LinkDel(link); err != nil {
				logger.GnbLogger.Error("error deleting TUN interface", zap.String("if", t.Name), zap.Error(err))
			}
		}
	}

	g.mu.Lock()
	g.tunnels = make(map[uint32]*Tunnel)
	g.mu.Unlock()

	g.n2Mu.Lock()

	g.n2Shutdown = true
	for _, peer := range g.n2Peers {
		if peer.conn != nil {
			if err := peer.conn.Close(); err != nil {
				logger.GnbLogger.Error("could not close SCTP connection", zap.String("peer", peer.address), zap.Error(err))
			}

			peer.conn = nil
		}
	}

	g.n2Active = -1
	g.n2Mu.Unlock()

	if g.N3Conn != nil {
		err := g.N3Conn.Close()
		if err != nil {
			logger.GnbLogger.Error("could not close GTP-U UDP connection", zap.Error(err))
		}
	}
}

func (g *GnodeB) SendUplinkNAS(nasPDU []byte, amfUENGAPID int64, ranUENGAPID int64) error {
	err := g.SendUplinkNASTransport(&UplinkNasTransportOpts{
		AMFUeNgapID: amfUENGAPID,
		RANUeNgapID: ranUENGAPID,
		Mcc:         g.MCC,
		Mnc:         g.MNC,
		GnbID:       g.GnbID,
		Tac:         g.TAC,
		NasPDU:      nasPDU,
	})
	if err != nil {
		return fmt.Errorf("could not send UplinkNASTransport: %v", err)
	}

	logger.GnbLogger.Debug(
		"Sent Uplink NAS Transport",
		zap.Int64("AMF UE NGAP ID", amfUENGAPID),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
		zap.String("GNB ID", g.GnbID),
	)

	return nil
}

func (g *GnodeB) SendInitialUEMessage(nasPDU []byte, ranUENGAPID int64, guti5G *nasType.GUTI5G, cause aper.Enumerated) error {
	opts := &InitialUEMessageOpts{
		Mcc:                   g.MCC,
		Mnc:                   g.MNC,
		GnbID:                 g.GnbID,
		Tac:                   g.TAC,
		RanUENGAPID:           ranUENGAPID,
		NasPDU:                nasPDU,
		Guti5g:                guti5G,
		RRCEstablishmentCause: cause,
	}

	pdu, err := BuildInitialUEMessage(opts)
	if err != nil {
		return fmt.Errorf("couldn't build InitialUEMessage: %s", err.Error())
	}

	err = g.SendMessage(pdu, NGAPProcedureInitialUEMessage)
	if err != nil {
		return fmt.Errorf("could not send InitialUEMessage: %v", err)
	}

	logger.GnbLogger.Debug(
		"Sent Initial UE Message",
		zap.String("GNB ID", g.GnbID),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
		zap.String("MCC", g.MCC),
		zap.String("MNC", g.MNC),
		zap.String("TAC", g.TAC),
	)

	return nil
}

func isClosedErr(err error) bool {
	if err == nil {
		return false
	}

	s := err.Error()

	return strings.Contains(s, "use of closed network connection") ||
		strings.Contains(s, "file already closed")
}
