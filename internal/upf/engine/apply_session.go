// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/upf")

// ohcNoPSC mirrors the C OHC_NO_PSC modifier bit (utils/pdr.h): ORed onto a
// GTP-U outer-header-creation value to emit a plain G-PDU with no PDU Session
// Container, as required on 4G S1-U.
const ohcNoPSC uint8 = 0x10

// Apply converges the UPF's datapath to the session state the SMF states. The
// first call for a SEID creates the session; later ones reconcile it. Every
// rule the session is to have is named in desired, so a rule absent from it is
// removed and a field absent from a rule is zero.
//
// It is all-or-nothing up to the removals: a failure unwinds the datapath
// writes it made and restores the session's rules, so what the engine holds and
// what the datapath enforces never diverge.
func (conn *SessionEngine) Apply(ctx context.Context, desired *models.SessionState) (*models.SessionApplied, error) {
	if desired == nil {
		return nil, fmt.Errorf("no session state to apply")
	}

	ctx, span := tracer.Start(ctx, "upf/apply_session",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("session.operation", "apply"),
			attribute.Int64("session.seid", int64(desired.SEID)),
			attribute.String("ue.imsi", desired.IMSI),
		),
	)
	defer span.End()

	session, fresh := conn.sessionToApply(desired.SEID)

	session.opMu.Lock()
	defer session.opMu.Unlock()

	if session.deleted {
		err := fmt.Errorf("session %d is being deleted", desired.SEID)
		span.RecordError(err)

		return nil, err
	}

	// Held across resolve → apply → register so a filter slot cannot be released
	// and reassigned to another policy before this session is visible to
	// propagateFilterIndex.
	if desired.PolicyID != "" {
		conn.filterMu.RLock()
		defer conn.filterMu.RUnlock()
	}

	held := session.held()

	plan, err := planSession(held, desired,
		conn.n3AddressIPv4, conn.n3AddressIPv6,
		conn.sessionFTEIDs(desired.SEID),
		conn.resolveFilterIndexLocked,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "could not resolve the session state")

		return nil, fmt.Errorf("couldn't plan session %d: %w", desired.SEID, err)
	}

	applied, committed, err := conn.commitPlan(ctx, session, plan, fresh)

	// A committed apply joins the engine even when a withdrawal failed: its rules
	// are on the datapath, and only a session the engine holds can be torn down.
	if committed && fresh {
		conn.mu.Lock()
		conn.sessions[desired.SEID] = session
		conn.registerPolicy(plan.policyID, desired.SEID)
		conn.mu.Unlock()
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "could not converge the session")

		return nil, err
	}

	logger.WithTrace(ctx, logger.UpfLog).Debug("Applied session state",
		logger.SEID(desired.SEID), zap.Int("pdrs", len(plan.pdrs)))

	return applied, nil
}

// sessionToApply returns the session held for the SEID, or an unregistered new
// one. A new session joins the engine only once its state is on the datapath,
// so a failed first Apply leaves nothing behind.
func (conn *SessionEngine) sessionToApply(seid uint64) (*Session, bool) {
	if session := conn.GetSession(seid); session != nil {
		return session, false
	}

	return NewSession(seid), true
}

// sessionFTEIDs binds the engine's F-TEID pool to one session.
func (conn *SessionEngine) sessionFTEIDs(seid uint64) localFTEIDs {
	return localFTEIDs{
		allocate: func() (uint32, error) {
			if conn.FteIDResourceManager == nil {
				return 0, fmt.Errorf("FTEID Resource Manager is not initialized")
			}

			teid, err := conn.FteIDResourceManager.AllocateTEID(seid)
			if err != nil {
				return 0, fmt.Errorf("can't allocate TEID: no resources available")
			}

			return teid, nil
		},
		release: func(teid uint32) {
			if conn.FteIDResourceManager != nil {
				conn.FteIDResourceManager.ReleaseTEID(seid, teid)
			}
		},
	}
}

// commitPlan writes a plan to the datapath. Creates and updates run first under
// a transaction; removals run after, best-effort, so an unwind never has to
// re-create a torn-down rule.
func (conn *SessionEngine) commitPlan(ctx context.Context, session *Session, plan *sessionPlan, fresh bool) (applied *models.SessionApplied, committed bool, err error) {
	bpfObjects := conn.BpfObjects
	seid := session.SEID

	snapshot := session.snapshot()

	var txn sessionTxn

	fail := func(err error) (*models.SessionApplied, bool, error) {
		txn.rollback(ctx)
		session.restore(snapshot)

		return nil, false, err
	}

	session.putRules(plan.fars, plan.qers, plan.ueIPv4, plan.ueIPv6)

	for _, far := range plan.fars {
		go addRemoteIPToNeigh(ctx, far.RemoteIP)
	}

	for _, urrID := range plan.createURRs {
		if err := bpfObjects.NewUrr(seid, urrID); err != nil {
			return fail(fmt.Errorf("can't put URR %d: %w", urrID, err))
		}

		txn.onRollback(func() error { return bpfObjects.DeleteUrr(seid, urrID) })
		session.addURR(urrID)
	}

	for _, planned := range plan.pdrs {
		if err := conn.commitPDR(session, &txn, planned, bpfObjects); err != nil {
			return fail(err)
		}
	}

	installed := framedRouteSet(snapshot.framedRoutes)

	for _, fr := range plan.addRoutes {
		if err := bpfObjects.PutFramedDownlink(fr, ueAddressFor(fr, plan.ueIPv4, plan.ueIPv6)); err != nil {
			return fail(fmt.Errorf("couldn't apply framed route %s: %w", fr, err))
		}

		txn.onRollback(func() error { return bpfObjects.DeleteFramedDownlink(fr) })

		installed[fr] = struct{}{}
	}

	// Committed from here: the datapath carries the stated rules, and what is
	// left is withdrawing the ones the statement dropped.
	var removeErr error

	for _, stale := range plan.removePDRs {
		if err := deletePDR(seid, stale, bpfObjects, conn.FteIDResourceManager); err != nil {
			// Keep the rule so a later delete of the session can still reach it.
			removeErr = errors.Join(removeErr, fmt.Errorf("couldn't delete PDR %d: %w", stale.PdrID, err))
			continue
		}

		session.RemovePDR(stale.PdrID)
	}

	for _, urrID := range plan.removeURRs {
		if err := bpfObjects.DeleteUrr(seid, urrID); err != nil {
			removeErr = errors.Join(removeErr, fmt.Errorf("couldn't delete URR %d: %w", urrID, err))
			continue
		}

		session.removeURR(urrID)
	}

	for _, fr := range plan.removeRoutes {
		if err := bpfObjects.DeleteFramedDownlink(fr); err != nil {
			removeErr = errors.Join(removeErr, fmt.Errorf("couldn't withdraw framed route %s: %w", fr, err))
			continue
		}

		delete(installed, fr)
	}

	session.SetFramedRoutes(framedRouteList(installed))

	conn.bindPolicy(session, plan.policyID, fresh)

	if removeErr != nil {
		logger.WithTrace(ctx, logger.UpfLog).Warn("applied session state with residual withdrawal errors",
			logger.SEID(seid), zap.Error(removeErr))
	}

	return conn.appliedState(session), true, removeErr
}

// commitPDR writes one planned PDR, leaving the datapath able to unwind to what
// it held.
func (conn *SessionEngine) commitPDR(session *Session, txn *sessionTxn, planned plannedPDR, bpfObjects *ebpf.BpfObjects) error {
	spdrInfo := planned.desired

	if planned.desired.Allocated {
		txn.onRollback(func() error {
			conn.FteIDResourceManager.ReleaseTEID(session.SEID, spdrInfo.TeID)
			return nil
		})
	}

	session.PutPDR(spdrInfo.PdrID, spdrInfo)

	if !planned.changed {
		return nil
	}

	if err := applyPDR(spdrInfo, session, bpfObjects); err != nil {
		return fmt.Errorf("couldn't apply PDR %d: %w", spdrInfo.PdrID, err)
	}

	previous, held := planned.previous, planned.held

	txn.onRollback(func() error {
		if err := unapplyPDR(spdrInfo, bpfObjects); err != nil {
			return err
		}

		if held {
			return applyPDR(previous, session, bpfObjects)
		}

		return nil
	})

	// A stale entry holds a downlink slot that survives teardown, which iterates
	// the current PDR set, and source_allowed keeps authorising the old address.
	if planned.supersedes {
		if err := unapplyPDR(previous, bpfObjects); err != nil {
			return fmt.Errorf("couldn't remove the superseded entry of PDR %d: %w", spdrInfo.PdrID, err)
		}
	}

	// A restated PDR is a fresh detection state, so the session may raise a
	// downlink data notification for it again.
	bpfObjects.ClearNotified(session.SEID, uint16(spdrInfo.PdrID), spdrInfo.PdrInfo.Qer.Qfi)

	return nil
}

func framedRouteSet(routes []netip.Prefix) map[netip.Prefix]struct{} {
	set := make(map[netip.Prefix]struct{}, len(routes))
	for _, fr := range routes {
		set[fr] = struct{}{}
	}

	return set
}

func framedRouteList(set map[netip.Prefix]struct{}) []netip.Prefix {
	routes := make([]netip.Prefix, 0, len(set))
	for fr := range set {
		routes = append(routes, fr)
	}

	return routes
}

// bindPolicy points the session at the policy whose SDF filters its PDRs use.
func (conn *SessionEngine) bindPolicy(session *Session, policyID string, fresh bool) {
	previous := session.PolicyID()
	if previous == policyID {
		return
	}

	session.SetPolicyID(policyID)

	if fresh {
		return
	}

	conn.mu.Lock()
	conn.deregisterPolicy(previous, session.SEID)
	conn.registerPolicy(policyID, session.SEID)
	conn.mu.Unlock()
}

// appliedState reports the local F-TEIDs the UPF holds for the session's uplink
// PDRs, restated in full so the SMF's copy converges on every Apply.
func (conn *SessionEngine) appliedState(session *Session) *models.SessionApplied {
	n3IPv4, n3IPv6 := conn.GetAdvertisedN3Address(), conn.GetAdvertisedN3AddressIPv6()

	applied := &models.SessionApplied{}

	for _, spdrInfo := range session.ListPDRs() {
		if spdrInfo.UEIP.IsValid() || spdrInfo.TeID == 0 {
			continue
		}

		applied.UplinkPDRs = append(applied.UplinkPDRs, models.AppliedPDR{
			PDRID:  uint16(spdrInfo.PdrID),
			TEID:   spdrInfo.TeID,
			N3IPv4: n3IPv4,
			N3IPv6: n3IPv6,
		})
	}

	return applied
}

// farInfoFromModel converts a models.FAR to an ebpf.FarInfo. The forwarding
// parameters state the rule in full, so a FAR with none names no tunnel
// endpoint.
func farInfoFromModel(far models.FAR, localIPv4 netip.Addr, localIPv6 netip.Addr) ebpf.FarInfo {
	info := ebpf.FarInfo{
		Action: encodeApplyAction(far.ApplyAction),
	}

	fp := far.ForwardingParameters
	if fp == nil {
		return info
	}

	ohc := fp.OuterHeaderCreation
	if ohc == nil {
		return info
	}

	info.OuterHeaderCreation = uint8(ohc.Description >> 8)

	// S1-U bearers emit plain GTP-U with no PDU Session Container (TS 38.415: the
	// container is N3/N9-only); ohcNoPSC tells the datapath to omit it.
	if ohc.S1U {
		info.OuterHeaderCreation |= ohcNoPSC
	}

	info.TeID = ohc.TEID

	if ohc.Description == models.OuterHeaderCreationGtpUUdpIpv6 && ohc.IPv6Address != nil {
		info.LocalIP = ebpf.IPToIn6Addr(localIPv6)

		if v6 := ohc.IPv6Address.To16(); v6 != nil {
			var v6arr [16]byte

			copy(v6arr[:], v6)

			info.RemoteIP = v6arr
		}

		return info
	}

	info.LocalIP = ebpf.IPToIn6Addr(localIPv4)

	if ohc.IPv4Address != nil {
		if ip4 := ohc.IPv4Address.To4(); ip4 != nil {
			var ip4arr [4]byte

			copy(ip4arr[:], ip4)

			info.RemoteIP = ebpf.IPToIn6Addr(netip.AddrFrom4(ip4arr))
		}
	}

	return info
}

// encodeApplyAction packs ApplyAction bools into the uint8 bit layout
// expected by the eBPF data plane.
func encodeApplyAction(a models.ApplyAction) uint8 {
	var v uint8
	if a.Drop {
		v |= 0x01
	}

	if a.Forw {
		v |= 0x02
	}

	if a.Buff {
		v |= 0x04
	}

	if a.Nocp {
		v |= 0x08
	}

	if a.Dupl {
		v |= 0x10
	}

	return v
}

// qerInfoFromModel converts a models.QER to an ebpf.QerInfo.
func qerInfoFromModel(qer models.QER) ebpf.QerInfo {
	info := ebpf.QerInfo{
		Qfi: qer.QFI,
	}

	if qer.GateStatus != nil {
		info.GateStatusDL = qer.GateStatus.DLGate
		info.GateStatusUL = qer.GateStatus.ULGate
	}

	if qer.MBR != nil {
		info.MaxBitrateDL = qer.MBR.DLMBR * 1000
		info.MaxBitrateUL = qer.MBR.ULMBR * 1000
	}

	return info
}

// addRemoteIPToNeigh adds the given remote IP (as an in6_addr [16]byte) to the kernel
// neighbour table so that GTP encapsulated packets can be forwarded.
func addRemoteIPToNeigh(ctx context.Context, remoteIP [16]byte) {
	var zero [16]byte
	if remoteIP == zero {
		return
	}

	ip := ebpf.In6AddrToIP(remoteIP)
	if !ip.IsValid() {
		return
	}

	if err := kernel.AddNeighbour(ctx, ip); err != nil {
		logger.UpfLog.Warn("could not add gnb IP to neighbour list", logger.IPAddress(ip.String()), zap.Error(err))
	}
}
