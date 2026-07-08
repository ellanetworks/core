// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"fmt"
	"net"
	"sync"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/models"
)

type PFCPSessionContext struct {
	LocalSEID  uint64
	RemoteSEID uint64
}

type UPTunnel struct {
	DataPath      *DataPath
	ANInformation struct {
		IPv4Address net.IP
		IPv6Address net.IP
		TEID        uint32
	}
}

type SMContext struct {
	Mutex sync.Mutex

	// Ref is the session's unique pool key, assigned once at creation and never
	// reused. Two sessions for the same (SUPI, PDU session id) — e.g. an old context
	// superseded by a re-attach and its replacement — get distinct Refs, so a release
	// targets the exact instance and can never tear down a newer session that reused
	// the (SUPI, id) slot. CanonicalName(SUPI, id) is the separate secondary index key.
	Ref string

	Supi                           etsi.SUPI
	Dnn                            string
	Snssai                         *models.Snssai
	Tunnel                         *UPTunnel
	PolicyData                     *Policy
	PFCPContext                    *PFCPSessionContext
	PDUSessionID                   uint8
	PDUIPV4Address                 net.IP
	PDUIPV6Prefix                  net.IP  // delegated /64 prefix base address (lower 64 bits = 0)
	IPv6IID                        [8]byte // random Interface Identifier sent to UE
	PDUSessionType                 uint8   // negotiated type: nasMessage.PDUSessionTypeIPv4/IPv6/IPv4IPv6
	PDUSessionReleaseDueToDupPduID bool

	// Access is the radio access the session was established over. Access4G marks
	// a 4G EPS session (PGW-C role): its PDUSessionID is the default bearer's EPS
	// bearer identity (5..15), which overlaps the 5G PDU session id range, so the
	// RAT cannot be inferred from the id. Downlink data for an EPS session is
	// paged via the MME (TS 23.401 §5.3.4.3), not the 5G N2 path.
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

	// pendingPolicy holds the policy of an outstanding network-requested modification
	// sent to a connected UE, committed to PolicyData only when the UE answers PDU
	// SESSION MODIFICATION COMPLETE (TS 24.501 §6.3.2.2 "consider the PDU session as
	// modified"); a reject or T3591 abort discards it, keeping the previous
	// configuration (§6.3.2.5). Guarded by Mutex.
	pendingPolicy *Policy
}

// stopProcedureTimer stops the retransmission guard. Safe to call when none is
// armed. Caller must hold Mutex.
func (smContext *SMContext) stopProcedureTimer() {
	smContext.procedureTimer.Stop()
}

// upConnectionActive reports whether the session's user-plane connection is up —
// the downlink FAR is forwarding — as opposed to idle/buffering after
// DeactivateSmContext (CM-IDLE). This is the authoritative data-plane state.
// Caller must hold Mutex.
func (smContext *SMContext) upConnectionActive() bool {
	if smContext.Tunnel == nil || smContext.Tunnel.DataPath == nil {
		return false
	}

	dl := smContext.Tunnel.DataPath.DownLinkTunnel
	if dl == nil || dl.PDR == nil || dl.PDR.FAR == nil {
		return false
	}

	return dl.PDR.FAR.ApplyAction.Forw
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

// IsPTIInUse reports whether a procedure with the given PTI is outstanding.
// Caller must hold Mutex.
func (smContext *SMContext) IsPTIInUse(pti uint8) bool {
	_, ok := smContext.outstandingPTIs[pti]

	return ok
}

func CanonicalName(identifier etsi.SUPI, pduSessID uint8) string {
	return fmt.Sprintf("%s-%d", identifier.String(), pduSessID)
}

func (smContext *SMContext) SetPolicyData(policy *Policy) {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	smContext.PolicyData = policy
}

func (smContext *SMContext) SetPFCPSession(seid uint64) {
	if smContext.PFCPContext != nil {
		return
	}

	smContext.PFCPContext = &PFCPSessionContext{
		LocalSEID: seid,
	}
}

func (smContext *SMContext) CanonicalName() string {
	return CanonicalName(smContext.Supi, smContext.PDUSessionID)
}
