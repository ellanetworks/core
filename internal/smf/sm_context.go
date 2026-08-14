// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"net"
	"net/netip"
	"sync"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/models"
)

// One SEID, not the local/remote pair PFCP defines for two nodes (TS 29.244
// §7.2.2.4.3): the UPF is in-process and keys its session map on the SMF's value.
type PFCPSessionContext struct {
	SEID uint64
	// SEID is assigned when the data path is built, before the establish, so it
	// cannot double as this flag.
	Established bool
}

// UPTunnel is a session's user plane: the facts the UPF's rules are derived
// from, plus the UPF end of the tunnel, which the UPF itself assigns at
// establish.
type UPTunnel struct {
	N3TEID uint32
	N3IPv4 netip.Addr
	N3IPv6 netip.Addr

	dataPlane
}

type SMContext struct {
	Mutex sync.Mutex

	// Ref is the session's unique pool key, assigned once at creation and never
	Ref string

	Supi        etsi.SUPI
	Dnn         string
	Snssai      *models.Snssai
	Tunnel      *UPTunnel
	PolicyData  *Policy
	PFCPContext *PFCPSessionContext

	SessionIdentity

	FramedRoutes   []netip.Prefix
	PDUIPV4Address net.IP
	PDUIPV6Prefix  net.IP // delegated /64 prefix base address (lower 64 bits = 0)
	// Reserved static address cached at establishment; invalid if dynamic.
	StaticIPv4                     netip.Addr
	StaticIPv6                     netip.Addr
	IPv6IID                        [8]byte // random Interface Identifier sent to UE
	PDUSessionType                 uint8   // negotiated type: nasMessage.PDUSessionTypeIPv4/IPv6/IPv4IPv6
	PDUSessionReleaseDueToDupPduID bool

	Access AccessType

	// outstandingPTIs holds the PTI of each 5GSM procedure awaiting a UE
	// completion or reject on this PDU session (TS 24.501 §7.3.1). A completion
	// or command-reject whose PTI is absent is a PTI mismatch (§7.3.1 a).
	// Guarded by Mutex.
	outstandingPTIs map[uint8]struct{}

	// procedureTimer is the T3591/T3592 retransmission guard for the outstanding
	// network-requested modification or release command (TS 24.501 §6.3.2.5,
	// §6.3.3). Its generation counter invalidates a firing that races a stop, so a
	// completed procedure cannot retransmit a stale command. Guarded by Mutex.
	procedureTimer guard.Guard

	// pendingPolicy holds the policy of an outstanding network-requested modification,
	// committed to PolicyData only when the UE answers PDU SESSION MODIFICATION
	// COMPLETE (TS 24.501 §6.3.2.2); a reject or T3591 abort discards it, keeping the
	// previous configuration (§6.3.2.5). Guarded by Mutex.
	pendingPolicy *Policy

	releasing                bool  // guarded by Mutex
	establishmentPTI         uint8 // PTI of the Establishment Accept, 0 until sent; guarded by Mutex
	establishmentOutstanding bool

	// The endpoint the target NG-RAN offered in its Handover Request Acknowledge
	// (TS 23.502 §4.9.1.3.2 step 10). It is bound only once the UE has arrived
	// (§4.9.1.3.3 step 10a); until then the downlink belongs to the source
	// NG-RAN. A handover that never completes drops it. Guarded by Mutex.
	handoverTargetAN *AnchorBinding

	pending *pendingTransfer

	transferGuard guard.Guard
}

// stopProcedureTimer stops the retransmission guard; safe to call when none is
// armed. Caller must hold Mutex.
func (smContext *SMContext) stopProcedureTimer() {
	smContext.procedureTimer.Stop()
}

// upConnectionActive reports whether the downlink is forwarding, as opposed to
// idle/buffering after DeactivateSmContext (CM-IDLE). Caller must hold Mutex.
func (smContext *SMContext) upConnectionActive() bool {
	return smContext.Tunnel != nil && smContext.Tunnel.Downlink == DownlinkForwarding
}

// MarkPTIInUse records that a 5GSM procedure with the given PTI is outstanding
// on this PDU session (TS 24.501 §7.3.1). Caller must hold Mutex.
func (smContext *SMContext) MarkPTIInUse(pti uint8) {
	if smContext.outstandingPTIs == nil {
		smContext.outstandingPTIs = make(map[uint8]struct{})
	}

	smContext.outstandingPTIs[pti] = struct{}{}
}

// ClearPTIInUse records that the procedure with the given PTI has completed.
// Caller must hold Mutex.
func (smContext *SMContext) ClearPTIInUse(pti uint8) {
	delete(smContext.outstandingPTIs, pti)
}

func (smContext *SMContext) IsPTIInUse(pti uint8) bool {
	_, ok := smContext.outstandingPTIs[pti]

	return ok
}

func (smContext *SMContext) SetPFCPSession(seid uint64) {
	if smContext.PFCPContext != nil {
		return
	}

	smContext.PFCPContext = &PFCPSessionContext{
		SEID: seid,
	}
}

func (smContext *SMContext) CanonicalName() string {
	return canonicalName(smContext.Supi, smContext.sessionKey())
}

func (smContext *SMContext) onEPS() bool {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	return smContext.Access == Access4G
}
