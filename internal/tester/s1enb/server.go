// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package s1enb is an S1AP eNB simulator that drives the 4G MME over S1-MME
// (TS 36.413) for integration tests. It is deliberately separate from
// internal/tester/enb, which is an ng-eNB (an LTE radio attached to the 5G core
// over NGAP) and therefore tests the AMF, not the MME. This package speaks S1AP
// (the in-repo github.com/ellanetworks/core/s1ap codecs) and EPS NAS, mirroring
// the structure of the gNB tester (internal/tester/gnb) without inheriting its
// NGAP/5G concepts.
package s1enb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/s1ap"
	"github.com/ishidawataru/sctp"
	"go.uber.org/zap"
)

// s1apPPID is the SCTP payload protocol identifier for S1AP (18, TS 36.412),
// pre-encoded in network byte order the way the SCTP socket layer expects —
// mirroring free5gc's ngap.PPID = 0x3c000000 (NGAP = 60). The pinned
// ishidawataru/sctp release writes SndRcvInfo.PPID verbatim, so the value must
// already be byte-swapped; the MME converts it back with networkToNativeEndianness32.
const s1apPPID uint32 = 0x12000000

// ErrNoActiveMME indicates no S1-MME peer is currently usable. Returned by
// SendMessage when every configured peer has failed.
var ErrNoActiveMME = errors.New("s1enb: no active S1-MME peer")

// Category is the S1AP PDU category a received frame belongs to.
type Category int

const (
	Initiating Category = iota
	Successful
	Unsuccessful
)

// Frame is a decoded inbound S1AP PDU: its category, procedure code, and the
// procedure's open-type value (ready for the matching s1ap.ParseXxx).
type Frame struct {
	Category      Category
	ProcedureCode s1ap.ProcedureCode
	Value         []byte
	ENBUES1APID   int64 // target eNB-UE-S1AP-ID; NoUEID for non-UE-associated frames
}

// NoUEID tags a frame (and a wait) that is not associated with a specific UE
// (S1 Setup, Reset). Real eNB-UE-S1AP-IDs start at 1 (see AllocateENBUEID).
const NoUEID int64 = 0

// StartOpts configures an eNB simulator.
type StartOpts struct {
	ENBID            uint32 // 20-bit macro eNB ID
	MCC              string
	MNC              string
	TAC              string // hex or decimal string; parsed as the 16-bit TAC
	Name             string
	CoreS1MMEAddress string // "<ip>:<port>"; the MME's S1-MME endpoint (default port 36412)
	// CoreS1MMEAddresses is the ordered list of MME S1-MME endpoints for
	// multi-core (HA) scenarios. The eNB keeps exactly one active SCTP
	// association at a time, starting at index 0 and falling through to the
	// next peer on read/dial/S1-Setup failure. When empty, CoreS1MMEAddress
	// is used as a single-element list.
	CoreS1MMEAddresses []string
	ENBAddress         string // local S1-MME bind address
	ENBN3Address       string // eNB S1-U (N3) address reported in bearer setup; defaults to ENBAddress
	// EnableDatapath binds the S1-U (GTP-U) socket so connectivity scenarios can
	// run a user-plane datapath. Off by default: signalling-only scenarios skip
	// it, and several eNBs can then share one N3 address without a port clash.
	EnableDatapath bool
	// SkipS1SetupWait sends the S1 Setup Request but returns without waiting for
	// the response, so a scenario can assert an S1 Setup Failure (TS 36.413).
	SkipS1SetupWait bool
}

// ENB is a connected S1AP eNB. Its methods are safe for concurrent use.
type ENB struct {
	enbID  uint32
	name   string
	plmn   s1ap.PLMNIdentity
	tac    uint16
	n3Addr net.IP // eNB S1-U endpoint reported in bearer setup

	n3Conn  *net.UDPConn       // S1-U (GTP-U) socket, nil when no N3 address is configured
	tunnels map[uint32]*tunnel // keyed by eNB downlink TEID

	mu             sync.Mutex
	cond           *sync.Cond
	receivedFrames map[Category]map[s1ap.ProcedureCode][]Frame
	closed         bool

	sendMu sync.Mutex // serializes SCTP writes so concurrent per-UE flows are safe

	nextENBUEID int64
	nextTEID    uint32

	// S1-MME peer management. Ordered list of MME endpoints; the eNB keeps
	// exactly one active SCTP association at a time, starting with index 0
	// and falling through on read/dial/S1-Setup failure. Guarded by mmeMu.
	mmeLocal    *sctp.SCTPAddr
	mmeMu       sync.RWMutex
	peers       []*mmePeer
	active      int           // index into peers; -1 when no active peer
	mmeShutdown bool          // set by Close; suppresses failover promotion
	mmeChange   chan struct{} // closed on every active-peer transition
}

// mmePeer is one ordered S1-MME endpoint.
type mmePeer struct {
	address string
	conn    *sctp.SCTPConn
	state   mmePeerState
}

type mmePeerState uint8

const (
	mmeStatePending mmePeerState = iota
	mmeStateActive
	mmeStateFailed
)

// Start dials the MME's S1-MME endpoint, begins receiving, sends an S1 Setup
// Request, and returns once the S1 Setup Response arrives. With multiple
// addresses (CoreS1MMEAddresses) the eNB tries them in order and keeps the
// first reachable one active, falling over to the next on failure.
func Start(opts *StartOpts) (*ENB, error) {
	plmn, err := plmnOctets(opts.MCC, opts.MNC)
	if err != nil {
		return nil, err
	}

	tac, err := parseTAC(opts.TAC)
	if err != nil {
		return nil, err
	}

	addresses := opts.CoreS1MMEAddresses
	if len(addresses) == 0 {
		addresses = []string{opts.CoreS1MMEAddress}
	}

	if len(addresses) == 0 || addresses[0] == "" {
		return nil, fmt.Errorf("s1enb: no S1-MME address configured")
	}

	var local *sctp.SCTPAddr
	if opts.ENBAddress != "" {
		local = &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP(opts.ENBAddress)}}}
	}

	n3 := opts.ENBN3Address
	if n3 == "" {
		n3 = opts.ENBAddress
	}

	n3IP := net.ParseIP(n3)
	if n3IP == nil {
		n3IP = net.IPv4(127, 0, 0, 1)
	}

	peers := make([]*mmePeer, len(addresses))
	for i, a := range addresses {
		peers[i] = &mmePeer{address: a, state: mmeStatePending}
	}

	e := &ENB{
		enbID:          opts.ENBID,
		name:           opts.Name,
		plmn:           plmn,
		tac:            tac,
		n3Addr:         n3IP,
		tunnels:        make(map[uint32]*tunnel),
		receivedFrames: make(map[Category]map[s1ap.ProcedureCode][]Frame),
		nextENBUEID:    1,
		nextTEID:       1,
		mmeLocal:       local,
		peers:          peers,
		active:         -1,
		mmeChange:      make(chan struct{}),
	}
	e.cond = sync.NewCond(&e.mu)

	// Open the S1-U (GTP-U) socket so connectivity scenarios can run a datapath.
	if opts.EnableDatapath {
		if opts.ENBN3Address == "" {
			return nil, fmt.Errorf("s1enb: EnableDatapath requires ENBN3Address")
		}

		n3Conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: n3IP, Port: gtpUDPPort})
		if err != nil {
			return nil, fmt.Errorf("listen S1-U UDP on %s: %w", opts.ENBN3Address, err)
		}

		e.n3Conn = n3Conn

		go e.gtpReader()
	}

	e.mmeMu.Lock()

	var lastErr error

	activated := false

	for idx := range e.peers {
		if err := e.dialAndActivateLocked(idx); err != nil {
			lastErr = err
			continue
		}

		activated = true

		break
	}

	e.mmeMu.Unlock()

	if !activated {
		if e.n3Conn != nil {
			_ = e.n3Conn.Close()
		}

		return nil, fmt.Errorf("no S1-MME peer reachable: %w", lastErr)
	}

	if opts.SkipS1SetupWait {
		return e, nil
	}

	if _, err := e.WaitForMessage(NoUEID, Successful, s1ap.ProcS1Setup, 5*time.Second); err != nil {
		_ = e.Close()
		return nil, fmt.Errorf("S1 Setup did not complete: %w", err)
	}

	logger.GnbLogger.Debug("S1 Setup complete", zap.String("enb", opts.Name), zap.Uint32("enb-id", opts.ENBID))

	return e, nil
}

// dialAndActivateLocked dials the peer at peers[idx], marks it active, starts
// its receiver goroutine, and sends an S1 Setup Request. Must be called with
// mmeMu write-held. On failure it marks the peer failed and returns the error
// so the caller can continue iterating.
func (e *ENB) dialAndActivateLocked(idx int) error {
	peer := e.peers[idx]

	rem, err := sctp.ResolveSCTPAddr("sctp", peer.address)
	if err != nil {
		peer.state = mmeStateFailed
		return fmt.Errorf("resolve %s: %w", peer.address, err)
	}

	conn, err := sctp.DialSCTPExt("sctp", e.mmeLocal, rem, sctp.InitMsg{NumOstreams: 2, MaxInstreams: 2})
	if err != nil {
		peer.state = mmeStateFailed
		return fmt.Errorf("dial %s: %w", peer.address, err)
	}

	if err := conn.SubscribeEvents(sctp.SCTP_EVENT_DATA_IO); err != nil {
		_ = conn.Close()
		peer.state = mmeStateFailed

		return fmt.Errorf("subscribe SCTP events on %s: %w", peer.address, err)
	}

	peer.conn = conn
	peer.state = mmeStateActive
	e.active = idx

	go e.runReceiver(idx, conn)

	if err := e.sendS1SetupOnConn(conn); err != nil {
		_ = conn.Close()
		peer.conn = nil
		peer.state = mmeStateFailed
		e.active = -1

		return fmt.Errorf("S1SetupRequest on %s: %w", peer.address, err)
	}

	e.signalChangeLocked()

	logger.GnbLogger.Info(
		"s1enb: active S1-MME peer set",
		zap.String("address", peer.address),
		zap.Int("index", idx),
	)

	return nil
}

// signalChangeLocked closes and replaces the change channel, waking any
// WaitForActivePeerChange waiter. Must be called with mmeMu write-held.
func (e *ENB) signalChangeLocked() {
	close(e.mmeChange)
	e.mmeChange = make(chan struct{})
}

// promoteNextFromReceiver is called by a receiver goroutine when its SCTP read
// errors. It advances the active peer to the next reachable candidate in wrap
// order. When no candidate remains, it marks the eNB closed so waiters in
// WaitForMessage unblock.
func (e *ENB) promoteNextFromReceiver(failedIdx int, failedConn *sctp.SCTPConn) {
	e.mmeMu.Lock()
	defer e.mmeMu.Unlock()

	if e.mmeShutdown {
		return
	}

	if e.active != failedIdx {
		return
	}

	peer := e.peers[failedIdx]
	peer.state = mmeStateFailed

	if peer.conn != nil && peer.conn == failedConn {
		_ = peer.conn.Close()
	}

	peer.conn = nil
	e.active = -1

	n := len(e.peers)
	for step := 1; step < n; step++ {
		cand := (failedIdx + step) % n
		if e.peers[cand].state == mmeStateFailed {
			continue
		}

		if err := e.dialAndActivateLocked(cand); err != nil {
			logger.GnbLogger.Warn(
				"s1enb failover: peer unreachable",
				zap.String("address", e.peers[cand].address),
				zap.Error(err),
			)

			continue
		}

		return
	}

	e.signalChangeLocked()

	e.mu.Lock()
	e.closed = true
	e.mu.Unlock()
	e.cond.Broadcast()

	logger.GnbLogger.Error("s1enb failover: no remaining S1-MME peers")
}

// ActiveMMEAddress returns the current active peer's address, or the empty
// string when no peer is active.
func (e *ENB) ActiveMMEAddress() string {
	e.mmeMu.RLock()
	defer e.mmeMu.RUnlock()

	if e.active < 0 || e.active >= len(e.peers) {
		return ""
	}

	return e.peers[e.active].address
}

// WaitForActivePeerChange blocks until the active peer transitions (to a new
// peer or to no-active), or ctx is cancelled. Returns the new active peer's
// address (empty when no active peer).
func (e *ENB) WaitForActivePeerChange(ctx context.Context) (string, error) {
	e.mmeMu.RLock()
	ch := e.mmeChange
	e.mmeMu.RUnlock()

	select {
	case <-ch:
		return e.ActiveMMEAddress(), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// AllocateENBUEID returns a fresh eNB UE S1AP ID for a new UE.
func (e *ENB) AllocateENBUEID() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	id := e.nextENBUEID
	e.nextENBUEID++

	return id
}

// WaitForMessage blocks until an inbound S1AP PDU of the given category and
// procedure code is available (consuming it), or the timeout elapses.
// WaitForMessage blocks until a frame of (cat, code) targeting enbUEID arrives, or
// timeout elapses. enbUEID is NoUEID for non-UE-associated messages (S1 Setup,
// Reset). Filtering by enbUEID lets concurrent per-UE flows share one association
// without consuming each other's frames.
func (e *ENB) WaitForMessage(enbUEID int64, cat Category, code s1ap.ProcedureCode, timeout time.Duration) (Frame, error) {
	deadline := time.Now().Add(timeout)

	timer := time.AfterFunc(timeout, func() { e.cond.Broadcast() })
	defer timer.Stop()

	e.mu.Lock()
	defer e.mu.Unlock()

	for {
		if byCode, ok := e.receivedFrames[cat]; ok {
			frames := byCode[code]

			for i, f := range frames {
				if f.ENBUES1APID != enbUEID {
					continue
				}

				rest := append(frames[:i:i], frames[i+1:]...)
				if len(rest) == 0 {
					delete(byCode, code)
				} else {
					byCode[code] = rest
				}

				return f, nil
			}
		}

		if e.closed {
			return Frame{}, errors.New("s1enb: connection closed")
		}

		if time.Now().After(deadline) {
			return Frame{}, fmt.Errorf("s1enb: timeout waiting for %s", messageName(cat, code))
		}

		e.cond.Wait()
	}
}

// SendMessage writes a marshalled S1AP PDU to the active MME peer. ueAssociated
// selects the SCTP stream (0 for non-UE procedures, 1 for UE-associated),
// matching the NGAP tester's convention.
func (e *ENB) SendMessage(pdu []byte, ueAssociated bool) error {
	e.mmeMu.RLock()

	var conn *sctp.SCTPConn
	if e.active >= 0 && e.active < len(e.peers) {
		conn = e.peers[e.active].conn
	}

	e.mmeMu.RUnlock()

	if conn == nil {
		return ErrNoActiveMME
	}

	e.sendMu.Lock()
	defer e.sendMu.Unlock()

	return writeMessage(conn, pdu, ueAssociated)
}

// writeMessage writes a marshalled S1AP PDU to a specific SCTP connection.
func writeMessage(conn *sctp.SCTPConn, pdu []byte, ueAssociated bool) error {
	var stream uint16
	if ueAssociated {
		stream = 1
	}

	if _, err := conn.SCTPWrite(pdu, &sctp.SndRcvInfo{Stream: stream, PPID: s1apPPID}); err != nil {
		return fmt.Errorf("s1enb: SCTP write: %w", err)
	}

	return nil
}

// sendS1SetupOnConn builds and writes an S1 Setup Request directly to conn.
// Called from the locked dial/promotion path, so it must not take mmeMu.
func (e *ENB) sendS1SetupOnConn(conn *sctp.SCTPConn) error {
	b, err := e.buildS1SetupRequest()
	if err != nil {
		return err
	}

	return writeMessage(conn, b, false)
}

// Close tears down every SCTP association and wakes any waiter. Setting the
// shutdown flag first suppresses failover promotion from the receiver
// goroutines as their reads unblock.
func (e *ENB) Close() error {
	e.mmeMu.Lock()
	alreadyShutdown := e.mmeShutdown
	e.mmeShutdown = true

	var conns []*sctp.SCTPConn

	for _, p := range e.peers {
		if p.conn != nil {
			conns = append(conns, p.conn)
			p.conn = nil
		}

		p.state = mmeStateFailed
	}

	e.active = -1
	e.mmeMu.Unlock()

	e.mu.Lock()
	if e.closed && alreadyShutdown {
		e.mu.Unlock()
		return nil
	}

	e.closed = true

	tunnels := make([]*tunnel, 0, len(e.tunnels))
	for _, t := range e.tunnels {
		tunnels = append(tunnels, t)
	}

	e.tunnels = make(map[uint32]*tunnel)
	e.mu.Unlock()
	e.cond.Broadcast()

	for _, t := range tunnels {
		t.close()
	}

	if e.n3Conn != nil {
		_ = e.n3Conn.Close()
	}

	var firstErr error

	for _, c := range conns {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// runReceiver reads S1AP PDUs off peers[idx]'s SCTP association and files them
// by category and procedure code for WaitForMessage. On read error it triggers
// promotion of the next peer and exits.
func (e *ENB) runReceiver(idx int, conn *sctp.SCTPConn) {
	buf := make([]byte, 65535)

	for {
		n, _, err := conn.SCTPRead(buf)
		if err != nil {
			e.promoteNextFromReceiver(idx, conn)
			return
		}

		if n <= 0 {
			continue
		}

		raw := make([]byte, n)
		copy(raw, buf[:n])

		f, ok := decodeFrame(raw)
		if !ok {
			logger.GnbLogger.Warn("s1enb: undecodable S1AP PDU", zap.Int("len", n))
			continue
		}

		e.mu.Lock()
		if e.receivedFrames[f.Category] == nil {
			e.receivedFrames[f.Category] = make(map[s1ap.ProcedureCode][]Frame)
		}

		e.receivedFrames[f.Category][f.ProcedureCode] = append(e.receivedFrames[f.Category][f.ProcedureCode], f)
		e.mu.Unlock()
		e.cond.Broadcast()
	}
}

func decodeFrame(raw []byte) (Frame, bool) {
	pdu, err := s1ap.Unmarshal(raw)
	if err != nil {
		return Frame{}, false
	}

	var f Frame

	switch p := pdu.(type) {
	case *s1ap.InitiatingMessage:
		f = Frame{Category: Initiating, ProcedureCode: p.ProcedureCode, Value: p.Value}
	case *s1ap.SuccessfulOutcome:
		f = Frame{Category: Successful, ProcedureCode: p.ProcedureCode, Value: p.Value}
	case *s1ap.UnsuccessfulOutcome:
		f = Frame{Category: Unsuccessful, ProcedureCode: p.ProcedureCode, Value: p.Value}
	default:
		return Frame{}, false
	}

	f.ENBUES1APID = frameENBUEID(f)

	return f, true
}

// frameENBUEID returns the eNB-UE-S1AP-ID a downlink frame targets, or NoUEID for
// non-UE-associated messages, so concurrent per-UE waits can demultiplex frames.
func frameENBUEID(f Frame) int64 {
	switch {
	case f.Category == Initiating && f.ProcedureCode == s1ap.ProcDownlinkNASTransport:
		if m, err := s1ap.ParseDownlinkNASTransport(f.Value); err == nil {
			return int64(m.ENBUES1APID)
		}
	case f.Category == Initiating && f.ProcedureCode == s1ap.ProcInitialContextSetup:
		if m, err := s1ap.ParseInitialContextSetupRequest(f.Value); err == nil {
			return int64(m.ENBUES1APID)
		}
	case f.Category == Initiating && f.ProcedureCode == s1ap.ProcUEContextRelease:
		if m, err := s1ap.ParseUEContextReleaseCommand(f.Value); err == nil {
			return int64(m.UES1APIDs.ENBUES1APID)
		}
	case f.Category == Initiating && f.ProcedureCode == s1ap.ProcERABSetup:
		if m, err := s1ap.ParseERABSetupRequest(f.Value); err == nil {
			return int64(m.ENBUES1APID)
		}
	case f.Category == Initiating && f.ProcedureCode == s1ap.ProcERABRelease:
		if m, err := s1ap.ParseERABReleaseCommand(f.Value); err == nil {
			return int64(m.ENBUES1APID)
		}
	case f.Category == Successful && f.ProcedureCode == s1ap.ProcPathSwitchRequest:
		if m, err := s1ap.ParsePathSwitchRequestAcknowledge(f.Value); err == nil {
			return int64(m.ENBUES1APID)
		}
	}

	return NoUEID
}
