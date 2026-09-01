// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/tester/air"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

const (
	SCTPReadBufferSize = 65535

	n2DialAttempts = 5
	n2DialBackoff  = 200 * time.Millisecond
	// n2DialTimeout bounds one handshake so a core that never answers the INIT
	// cannot stall the failover below.
	n2DialTimeout = 2 * time.Second

	// drainTimeout bounds how long the receiver waits for in-flight frames
	// to be handled before promoting the next peer on failure. A tester must
	// always make progress toward the next peer; ha/failover_connectivity_5g
	// and TestIntegration5GHAFailover depend on this bound.
	drainTimeout = 2 * time.Second
)

// waitTimeout waits for wg to become empty or d to elapse, returning whether
// it emptied first.
func waitTimeout(wg *sync.WaitGroup, d time.Duration) bool {
	done := make(chan struct{})

	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

// dialN2 establishes the N2 SCTP association, retrying with a fresh socket on
// transient connect failures (e.g. EISCONN from a lingering association left by
// a prior gNB process on the same source address). Each attempt is bounded by
// n2DialTimeout, so an unreachable core fails the whole loop in ~12s rather
// than hanging on the kernel's INIT retries.
func dialN2(local, rem *sctp.SCTPAddr) (*sctp.SCTPConn, error) {
	var lastErr error

	for attempt := 0; attempt < n2DialAttempts; attempt++ {
		conn, err := dialN2Once(local, rem)
		if err == nil {
			return conn, nil
		}

		lastErr = err

		if attempt < n2DialAttempts-1 {
			time.Sleep(time.Duration(attempt+1) * n2DialBackoff)
		}
	}

	return nil, fmt.Errorf("%d attempts: %w", n2DialAttempts, lastErr)
}

// dialN2Once performs a single bounded handshake attempt.
func dialN2Once(local, rem *sctp.SCTPAddr) (*sctp.SCTPConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), n2DialTimeout)
	defer cancel()

	return sctp.Dial(ctx, "sctp", local, rem, sctp.InitMsg{NumOstreams: 2, MaxInstreams: 2})
}

// ngapPPID is the SCTP payload protocol identifier for NGAP (TS 38.412), in
// host order; sctp.PPIDWireOrder byte-swaps it at each write.
const ngapPPID uint32 = 60

// ErrNoActivePeer indicates no N2 peer is currently usable. Returned by
// SendToRan when every configured peer has failed.
var ErrNoActivePeer = errors.New("gnb: no active N2 peer")

// ErrNoRotationCandidate indicates RotateToNextPeer was called but no other
// peer in the configured list is eligible (every other peer is marked
// n2StateFailed, or there is only one peer configured).
var ErrNoRotationCandidate = errors.New("gnb: no rotation candidate available")

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
	// receivedFrames is keyed by (Category, ProcedureCode) only, so in a multi-UE
	// scenario WaitForMessage can return another UE's frame. Pre-existing; s1enb
	// keys its equivalent by the UE id (see ENB.WaitForMessage).
	receivedFrames    map[Category]map[ngap.ProcedureCode][]SCTPFrame
	mu                sync.Mutex
	cond              *sync.Cond
	N3Address         netip.Addr
	pduSessions       map[int64]map[int64]*PDUSessionInformation // RANUENGAPID -> PDUSessionID -> PDUSessionInformation
	sessionGen        uint64                                     // bumped on every store; see awaitPDUSession
	UEAmbr            map[int64]*UEAmbrInformation               // RANUENGAPID -> UE AMBR
	UERadioCapability []byte
	radioCapReported  map[int64]bool
	dispatcher        *dispatcher // per-UE frame queues; see dispatch.go

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

// storePDUSession records what the network set up for one PDU session. Every
// store stamps a fresh generation, so a procedure can await the session its own
// signalling established instead of one an earlier procedure left behind.
func (g *GnodeB) storePDUSession(ranUeID int64, info *PDUSessionInformation) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.pduSessions == nil {
		g.pduSessions = make(map[int64]map[int64]*PDUSessionInformation)
	}

	if g.pduSessions[ranUeID] == nil {
		g.pduSessions[ranUeID] = make(map[int64]*PDUSessionInformation)
	}

	g.sessionGen++
	info.generation = g.sessionGen

	if existing, ok := g.pduSessions[ranUeID][info.PDUSessionID]; ok {
		*existing = *info
	} else {
		g.pduSessions[ranUeID][info.PDUSessionID] = info
	}

	g.cond.Broadcast()
}

// sessionGeneration reports the store's current generation. Read it before
// sending, then hand it to awaitPDUSession to ignore anything already stored.
func (g *GnodeB) sessionGeneration() uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.sessionGen
}

// awaitPDUSession blocks until the network establishes PDU session
// pduSessionID for ranUeID with a generation newer than after, then returns it
// by value. Nothing outside this package ever holds the stored struct:
// storePDUSession updates it in place, so a shared pointer would report a later
// TEID reallocation to a caller that had already built its tunnel.
func (g *GnodeB) awaitPDUSession(ranUeID, pduSessionID int64, after uint64, timeout time.Duration) (PDUSessionInformation, error) {
	deadline := time.Now().Add(timeout)

	timer := time.AfterFunc(timeout, func() { g.cond.Broadcast() })
	defer timer.Stop()

	g.mu.Lock()
	defer g.mu.Unlock()

	for {
		if s, ok := g.pduSessions[ranUeID][pduSessionID]; ok && s.generation > after {
			return *s, nil
		}

		if time.Now().After(deadline) {
			return PDUSessionInformation{}, fmt.Errorf("timeout waiting for PDU session %d of RAN UE NGAP ID %d", pduSessionID, ranUeID)
		}

		g.cond.Wait()
	}
}

type UEAmbrInformation struct {
	UplinkBps   int64
	DownlinkBps int64
}

var DefaultUERadioCapability = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}

func (g *GnodeB) claimRadioCapabilityReport(ranUeId int64) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.UERadioCapability) == 0 {
		return false
	}

	if g.radioCapReported == nil {
		g.radioCapReported = make(map[int64]bool)
	}

	if g.radioCapReported[ranUeId] {
		return false
	}

	g.radioCapReported[ranUeId] = true

	return true
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

// pduSessionsFor returns the sessions currently stored for ranUeID, by value
// and ordered by PDU session ID, for building a response that must list them
// all.
func (g *GnodeB) pduSessionsFor(ranUeID int64) []PDUSessionInformation {
	g.mu.Lock()
	defer g.mu.Unlock()

	sessions := make([]PDUSessionInformation, 0, len(g.pduSessions[ranUeID]))
	for _, s := range g.pduSessions[ranUeID] {
		sessions = append(sessions, *s)
	}

	sort.Slice(sessions, func(i, j int) bool { return sessions[i].PDUSessionID < sessions[j].PDUSessionID })

	return sessions
}

func (g *GnodeB) dropRadioCapabilityReport(ranUeID int64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.radioCapReported, ranUeID)
}

func (g *GnodeB) dropPDUSessions(ranUeID int64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.pduSessions, ranUeID)
}

func (g *GnodeB) removePDUSession(ranUeID int64, pduSessionID int64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.pduSessions[ranUeID], pduSessionID)
	g.cond.Broadcast()
}

// awaitPDUSessionRelease blocks until the gNB no longer holds resources for
// pduSessionID, which it drops when the AMF asks for them back in a PDU SESSION
// RESOURCE RELEASE COMMAND.
func (g *GnodeB) awaitPDUSessionRelease(ranUeID, pduSessionID int64, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	timer := time.AfterFunc(timeout, func() { g.cond.Broadcast() })
	defer timer.Stop()

	g.mu.Lock()
	defer g.mu.Unlock()

	for {
		if _, ok := g.pduSessions[ranUeID][pduSessionID]; !ok {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for the release of PDU session %d of RAN UE NGAP ID %d", pduSessionID, ranUeID)
		}

		g.cond.Wait()
	}
}

func (g *GnodeB) updatePDUSessionQoS(ranUeID int64, pduSessionID int64, info *PDUSessionModifyInfo) {
	g.mu.Lock()
	defer g.mu.Unlock()

	session := g.pduSessions[ranUeID][pduSessionID]
	if session == nil {
		return
	}

	if info.FiveQi != 0 {
		session.FiveQi = info.FiveQi
	}

	if info.PriArp != 0 {
		session.PriArp = info.PriArp
	}

	if info.QFI != 0 {
		session.QosId = info.QFI
		session.QFI = info.QFI
	}

	if info.AmbrUplink != 0 {
		session.AmbrUplink = info.AmbrUplink
	}

	if info.AmbrDownlink != 0 {
		session.AmbrDownlink = info.AmbrDownlink
	}

	g.cond.Broadcast()
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

// WaitForMessage waits for an inbound PDU of the given category and procedure,
// consuming it from the receive buffer. internal/tester/s1enb waits the same way.
func (g *GnodeB) WaitForMessage(cat Category, code ngap.ProcedureCode, timeout time.Duration) (SCTPFrame, error) {
	deadline := time.Now().Add(timeout)

	timer := time.AfterFunc(timeout, func() {
		g.cond.Broadcast()
	})
	defer timer.Stop()

	g.mu.Lock()
	defer g.mu.Unlock()

	for {
		if byCode, ok := g.receivedFrames[cat]; ok {
			if frames := byCode[code]; len(frames) > 0 {
				frame := frames[0]

				if len(frames) == 1 {
					delete(byCode, code)
				} else {
					byCode[code] = frames[1:]
				}

				return frame, nil
			}
		}

		if time.Now().After(deadline) {
			return SCTPFrame{}, fmt.Errorf("timeout waiting for NGAP message %s", messageName(cat, code))
		}

		g.cond.Wait()
	}
}

// Category is the NGAP-PDU CHOICE alternative a received frame arrived in.
// internal/tester/s1enb categorises S1AP the same way.
type Category int

const (
	Initiating Category = iota
	Successful
	Unsuccessful
)

// SCTPFrame is a decoded inbound NGAP PDU: its category, procedure code, and
// the procedure's open-type value (ready for the matching ngap.ParseXxx). Data
// is the PDU as received.
type SCTPFrame struct {
	Category      Category
	ProcedureCode ngap.ProcedureCode
	Value         []byte
	Data          []byte
	Info          *sctp.SndRcvInfo
}

// NewGnodeB constructs a gNB around a single pre-dialed N2 conn, for ng-eNB
// scenarios that dial their own SCTP, and starts the receiver on it.
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
		UERadioCapability: DefaultUERadioCapability,
		GnbID:             gnbID,
		MCC:               mcc,
		MNC:               mnc,
		SST:               sst,
		SD:                sd,
		DNN:               dnn,
		TAC:               tac,
		Name:              name,
		N3Conn:            n3Conn,
		tunnels:           make(map[uint32]*Tunnel),
		N3Address:         n3Address,
		n2Peers: []*n2Peer{{
			address: "pre-dialed",
			conn:    n2Conn,
			state:   n2StateActive,
		}},
		n2Active: 0,
		n2Change: make(chan struct{}),
	}
	g.cond = sync.NewCond(&g.mu)
	g.dispatcher = newDispatcher(g)

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
	// Ordered Ella Core N2 endpoints: the gNB uses the first as primary and
	// falls through to the next on failure.
	CoreN2Addresses []string
	GnbN2Address    string
	GnbN3Address    string
}

// Start builds a gNB and establishes one active N2 SCTP association. The
// CoreN2Addresses are tried in order; the first where dial and NG Setup send
// both succeed becomes active, else Start returns an error.
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
		UERadioCapability: DefaultUERadioCapability,
		GnbID:             opts.GnbID,
		MCC:               opts.MCC,
		MNC:               opts.MNC,
		SST:               opts.SST,
		SD:                opts.SD,
		Slices:            opts.Slices,
		DNN:               opts.DNN,
		TAC:               opts.TAC,
		Name:              opts.Name,
		N3Conn:            n3Conn,
		tunnels:           make(map[uint32]*Tunnel),
		N3Address:         gnbN3IPAddress,
		n2Local:           local,
		n2Peers:           peers,
		n2Active:          -1,
		n2Change:          make(chan struct{}),
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
	g.dispatcher = newDispatcher(g)

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

	conn, err := dialN2(g.n2Local, rem)
	if err != nil {
		peer.state = n2StateFailed
		return err
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
	pkt, err := BuildNGSetupRequest(&g.n2SetupOpts)
	if err != nil {
		return fmt.Errorf("build NGSetupRequest: %w", err)
	}

	return writeToConn(conn, pkt, NGAPProcedureNGSetupRequest)
}

// runReceiver reads SCTP frames on conn, handles the unkeyed ones inline and
// dispatches the UE-associated ones to that UE's worker.
//
// TS 38.413 §6: "The signalling connection shall provide in sequence delivery of NGAP
// messages." TS 38.412 §7: "For a single UE-associated signalling, the NG-RAN node
// shall use one SCTP association and one SCTP stream, and the SCTP association/stream
// should not be changed during the communication of the UE-associated signalling...".
// The ordering the tester owes the core is therefore per UE-associated stream, which
// is what the dispatcher provides (see dispatch.go).
//
// Unkeyed frames run inline so a context-creating procedure (HANDOVER REQUEST)
// completes before any later frame for the context it creates.
func (g *GnodeB) runReceiver(idx int, conn *sctp.SCTPConn) {
	buf := make([]byte, SCTPReadBufferSize)

	var inflight sync.WaitGroup // frames this receiver enqueued, not yet handled

	for {
		n, info, err := conn.ReadMsg(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				logger.GnbLogger.Debug("SCTP peer closed (EOF)", zap.Int("peer", idx))
			} else {
				logger.GnbLogger.Warn("SCTP read error", zap.Int("peer", idx), zap.Error(err))
			}

			// Bounded drain: frames read before the failure are handled before
			// the next peer becomes active. During the drain g.n2Active is still
			// the failed index, so any reply goes to the dead socket and is
			// dropped with an error log — this is by design, so no message of
			// the dead association is answered on the new one.
			if !waitTimeout(&inflight, drainTimeout) {
				logger.GnbLogger.Warn("gnb: frames of the failed association still in flight at failover",
					zap.Int("peer", idx))
			}

			g.promoteNextFromReceiver(idx, conn)

			return
		}

		if n == 0 {
			continue
		}

		frame, err := decodeFrame(append([]byte(nil), buf[:n]...), info)
		if err != nil {
			logger.GnbLogger.Warn("gnb: undecodable NGAP PDU", zap.Int("len", n), zap.Error(err))
			continue
		}

		ranUEID, ok := frameRANUEID(frame)
		if !ok {
			// Node-level and context-creating procedures run inline, so nothing
			// dispatched later can overtake them.
			if err := HandleFrame(g, frame); err != nil {
				logger.GnbLogger.Error("could not handle SCTP frame", zap.Error(err))
			}

			continue
		}

		inflight.Add(1)
		g.dispatcher.dispatch(ranUEID, frame, inflight.Done)
	}
}

// promoteNextFromReceiver is called by a receiver goroutine when its SCTP
// read errors. It advances the active peer to the next candidate in order.
//
// If the current active peer has already been advanced by another caller (e.g.
// Close), this is a no-op.
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

	// Scan the remaining peers in wrap order, skipping anything already
	// marked failed. Wrapping matters after a rotation has moved us off
	// the head of the list: if the current peer then truly dies (SCTP
	// ABORT), we still want to try the earlier (non-failed) peers.
	n := len(g.n2Peers)
	for step := 1; step < n; step++ {
		cand := (failedIdx + step) % n
		if g.n2Peers[cand].state == n2StateFailed {
			continue
		}

		if err := g.n2DialAndActivateLocked(cand); err != nil {
			logger.GnbLogger.Warn(
				"gnb failover: peer unreachable",
				zap.String("address", g.n2Peers[cand].address),
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

func (g *GnodeB) ActivePeerAddress() string {
	g.n2Mu.RLock()
	defer g.n2Mu.RUnlock()

	if g.n2Active < 0 || g.n2Active >= len(g.n2Peers) {
		return ""
	}

	return g.n2Peers[g.n2Active].address
}

// WaitForActivePeerChange blocks until the active peer transitions or ctx is
// cancelled, returning the new active peer's address (empty when none).
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

// TriggerFailover closes the active peer's SCTP conn so the receiver promotes
// the next peer, forcing a failover in tests without killing the remote.
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

// RotateToNextPeer moves the active SCTP association to the next non-failed
// peer under n2Mu, wrapping around. The previous peer is left n2StatePending
// (not failed) so a later rotation can return to it: a follower that rejects
// our writes today may become the Raft leader tomorrow.
//
// Returns ErrNoRotationCandidate when no other non-failed peer exists, leaving
// the active peer untouched. If every candidate dial fails, the gNB ends up
// with no active peer and the error describes the last dial failure.
//
// The previous peer's receiver still fires on conn close, but its
// `g.n2Active != failedIdx` guard makes it a no-op once the index has moved.
func (g *GnodeB) RotateToNextPeer() error {
	g.n2Mu.Lock()
	defer g.n2Mu.Unlock()

	if g.n2Shutdown {
		return errors.New("gnb: shutdown")
	}

	oldActive := g.n2Active
	if oldActive < 0 || oldActive >= len(g.n2Peers) {
		return fmt.Errorf("%w: no current active peer", ErrNoRotationCandidate)
	}

	n := len(g.n2Peers)

	haveCandidate := false

	for step := 1; step < n; step++ {
		if g.n2Peers[(oldActive+step)%n].state != n2StateFailed {
			haveCandidate = true
			break
		}
	}

	if !haveCandidate {
		return ErrNoRotationCandidate
	}

	// Tear down the current active peer. Leave it in n2StatePending (not
	// failed) so later rotations can return to it.
	oldPeer := g.n2Peers[oldActive]
	if oldPeer.conn != nil {
		_ = oldPeer.conn.Close()
	}

	oldPeer.conn = nil
	oldPeer.state = n2StatePending
	g.n2Active = -1

	var lastDialErr error

	for step := 1; step < n; step++ {
		cand := (oldActive + step) % n
		if g.n2Peers[cand].state == n2StateFailed {
			continue
		}

		if err := g.n2DialAndActivateLocked(cand); err != nil {
			logger.GnbLogger.Warn(
				"gnb rotate: candidate unreachable",
				zap.String("address", g.n2Peers[cand].address),
				zap.Error(err),
			)

			lastDialErr = err

			continue
		}

		return nil
	}

	close(g.n2Change)
	g.n2Change = make(chan struct{})

	if lastDialErr != nil {
		return fmt.Errorf("gnb rotate: all candidates failed; last error: %w", lastDialErr)
	}

	return fmt.Errorf("gnb rotate: all candidates failed")
}

func (g *GnodeB) allocTEID() uint32 {
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

	// Stop the per-UE workers after the associations are closed: the receivers
	// stop enqueueing once their conn is closed, so this only joins the
	// handlers still running.
	g.dispatcher.closeAll()

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

func (g *GnodeB) SendInitialUEMessage(nasPDU []byte, ranUENGAPID int64, guti5G []byte, cause ngap.RRCEstablishmentCause) error {
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

	pkt, err := BuildInitialUEMessage(opts)
	if err != nil {
		return fmt.Errorf("couldn't build InitialUEMessage: %s", err.Error())
	}

	err = g.SendToRan(pkt, NGAPProcedureInitialUEMessage)
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
