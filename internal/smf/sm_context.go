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
	"github.com/ellanetworks/core/internal/smf/procedure"
)

// UPFSessionContext is the session's identity toward the UPF. One identifier
// names it on both sides: the SMF allocates the SEID and the UPF keys its
// datapath state by that same value.
type UPFSessionContext struct {
	SEID uint64

	// Established records that the UPF holds the session, so a later rule change
	// is a modification of it.
	Established bool
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
	mu sync.Mutex

	// Ref is the session's unique pool key, assigned once at creation and never
	// reused: two sessions for the same slot get distinct Refs, so a release
	// targets an exact instance.
	Ref string

	Supi       etsi.SUPI
	Dnn        string
	Snssai     *models.Snssai
	Tunnel     *UPTunnel
	PolicyData *Policy
	UPFSession *UPFSessionContext

	// The PDU session half is fixed at creation; the EPS half is reassigned under
	// mu and the registry lock.
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

	// Access4G marks the PGW-C role and the S1-U user plane, with paging via the
	// MME (TS 23.401 §5.3.4.3); Access5G marks N3 with 5GSM terminated in the SMF.
	Access AccessType

	// outstandingPTIs holds the PTI of each 5GSM procedure awaiting a UE
	// completion or reject on this PDU session (TS 24.501 §7.3.1). A completion
	// or command-reject whose PTI is absent is a PTI mismatch (§7.3.1 a).
	// Guarded by mu.
	outstandingPTIs map[uint8]struct{}

	// Not guarded by mu: the registry has its own.
	procedures *procedure.Registry

	// procedureTimer is the T3591/T3592 retransmission guard for the outstanding
	// network-requested modification or release command (TS 24.501 §6.3.2.5,
	// §6.3.3). Its generation counter invalidates a firing that races a stop, so a
	// completed procedure cannot retransmit a stale command. Guarded by mu.
	procedureTimer guard.Guard

	// pendingPolicy holds the policy of an outstanding network-requested modification,
	// committed to PolicyData only when the UE answers PDU SESSION MODIFICATION
	// COMPLETE (TS 24.501 §6.3.2.2); a reject or T3591 abort discards it, keeping the
	// previous configuration (§6.3.2.5). Guarded by mu.
	pendingPolicy *Policy

	// pending is the move the UE asked for whose target access has not bound its
	// downlink. Guarded by mu.
	pending *pendingTransfer

	releasing        bool  // guarded by mu
	establishmentPTI uint8 // PTI of the Establishment Accept, 0 until sent; guarded by mu
}

// Caller must hold mu.
func (smContext *SMContext) stopProcedureTimer() {
	smContext.procedureTimer.Stop()
}

// hasUserPlane reports whether the session holds the UPF session and downlink
// rule a move rebinds. Caller must hold mu.
func (smContext *SMContext) hasUserPlane() bool {
	if smContext.UPFSession == nil || smContext.Tunnel == nil || smContext.Tunnel.DataPath == nil {
		return false
	}

	dl := smContext.Tunnel.DataPath.DownLinkTunnel

	return dl != nil && dl.PDR != nil
}

// upConnectionActive reports whether the downlink FAR is forwarding; a UE in
// CM-IDLE leaves it buffering. Caller must hold mu.
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
// on this PDU session (TS 24.501 §7.3.1). Caller must hold mu.
func (smContext *SMContext) MarkPTIInUse(pti uint8) {
	if smContext.outstandingPTIs == nil {
		smContext.outstandingPTIs = make(map[uint8]struct{})
	}

	smContext.outstandingPTIs[pti] = struct{}{}
}

// ClearPTIInUse records that the procedure with the given PTI has completed.
// Caller must hold mu.
func (smContext *SMContext) ClearPTIInUse(pti uint8) {
	delete(smContext.outstandingPTIs, pti)
}

func (smContext *SMContext) IsPTIInUse(pti uint8) bool {
	_, ok := smContext.outstandingPTIs[pti]

	return ok
}

// SetPolicyData installs the policy of the access serving the session. A move
// the target has not bound leaves the source access's policy in force, and the
// commit adopts the target's.
func (smContext *SMContext) SetPolicyData(policy *Policy) {
	smContext.mu.Lock()
	defer smContext.mu.Unlock()

	if smContext.pending != nil {
		return
	}

	smContext.PolicyData = policy
}

func (smContext *SMContext) SetUPFSession(seid uint64) {
	if smContext.UPFSession != nil {
		return
	}

	smContext.UPFSession = &UPFSessionContext{
		SEID: seid,
	}
}

// SMContextView is a session's state taken in one critical section, for a reader
// outside the package. It names no access because it changes nothing: every
// entry point that writes reaches the session through sessionFor or
// sessionBinding, which name the access they speak for.
type SMContextView struct {
	PDUSessionID                   uint8
	PDUSessionType                 uint8
	Dnn                            string
	PDUSessionReleaseDueToDupPduID bool
	PDUIPV4Address                 string
	PDUIPV6Prefix                  string
	PolicyData                     *Policy
	// AN endpoint of the access serving the session; absent when the user plane
	// has been torn down.
	AN      *ANEndpointView
	UPFSEID *uint64
}

// ANEndpointView is the RAN tunnel endpoint the downlink is bound to.
type ANEndpointView struct {
	IPAddress string
	TEID      uint32
}

func (smContext *SMContext) View() SMContextView {
	smContext.mu.Lock()
	defer smContext.mu.Unlock()

	view := SMContextView{
		PDUSessionID:                   smContext.PDUSessionID,
		PDUSessionType:                 smContext.PDUSessionType,
		Dnn:                            smContext.Dnn,
		PDUSessionReleaseDueToDupPduID: smContext.PDUSessionReleaseDueToDupPduID,
		PolicyData:                     smContext.PolicyData,
	}

	if smContext.PDUIPV4Address != nil {
		view.PDUIPV4Address = smContext.PDUIPV4Address.String()
	}

	if smContext.PDUIPV6Prefix != nil {
		view.PDUIPV6Prefix = smContext.PDUIPV6Prefix.String()
	}

	if smContext.Tunnel != nil {
		an := &ANEndpointView{TEID: smContext.Tunnel.ANInformation.TEID}
		if smContext.Tunnel.ANInformation.IPv4Address != nil {
			an.IPAddress = smContext.Tunnel.ANInformation.IPv4Address.String()
		}

		view.AN = an
	}

	if smContext.UPFSession != nil {
		seid := smContext.UPFSession.SEID
		view.UPFSEID = &seid
	}

	return view
}
