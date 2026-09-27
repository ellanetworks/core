// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/ellanetworks/core/internal/tracing/attrs"
	"github.com/ellanetworks/core/nas/fgs"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type reconcileAction int

const (
	reconcileNone reconcileAction = iota
	reconcileRelease
	reconcileModify
)

type reconcileDecision struct {
	action reconcileAction
	policy *Policy
	mod    models.EPSBearerModification
}

type subscriptionDelta struct {
	FramedRoutes bool
	StaticIP     bool
}

func (s *SMF) Reconcile(ctx context.Context) {
	s.mu.RLock()
	refs := make([]string, 0, len(s.pool))

	for ref := range s.pool {
		refs = append(refs, ref)
	}

	s.mu.RUnlock()

	if len(refs) == 0 {
		return
	}

	ctx, span := tracer.Start(ctx, "smf/reconcile_sessions",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attribute.Int("reconcile.session_count", len(refs))),
	)
	defer span.End()

	for _, ref := range refs {
		if err := s.ReconcileSession(ctx, ref); err != nil {
			logger.SmfLog.Warn("session reconcile failed", logger.SMContextRef(ref), zap.Error(err))
		}
	}
}

// ReconcileSession applies an OAM-initiated policy change to an active session,
// on either access (TS 23.502 §4.3.3.2 step 1d, TS 23.401 §5.4.2.1).
//
// QoS/AMBR/DNS changes use the network-requested modification procedure: PFCP
// update plus N1+N2 to the UE and gNB, or the MME's EPS bearer modification.
//
// Slice (SST/SD), MTU, or IP pool changes release the session with cause #39
// "reactivation requested" (TS 24.501, TS 24.301) so the UE re-establishes with
// the correct configuration; TS 23.501 does not address dynamic MTU adjustment,
// and IP pools have no in-place modification mechanism.
func (s *SMF) ReconcileSession(ctx context.Context, ref string) error {
	ctx, span := tracer.Start(ctx, "smf/reconcile_session",
		trace.WithAttributes(attrs.SMContextRef(ref)),
	)
	defer span.End()

	smContext := s.GetSession(ref)
	if smContext == nil {
		return nil
	}

	smContext.Mutex.Lock()
	supi, snssai, dnn := smContext.Supi, smContext.Snssai, smContext.Dnn
	smContext.Mutex.Unlock()

	policy, _, err := s.resolveEPSPolicy(ctx, supi, dnn, snssai)
	if err != nil && !permanentPolicyFailure(err) {
		logger.SmfLog.Warn("transient error fetching session policy, skipping reconciliation",
			logger.SMContextRef(ref), zap.Error(err))

		return nil
	}

	smContext.Mutex.Lock()
	decision, err := s.reconcileLocked(ctx, smContext, policy)
	onEPS := smContext.Access == Access4G
	ebi := smContext.EBI
	smContext.Mutex.Unlock()

	if err != nil {
		span.RecordError(err)

		return err
	}

	if !onEPS {
		return nil
	}

	switch decision.action {
	case reconcileRelease:
		err = s.mme.ReactivateEPSBearer(ctx, supi.IMSI(), ebi)
	case reconcileModify:
		err = s.mme.ModifyEPSBearer(ctx, supi.IMSI(), ebi, decision.mod)
		if err != nil {
			smContext.Mutex.Lock()
			if smContext.pendingPolicy == decision.policy {
				smContext.pendingPolicy = nil
			}
			smContext.Mutex.Unlock()
		}
	}

	if errors.Is(err, ErrUENotReachable) {
		logger.SmfLog.Debug("EPS bearer not signallable, deferring reconciliation",
			logger.SUPI(supi.String()), zap.Uint8("ebi", ebi), zap.Error(err))

		return nil
	}

	return err
}

// Provisioning failures are permanent until an operator acts, so the session is
// released; a DB outage or Raft timeout is not, and the backstop retries it.
// Missing a permanent cause is the expensive mistake: the reconciler then logs
// "transient error … skipping" on every sweep, forever.
func permanentPolicyFailure(err error) bool {
	return errors.Is(err, ErrNoPolicyMatch) ||
		errors.Is(err, ErrDNNNotFound) ||
		errors.Is(err, ErrDNNNotInSlice)
}

func (s *SMF) reconcileLocked(ctx context.Context, smContext *SMContext, policy *Policy) (reconcileDecision, error) {
	if smContext.Tunnel == nil || smContext.PolicyData == nil || smContext.releasing || smContext.pending != nil || smContext.activating {
		return reconcileDecision{}, nil
	}

	// A network-requested modification or release is already outstanding
	// (T3591/T3592 running); re-firing resends the command and resets the
	// retransmission counter, or double-frees on release. Defer to the next
	// backstop sweep.
	if smContext.procedureTimer.Active() || (smContext.Access == Access4G && smContext.pendingPolicy != nil) {
		logger.SmfLog.Debug("a network-requested procedure is in flight, skipping reconciliation",
			logger.SUPI(smContext.Supi.String()),
			logger.PDUSessionID(smContext.PDUSessionID),
		)

		return reconcileDecision{}, nil
	}

	// Slice (SST/SD) change: stored Snssai matches no configured slice. Release
	// with cause #39 so the UE re-establishes on the new slice (TS 23.502).
	if policy == nil {
		return s.releaseForReactivation(ctx, smContext)
	}

	if smContext.userPlaneStale {
		if err := s.updatePFCPRules(ctx, smContext, smContext.PolicyData); err != nil {
			return reconcileDecision{}, fmt.Errorf("re-apply the committed policy to the UPF: %w", err)
		}

		smContext.userPlaneStale = false
	}

	current := smContext.PolicyData

	// MTU or IP pool change: release for re-establishment. Zero/empty values
	// mean "unspecified" (unchanged), avoiding spurious releases on a partial
	// policy.
	mtuChanged := policy.MTU != 0 && current.MTU != policy.MTU
	ipv4PoolChanged := policy.IPv4Pool != "" && current.IPv4Pool != policy.IPv4Pool
	ipv6PoolChanged := policy.IPv6Pool != "" && current.IPv6Pool != policy.IPv6Pool

	if mtuChanged || ipv4PoolChanged || ipv6PoolChanged {
		logger.SmfLog.Info("MTU or IP pool changed, releasing session for re-establishment",
			logger.SUPI(smContext.Supi.String()),
			logger.PDUSessionID(smContext.PDUSessionID),
			zap.Uint16("old_mtu", current.MTU),
			zap.Uint16("new_mtu", policy.MTU),
			zap.String("old_ipv4_pool", current.IPv4Pool),
			zap.String("new_ipv4_pool", policy.IPv4Pool),
			zap.String("old_ipv6_pool", current.IPv6Pool),
			zap.String("new_ipv6_pool", policy.IPv6Pool),
		)

		return s.releaseForReactivation(ctx, smContext)
	}

	delta, err := s.subscriptionChanged(ctx, smContext)
	if err != nil {
		logger.SmfLog.Warn("failed to evaluate the subscription during reconciliation; deferring to backstop",
			logger.SUPI(smContext.Supi.String()),
			logger.PDUSessionID(smContext.PDUSessionID),
			zap.Error(err),
		)

		return reconcileDecision{}, nil
	}

	// A framed-route change cannot be applied in place: TS 23.501 §5.6.14 requires
	// the SMF to release the PDU session (framed routes are provisioned in the
	// session's downlink PDRs) so the UE re-establishes with the new routes.
	// Checked before the in-place QoS/AMBR path so a framed-only change releases.
	if delta.FramedRoutes {
		logger.SmfLog.Info("framed routes changed, releasing session for re-establishment",
			logger.SUPI(smContext.Supi.String()),
			logger.PDUSessionID(smContext.PDUSessionID),
		)

		return s.releaseForReactivation(ctx, smContext)
	}

	// The UE IP is fixed for the session lifetime (TS 23.501 §5.8.2.2); a
	// reservation change requires release, not in-place modification.
	if delta.StaticIP {
		logger.SmfLog.Info("static IP changed, releasing session for re-establishment",
			logger.SUPI(smContext.Supi.String()),
			logger.PDUSessionID(smContext.PDUSessionID),
		)

		return s.releaseForReactivation(ctx, smContext)
	}

	if policy.PolicyID != current.PolicyID {
		rebound := *current
		rebound.PolicyID = policy.PolicyID
		rebound.NetworkRules = policy.NetworkRules

		if err := s.updatePFCPRules(ctx, smContext, &rebound); err != nil {
			return reconcileDecision{}, fmt.Errorf("bind the session to policy %q: %w", policy.PolicyID, err)
		}

		smContext.PolicyData = &rebound
		current = &rebound
	}

	oldQoS := current.QosData

	oldArp, newArp := int32(0), int32(0)
	if oldQoS.Arp != nil {
		oldArp = oldQoS.Arp.PriorityLevel
	}

	if policy.QosData.Arp != nil {
		newArp = policy.QosData.Arp.PriorityLevel
	}

	has5QIChange := oldQoS.Var5qi != policy.QosData.Var5qi
	hasQoSChange := has5QIChange || oldArp != newArp
	hasAmbrChange := !current.Ambr.Uplink.Equal(policy.Ambr.Uplink) || !current.Ambr.Downlink.Equal(policy.Ambr.Downlink)
	hasDNSChange := policy.DNS != nil && !policy.DNS.Equal(current.DNS)

	if !hasQoSChange && !hasAmbrChange && !hasDNSChange {
		return reconcileDecision{}, nil
	}

	dns := current.DNS
	if hasDNSChange {
		dns = policy.DNS
	}

	newPolicy := &Policy{
		PolicyID: current.PolicyID,
		Ambr:     policy.Ambr,
		QosData: models.QosData{
			QFI:    current.QosData.QFI,
			Var5qi: policy.QosData.Var5qi,
			Arp:    policy.QosData.Arp,
		},
		NetworkRules: current.NetworkRules,
		DNS:          dns,
		MTU:          current.MTU,
		IPv4Pool:     current.IPv4Pool,
		IPv6Pool:     current.IPv6Pool,
	}

	if smContext.Access == Access5G && hasQoSChange && !has5QIChange && !hasAmbrChange && !hasDNSChange && !smContext.upConnectionActive() {
		smContext.PolicyData = newPolicy

		return reconcileDecision{}, nil
	}

	if smContext.Access == Access4G {
		mod, err := epsBearerModification(smContext, newPolicy, hasQoSChange, has5QIChange, hasAmbrChange, hasDNSChange)
		if err != nil {
			return reconcileDecision{}, err
		}

		smContext.pendingPolicy = newPolicy

		return reconcileDecision{action: reconcileModify, policy: newPolicy, mod: mod}, nil
	}

	if err := s.sendSessionModification(ctx, smContext, newPolicy, hasAmbrChange, hasQoSChange, has5QIChange, hasDNSChange); err != nil {
		if !errors.Is(err, ErrUENotReachable) {
			logger.SmfLog.Error("failed to send session modification to UE/gNB",
				zap.Error(err),
				logger.SUPI(smContext.Supi.String()),
				logger.PDUSessionID(smContext.PDUSessionID),
			)

			return reconcileDecision{}, fmt.Errorf("failed to send session modification: %w", err)
		}

		logger.SmfLog.Debug("UE not reachable, deferring the modification until it returns",
			logger.SUPI(smContext.Supi.String()),
			logger.PDUSessionID(smContext.PDUSessionID),
		)

		return reconcileDecision{}, nil
	}

	// UE connected: the modification command is outstanding. Commit only when
	// the UE answers PDU SESSION MODIFICATION COMPLETE (TS 24.501 §6.3.2.2); a
	// reject or T3591 abort discards this and keeps the previous configuration
	// (§6.3.2.5), which the backstop then re-attempts.
	smContext.pendingPolicy = newPolicy

	return reconcileDecision{}, nil
}

func (s *SMF) releaseForReactivation(ctx context.Context, smContext *SMContext) (reconcileDecision, error) {
	if smContext.Access == Access4G {
		return reconcileDecision{action: reconcileRelease}, nil
	}

	return reconcileDecision{}, s.sendSessionRelease(ctx, smContext)
}

func epsBearerModification(smContext *SMContext, policy *Policy, hasQoSChange, has5QIChange, hasAmbrChange, hasDNSChange bool) (models.EPSBearerModification, error) {
	var mod models.EPSBearerModification

	qos := epsBearerQoS(policy)

	if hasQoSChange {
		mod.QoS = &qos
	}

	if hasAmbrChange {
		mod.APNAMBR = &policy.Ambr
	}

	if hasDNSChange {
		if addr, ok := netip.AddrFromSlice(policy.DNS); ok {
			mod.DNS = addr.Unmap()
		}

		mod.MTU = policy.MTU
	}

	if (has5QIChange || hasAmbrChange) && smContext.PDUSessionID != 0 && smContext.Snssai != nil {
		mapped, err := nas.MappedFiveGSQoSRefresh(smContext.EBI, &policy.QosData, &policy.Ambr)
		if err != nil {
			return models.EPSBearerModification{}, fmt.Errorf("encode the mapped 5GS QoS parameters: %w", err)
		}

		mod.MappedFiveGSQoS = mapped
	}

	return mod, nil
}

func (s *SMF) CommitEPSBearerModification(ctx context.Context, ref string, accepted bool) {
	smContext := s.GetSession(ref)
	if smContext == nil {
		return
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.Access != Access4G {
		return
	}

	if !accepted {
		smContext.pendingPolicy = nil
		return
	}

	s.commitPendingPolicy(ctx, smContext)
}

func (s *SMF) commitPendingPolicy(ctx context.Context, smContext *SMContext) {
	pending := smContext.pendingPolicy
	smContext.pendingPolicy = nil

	if pending == nil {
		return
	}

	current := smContext.PolicyData
	if current == nil || current.QosData.QFI != pending.QosData.QFI ||
		!current.Ambr.Uplink.Equal(pending.Ambr.Uplink) || !current.Ambr.Downlink.Equal(pending.Ambr.Downlink) {
		if err := s.updatePFCPRules(ctx, smContext, pending); err != nil {
			logger.From(ctx, logger.SmfLog).Warn("failed to apply the accepted policy to the UPF; the next reconcile retries it",
				zap.Error(err), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

			smContext.userPlaneStale = true
		}
	}

	smContext.PolicyData = pending
}

// sendSessionModification builds and sends N1+N2 for the network-requested PDU
// Session Modification (TS 23.502): the PDU Session Modification Command (N1, to
// UE) and the PDU Session Resource Modify Request Transfer (N2, to gNB), each
// carrying only the changed AMBR/QoS IEs (TS 24.501). A DNS-only change sends N1
// alone, since DNS travels in the NAS Extended PCO and does not affect the gNB.
func (s *SMF) sendSessionModification(ctx context.Context, smContext *SMContext, policy *Policy, hasAmbrChange, hasQoSChange, has5QIChange, hasDNSChange bool) error {
	var n1Ambr *models.Ambr
	if hasAmbrChange {
		n1Ambr = &policy.Ambr
	}

	var n1QoS *models.QosData
	if hasQoSChange {
		n1QoS = &policy.QosData
	}

	var n1DNS net.IP
	if hasDNSChange && policy.DNS != nil {
		n1DNS = policy.DNS
	}

	var mappedEPSQoS *nas.MappedEPSQoS
	if smContext.EBI != 0 && (has5QIChange || hasAmbrChange) {
		mappedEPSQoS = &nas.MappedEPSQoS{QosData: policy.QosData, Ambr: policy.Ambr}
	}

	n1Msg, err := nas.BuildPDUSessionModificationCommand(smContext.PDUSessionID, networkRequestedPTI, n1Ambr, n1QoS, n1DNS, smContext.EBI, mappedEPSQoS, nil)
	if err != nil {
		return fmt.Errorf("build PDU Session Modification Command (N1): %w", err)
	}

	// DNS travels in the NAS Extended PCO and needs no N2 signaling.
	var n2Msg []byte

	if (hasAmbrChange || hasQoSChange) && smContext.upConnectionActive() {
		var n2Ambr *models.Ambr
		if hasAmbrChange {
			n2Ambr = &policy.Ambr
		}

		var n2QoS *models.QosData
		if hasQoSChange {
			n2QoS = &policy.QosData
		}

		n2Msg, err = ngap.BuildPDUSessionResourceModifyRequestTransfer(n2Ambr, n2QoS)
		if err != nil {
			return fmt.Errorf("build PDU Session Resource Modify Request Transfer (N2): %w", err)
		}
	}

	if err := s.amf.ModifyN1N2(ctx, smContext.Supi, smContext.PDUSessionID, n1Msg, n2Msg); err != nil {
		return fmt.Errorf("transfer N1N2 message: %w", err)
	}

	// A network-requested modification uses PTI "no procedure transaction
	// identity assigned" (0) and awaits the UE's Modification Complete or
	// Command Reject (TS 24.501).
	smContext.MarkPTIInUse(networkRequestedPTI)

	// T3591 retransmits the command until the UE replies; on the final expiry
	// the procedure is aborted and the session stays PDU SESSION ACTIVE
	// (TS 24.501).
	supi := smContext.Supi
	pduSessionID := smContext.PDUSessionID
	s.armRetransmit(ctx, smContext, s.timerT3591(),
		func(ctx context.Context) error { return s.amf.ModifyN1N2(ctx, supi, pduSessionID, n1Msg, n2Msg) },
		func(ctx context.Context, sc *SMContext) {
			sc.ClearPTIInUse(networkRequestedPTI)
			// Discard the uncommitted policy: the UE never confirmed, so the session
			// keeps its previous configuration and the backstop re-attempts (TS 24.501
			// §6.3.2.5).
			sc.pendingPolicy = nil

			logger.SmfLog.Warn("T3591 expired; PDU session modification aborted, session remains active",
				logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID))
		})

	logger.SmfLog.Info("session modification N1+N2 sent",
		logger.SUPI(smContext.Supi.String()),
		logger.PDUSessionID(smContext.PDUSessionID),
		zap.Bool("ambr_change", hasAmbrChange),
		zap.Bool("qos_change", hasQoSChange),
		zap.Bool("dns_change", hasDNSChange),
		zap.Bool("mapped_eps_bearer_refresh", mappedEPSQoS != nil),
	)

	return nil
}

// updatePFCPRules pushes the policy's QoS (QFI + session-AMBR) to the UPF data
// plane (TS 29.244).
func (s *SMF) updatePFCPRules(ctx context.Context, smContext *SMContext, policy *Policy) error {
	return s.applySessionQERs(ctx, smContext, policy.PolicyID, policy.QosData.QFI, policy.Ambr.Uplink, policy.Ambr.Downlink)
}

func (s *SMF) applySessionQERs(ctx context.Context, smContext *SMContext, policyID string, qfi uint8, ambrUplink, ambrDownlink models.BitRate) error {
	if smContext.PFCPContext == nil || !smContext.PFCPContext.Established {
		return fmt.Errorf("PFCP session not established")
	}

	if smContext.Tunnel == nil {
		return fmt.Errorf("data path not available")
	}

	next := smContext.Tunnel.dataPlane
	next.QFI = qfi
	next.AMBR = models.Ambr{Uplink: ambrUplink, Downlink: ambrDownlink}

	return s.applyDataPlane(ctx, smContext, next, policyID)
}

func (s *SMF) subscriptionChanged(ctx context.Context, smContext *SMContext) (subscriptionDelta, error) {
	dn, err := s.store.ResolveDNN(ctx, smContext.Dnn)
	if err != nil {
		return subscriptionDelta{}, fmt.Errorf("resolve data network: %w", err)
	}

	framed, err := framedRoutesChanged(ctx, dn, smContext)
	if err != nil {
		return subscriptionDelta{}, fmt.Errorf("framed routes: %w", err)
	}

	if framed {
		return subscriptionDelta{FramedRoutes: true}, nil
	}

	static, err := staticIPChanged(ctx, dn, smContext)
	if err != nil {
		return subscriptionDelta{}, fmt.Errorf("static IP: %w", err)
	}

	return subscriptionDelta{StaticIP: static}, nil
}

// framedRoutesChanged reports whether the subscriber's currently provisioned
// framed routes differ from those installed on the session at establishment.
// Caller holds smContext.Mutex.
func framedRoutesChanged(ctx context.Context, dn DNNStore, smContext *SMContext) (bool, error) {
	current, err := dn.ListFramedRoutes(ctx, smContext.Supi.IMSI())
	if err != nil {
		return false, err
	}

	return !framedRoutesEqual(current, smContext.FramedRoutes), nil
}

// framedRoutesEqual compares two prefix sets independent of order. Both sides
// are normalized (masked) at the source, so prefix equality is exact.
func framedRoutesEqual(a, b []netip.Prefix) bool {
	if len(a) != len(b) {
		return false
	}

	counts := make(map[netip.Prefix]int, len(a))
	for _, p := range a {
		counts[p]++
	}

	for _, p := range b {
		counts[p]--
		if counts[p] < 0 {
			return false
		}
	}

	return true
}

// staticIPChanged reports whether the subscriber's reserved static IP changed
// since it was cached at establishment. Caller holds smContext.Mutex.
func staticIPChanged(ctx context.Context, dn DNNStore, smContext *SMContext) (bool, error) {
	imsi := smContext.Supi.IMSI()

	if smContext.PDUIPV4Address != nil {
		changed, err := staticReservationChanged(ctx, dn, imsi, false, smContext.StaticIPv4)
		if err != nil || changed {
			return changed, err
		}
	}

	if smContext.PDUIPV6Prefix != nil {
		changed, err := staticReservationChanged(ctx, dn, imsi, true, smContext.StaticIPv6)
		if err != nil || changed {
			return changed, err
		}
	}

	return false, nil
}

func staticReservationChanged(ctx context.Context, dn DNNStore, imsi string, ipv6 bool, cached netip.Addr) (bool, error) {
	current, has, err := dn.GetStaticIP(ctx, imsi, ipv6)
	if err != nil {
		return false, err
	}

	if has != cached.IsValid() {
		return true, nil
	}

	return has && current != cached, nil
}

// sendSessionRelease performs the network-requested PDU session release
// (TS 23.502, TS 24.501) with cause #39 "reactivation requested" so the UE
// re-establishes on the correct slice. Caller must hold smContext.Mutex.
func (s *SMF) sendSessionRelease(ctx context.Context, smContext *SMContext) error {
	return s.startRelease(ctx, smContext, 0, fgs.GSMCauseReactivationRequested)
}
