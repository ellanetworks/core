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
}

// StartOpts configures an eNB simulator.
type StartOpts struct {
	ENBID            uint32 // 20-bit macro eNB ID
	MCC              string
	MNC              string
	TAC              string // hex or decimal string; parsed as the 16-bit TAC
	Name             string
	CoreS1MMEAddress string // "<ip>:<port>"; the MME's S1-MME endpoint (default port 36412)
	ENBAddress       string // local S1-MME bind address
	ENBN3Address     string // eNB S1-U (N3) address reported in bearer setup; defaults to ENBAddress
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
	conn *sctp.SCTPConn

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

	nextENBUEID int64
	nextTEID    uint32
}

// Start dials the MME's S1-MME endpoint, begins receiving, sends an S1 Setup
// Request, and returns once the S1 Setup Response arrives.
func Start(opts *StartOpts) (*ENB, error) {
	plmn, err := plmnOctets(opts.MCC, opts.MNC)
	if err != nil {
		return nil, err
	}

	tac, err := parseTAC(opts.TAC)
	if err != nil {
		return nil, err
	}

	rem, err := sctp.ResolveSCTPAddr("sctp", opts.CoreS1MMEAddress)
	if err != nil {
		return nil, fmt.Errorf("resolve S1-MME address %q: %w", opts.CoreS1MMEAddress, err)
	}

	var local *sctp.SCTPAddr
	if opts.ENBAddress != "" {
		local = &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP(opts.ENBAddress)}}}
	}

	conn, err := sctp.DialSCTPExt("sctp", local, rem, sctp.InitMsg{NumOstreams: 2, MaxInstreams: 2})
	if err != nil {
		return nil, fmt.Errorf("dial S1-MME SCTP: %w", err)
	}

	if err := conn.SubscribeEvents(sctp.SCTP_EVENT_DATA_IO); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("subscribe SCTP events: %w", err)
	}

	n3 := opts.ENBN3Address
	if n3 == "" {
		n3 = opts.ENBAddress
	}

	n3IP := net.ParseIP(n3)
	if n3IP == nil {
		n3IP = net.IPv4(127, 0, 0, 1)
	}

	e := &ENB{
		conn:           conn,
		enbID:          opts.ENBID,
		name:           opts.Name,
		plmn:           plmn,
		tac:            tac,
		n3Addr:         n3IP,
		tunnels:        make(map[uint32]*tunnel),
		receivedFrames: make(map[Category]map[s1ap.ProcedureCode][]Frame),
		nextENBUEID:    1,
		nextTEID:       1,
	}
	e.cond = sync.NewCond(&e.mu)

	// Open the S1-U (GTP-U) socket so connectivity scenarios can run a datapath.
	if opts.EnableDatapath {
		if opts.ENBN3Address == "" {
			_ = conn.Close()
			return nil, fmt.Errorf("s1enb: EnableDatapath requires ENBN3Address")
		}

		n3Conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: n3IP, Port: gtpUDPPort})
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("listen S1-U UDP on %s: %w", opts.ENBN3Address, err)
		}

		e.n3Conn = n3Conn

		go e.gtpReader()
	}

	go e.receive()

	if err := e.sendS1SetupRequest(); err != nil {
		_ = e.Close()
		return nil, err
	}

	if opts.SkipS1SetupWait {
		return e, nil
	}

	if _, err := e.WaitForMessage(Successful, s1ap.ProcS1Setup, 5*time.Second); err != nil {
		_ = e.Close()
		return nil, fmt.Errorf("S1 Setup did not complete: %w", err)
	}

	logger.GnbLogger.Debug("S1 Setup complete", zap.String("enb", opts.Name), zap.Uint32("enb-id", opts.ENBID))

	return e, nil
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
func (e *ENB) WaitForMessage(cat Category, code s1ap.ProcedureCode, timeout time.Duration) (Frame, error) {
	deadline := time.Now().Add(timeout)

	timer := time.AfterFunc(timeout, func() { e.cond.Broadcast() })
	defer timer.Stop()

	e.mu.Lock()
	defer e.mu.Unlock()

	for {
		if byCode, ok := e.receivedFrames[cat]; ok {
			if frames := byCode[code]; len(frames) > 0 {
				f := frames[0]

				if len(frames) == 1 {
					delete(byCode, code)
				} else {
					byCode[code] = frames[1:]
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

// SendMessage writes a marshalled S1AP PDU to the MME. ueAssociated selects the
// SCTP stream (0 for non-UE procedures, 1 for UE-associated), matching the
// NGAP tester's convention.
func (e *ENB) SendMessage(pdu []byte, ueAssociated bool) error {
	var stream uint16
	if ueAssociated {
		stream = 1
	}

	if _, err := e.conn.SCTPWrite(pdu, &sctp.SndRcvInfo{Stream: stream, PPID: s1apPPID}); err != nil {
		return fmt.Errorf("s1enb: SCTP write: %w", err)
	}

	return nil
}

// Close tears down the SCTP association and wakes any waiter.
func (e *ENB) Close() error {
	e.mu.Lock()
	if e.closed {
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

	return e.conn.Close()
}

// receive reads S1AP PDUs off the SCTP association and files them by category
// and procedure code for WaitForMessage.
func (e *ENB) receive() {
	buf := make([]byte, 65535)

	for {
		n, _, err := e.conn.SCTPRead(buf)
		if err != nil {
			e.mu.Lock()
			e.closed = true
			e.mu.Unlock()
			e.cond.Broadcast()

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

	switch p := pdu.(type) {
	case *s1ap.InitiatingMessage:
		return Frame{Category: Initiating, ProcedureCode: p.ProcedureCode, Value: p.Value}, true
	case *s1ap.SuccessfulOutcome:
		return Frame{Category: Successful, ProcedureCode: p.ProcedureCode, Value: p.Value}, true
	case *s1ap.UnsuccessfulOutcome:
		return Frame{Category: Unsuccessful, ProcedureCode: p.ProcedureCode, Value: p.Value}, true
	default:
		return Frame{}, false
	}
}
