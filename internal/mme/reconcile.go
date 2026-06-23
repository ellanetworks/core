// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"net/netip"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// ReconcileDataNetwork re-evaluates every connected EPS bearer against the
// current subscription and data-network configuration. For a DNS-only change it
// updates the bearer in place with a MODIFY EPS BEARER CONTEXT REQUEST
// (TS 24.301 §6.4.2); for an IP-pool or MTU change — which the UE cannot adopt
// without a new address or link config — it deactivates the bearer with ESM
// cause #39 "reactivation requested" (TS 24.301 §6.4.4.2) so the UE
// re-establishes.
func (m *MME) ReconcileDataNetwork(ctx context.Context) {
	m.mu.Lock()

	ues := make([]*UeContext, 0, len(m.ues))
	for _, ue := range m.ues {
		ues = append(ues, ue)
	}

	m.mu.Unlock()

	for _, ue := range ues {
		m.reconcileUE(ctx, ue)
	}
}

// reconcileUE reconciles every PDN connection of a UE against the current
// data-network configuration. Only a registered UE with an active S1 connection
// is signalled; an idle UE is signalled when it returns to ECM-CONNECTED
// (reconcileBearer on the ICS Response) or by the next backstop sweep.
func (m *MME) reconcileUE(ctx context.Context, ue *UeContext) {
	if ue.emmState.load() != EMMRegistered || ue.ecmState.load() != ECMConnected {
		return
	}

	// Defer reconciliation while an S1 handover is in flight: an E-RAB Modify or
	// Release would collide with the handover's bearer signalling (TS 36.413
	// §8.4.1.2). The next sweep re-converges the UE once the handover clears.
	if m.handoverInProgress(ue) {
		return
	}

	for _, p := range m.snapshotPDNs(ue) {
		m.reconcileBearer(ctx, ue, p)
	}
}

// snapshotPDNs returns the UE's PDN connections as a slice taken under the lock,
// so the reconciler does not iterate the map while a NAS handler mutates it.
func (m *MME) snapshotPDNs(ue *UeContext) []*pdnConnection {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	out := make([]*pdnConnection, 0, len(ue.pdns))
	for _, p := range ue.pdns {
		out = append(out, p)
	}

	return out
}

// clearPendingModifyLocked clears a PDN connection's in-flight modification
// bookkeeping. The caller holds ue.mu.
func clearPendingModifyLocked(p *pdnConnection) {
	p.modifying = false
	p.pendingDNConfig = ""
	p.pendingSessAmbrDLBps = 0
	p.pendingSessAmbrULBps = 0
	p.pendingQCI = 0
	p.pendingARP = 0
}

// reconcileBearer reconciles a single PDN connection against its current policy
// and data-network configuration. A DNS and/or Session-AMBR change is applied in
// place with a Modify EPS Bearer Context; an IP-pool or MTU change reactivates the
// bearer.
func (m *MME) reconcileBearer(ctx context.Context, ue *UeContext, p *pdnConnection) {
	// Snapshot the connection's mutable policy state under the lock so a NAS
	// handler or the NAS-guard timer does not mutate the in-flight flags or the
	// stored config while the reconciler reads them.
	ue.mu.Lock()

	busy := p.deactivating || p.modifying
	curDNConfig := p.dnConfig
	curSessAmbrDLBps, curSessAmbrULBps := p.sessAmbrDLBps, p.sessAmbrULBps
	curQCI, curARP := p.qci, p.arp

	ue.mu.Unlock()

	if busy {
		return
	}

	qos, err := m.resolveQoSByAPN(ctx, ue.imsi, p.apn)
	if err != nil {
		logger.MmeLog.Warn("reconcile: failed to resolve QoS for APN",
			zap.String("imsi", ue.imsi), zap.String("apn", p.apn), zap.Error(err))

		return
	}

	newFingerprint := qos.dnFingerprint()
	dnChanged := newFingerprint != curDNConfig

	ambrChanged := bitRateToBps(qos.SessAmbrDLStr) != curSessAmbrDLBps ||
		bitRateToBps(qos.SessAmbrULStr) != curSessAmbrULBps

	qosChanged := qos.QCI != curQCI || qos.ARP != curARP

	if !dnChanged && !ambrChanged && !qosChanged {
		return
	}

	// An IP-pool or MTU change cannot be adopted in place; reactivate so the UE
	// re-establishes (the new bearer also picks up the new QoS/Session-AMBR).
	if dnChanged && !dnsOnlyChange(curDNConfig, newFingerprint) {
		logger.MmeLog.Info("data-network configuration changed; reactivating EPS bearer",
			zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.String("imsi", ue.imsi), zap.String("apn", p.apn))
		m.reactivateBearer(ctx, ue, p)

		return
	}

	logger.MmeLog.Info("policy/data-network changed; modifying EPS bearer in place",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.String("imsi", ue.imsi), zap.String("apn", p.apn),
		zap.Bool("dns", dnChanged), zap.Bool("session-ambr", ambrChanged), zap.Bool("qos", qosChanged))
	m.modifyBearer(ctx, ue, p, qos, dnChanged, ambrChanged, qosChanged)
}

// dnsOnlyChange reports whether the data-network fingerprint changed in the DNS
// field alone (IP pools and MTU unchanged), so the bearer can be modified in
// place rather than reactivated. A malformed stored fingerprint returns false,
// so the caller falls back to the safe reactivation path. The fingerprint is
// "ipv4pool|ipv6pool|dns|mtu" (epsQoS.dnFingerprint).
func dnsOnlyChange(oldFingerprint, newFingerprint string) bool {
	o := strings.Split(oldFingerprint, "|")
	n := strings.Split(newFingerprint, "|")

	if len(o) != 4 || len(n) != 4 {
		return false
	}

	return o[2] != n[2] && o[0] == n[0] && o[1] == n[1] && o[3] == n[3]
}

// modifyBearer updates an active default bearer in place with a single MODIFY EPS
// BEARER CONTEXT REQUEST (TS 24.301 §6.4.2): a changed DNS server in the Protocol
// Configuration Options (TS 24.008 §10.5.6.3) and/or the per-APN Session-AMBR
// (§9.9.4.2). The new values are committed only when the UE accepts, so an aborted
// modification leaves the stored config stale for the backstop to retry. The
// Session-AMBR is also pushed to the UPF QER so the data plane enforces it.
func (m *MME) modifyBearer(ctx context.Context, ue *UeContext, p *pdnConnection, qos *epsQoS, includeDNS, includeAMBR, includeQoS bool) {
	req := &eps.ModifyEPSBearerContextRequest{
		EPSBearerIdentity:            p.ebi,
		ProcedureTransactionIdentity: 0,
	}

	if includeQoS {
		req.NewEPSQoS = eps.EPSQoS{QCI: qos.QCI}.Marshal()
	}

	var (
		dns      netip.Addr
		dnsValid bool
	)

	if includeDNS {
		var dnsServers [][]byte

		if parsed, err := netip.ParseAddr(qos.DNS); err == nil {
			dns, dnsValid = parsed, true

			if dns.Is4() {
				b := dns.As4()
				dnsServers = [][]byte{b[:]}
			} else {
				b := dns.As16()
				dnsServers = [][]byte{b[:]}
			}
		}

		var ipv4LinkMTU uint16
		if p.pdnType == eps.PDNTypeIPv4 || p.pdnType == eps.PDNTypeIPv4v6 {
			ipv4LinkMTU = qos.MTU
		}

		req.ProtocolConfigurationOptions = eps.BuildProtocolConfigurationOptions(dnsServers, ipv4LinkMTU)
	}

	if includeAMBR {
		// Update the UPF QER (the enforcement point) before signalling the AMBR, and
		// abort on failure: signalling anyway commits the new AMBR on UE-accept while
		// the UPF stays behind, and reconcile then sees no diff to retry.
		if err := m.session.UpdateEPSSessionAMBR(ctx, ue.imsi, p.ebi, qos.SessAmbrULStr, qos.SessAmbrDLStr); err != nil {
			logger.MmeLog.Error("failed to update UPF Session-AMBR; deferring EPS bearer modification to the next reconcile",
				zap.String("imsi", ue.imsi), zap.String("apn", p.apn), zap.Error(err))

			return
		}

		req.APNAMBR = eps.APNAMBRFromBitsPerSecond(bitRateToBps(qos.SessAmbrDLStr), bitRateToBps(qos.SessAmbrULStr)).Marshal()
	}

	naspdu, err := m.protectDownlink(ue, req)
	if err != nil {
		logger.MmeLog.Error("failed to protect Modify EPS Bearer Context Request", zap.Error(err))
		return
	}

	ue.mu.Lock()
	p.modifying = true
	p.pendingDNConfig = qos.dnFingerprint()
	p.pendingSessAmbrDLBps = bitRateToBps(qos.SessAmbrDLStr)
	p.pendingSessAmbrULBps = bitRateToBps(qos.SessAmbrULStr)
	p.pendingQCI = qos.QCI
	p.pendingARP = qos.ARP

	if dnsValid {
		p.dns = dns
	}
	ue.mu.Unlock()

	if includeQoS {
		// A QCI/ARP change reconfigures the radio bearer, so the NAS message is
		// piggybacked in an S1AP E-RAB Modify Request (TS 36.413 §8.2.2).
		m.sendERABModify(ctx, ue, p, qos, naspdu)
	} else {
		// DNS and/or Session-AMBR only: no radio change, so the NAS message is sent
		// standalone in a Downlink NAS Transport (TS 23.401 §5.4.3).
		m.sendDownlink(ctx, ue, naspdu)
	}

	m.armNASGuardAbortOnly(ue, "Modify EPS Bearer Context Request", naspdu, func() {
		// An aborted modification leaves the UE connected and its data-network
		// fingerprint stale, so the backstop reconcile retries it later.
		ue.mu.Lock()
		if p := ue.defaultPDNLocked(); p != nil {
			clearPendingModifyLocked(p)
		}
		ue.mu.Unlock()
	})
}

// sendERABModify reconfigures the UE's default-bearer radio QoS with an S1AP
// E-RAB MODIFY REQUEST (TS 36.413 §8.2.2): the new E-RAB-level QoS (QCI, ARP) for
// the eNB, carrying the MODIFY EPS BEARER CONTEXT REQUEST piggybacked in the
// NAS-PDU for the UE. Completion is the NAS Modify Accept, not the E-RAB Modify
// Response, so this does not block on it.
func (m *MME) sendERABModify(ctx context.Context, ue *UeContext, p *pdnConnection, qos *epsQoS, naspdu []byte) {
	req := &s1ap.ERABModifyRequest{
		MMEUES1APID: ue.MMEUES1APID,
		ENBUES1APID: ue.ENBUES1APID,
		ERABToBeModified: []s1ap.ERABToBeModifiedItemBearerModReq{{
			ERABID: s1ap.ERABID(p.ebi),
			QoS: s1ap.ERABLevelQoSParameters{
				QCI: s1ap.QCI(qos.QCI),
				ARP: s1ap.AllocationAndRetentionPriority{
					PriorityLevel:           qos.ARP,
					PreemptionCapability:    s1ap.PreemptionShallNotTrigger,
					PreemptionVulnerability: s1ap.PreemptionNotPreemptable,
				},
			},
			NASPDU: s1ap.NASPDU(naspdu),
		}},
	}

	b, err := req.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal E-RAB Modify Request", zap.Error(err))
		return
	}

	m.sendS1AP(ctx, ue, S1APProcedureERABModifyRequest, b)
}

// handleERABModifyResponse records the eNB's E-RAB Modify outcome. The procedure
// completes on the NAS Modify Accept, so a failed-to-modify list is logged but
// does not itself abort the modification (TS 36.413 §8.2.2).
func (m *MME) handleERABModifyResponse(value []byte) {
	resp, err := s1ap.ParseERABModifyResponse(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode E-RAB Modify Response", zap.Error(err))
		return
	}

	if len(resp.ERABFailedToModify) > 0 {
		logger.MmeLog.Warn("eNB failed to modify E-RAB(s)",
			zap.Uint32("mme-ue-id", uint32(resp.MMEUES1APID)), zap.Int("failed", len(resp.ERABFailedToModify)))
	}
}

// reactivateBearer asks the UE to re-establish its PDN connection by deactivating
// the default bearer with ESM cause #39 "reactivation requested" (TS 24.301
// §6.4.4.2). The request is guarded and retransmitted until the UE answers with
// DEACTIVATE EPS BEARER CONTEXT ACCEPT.
func (m *MME) reactivateBearer(ctx context.Context, ue *UeContext, p *pdnConnection) {
	m.deactivateBearer(ctx, ue, p, eps.ESMCauseReactivationRequested, 0, false)
}

// handleESM dispatches an uplink ESM (session management) NAS message.
func (m *MME) handleESM(ctx context.Context, ue *UeContext, plain []byte) {
	mt, err := eps.PeekESMMessageType(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to read ESM message type", zap.Error(err))
		return
	}

	ctx, span := tracer.Start(ctx, "mme/esm",
		trace.WithAttributes(attribute.Int("esm.message_type", int(mt))))
	defer span.End()

	switch mt {
	case eps.MsgPDNConnectivityRequest:
		m.onPDNConnectivityRequest(ctx, ue, plain)
	case eps.MsgPDNDisconnectRequest:
		m.onPDNDisconnectRequest(ctx, ue, plain)
	case eps.MsgActivateDefaultEPSBearerContextAccept:
		m.onActivateDefaultBearerAccept(ue, plain)
	case eps.MsgActivateDefaultEPSBearerContextReject:
		m.onActivateDefaultBearerReject(ue, plain)
	case eps.MsgDeactivateEPSBearerContextAccept:
		m.onDeactivateBearerAccept(ctx, ue, plain)
	case eps.MsgModifyEPSBearerContextAccept:
		m.onModifyBearerAccept(ue, plain)
	case eps.MsgModifyEPSBearerContextReject:
		m.onModifyBearerReject(ue, plain)
	default:
		logger.MmeLog.Warn("unhandled ESM message", zap.Int("message-type-value", int(mt)))
	}
}

// onModifyBearerAccept commits the new bearer configuration once the UE accepts
// the in-place modification (TS 24.301 §6.4.2.3). The accept's EPS bearer identity
// selects the PDN connection, so a modification of an additional PDN commits to
// the right bearer.
func (m *MME) onModifyBearerAccept(ue *UeContext, plain []byte) {
	m.stopNASGuard(ue)

	p := m.defaultPDN(ue)
	if accept, err := eps.ParseModifyEPSBearerContextAccept(plain); err == nil {
		if named := m.lookupPDN(ue, accept.EPSBearerIdentity); named != nil {
			p = named
		}
	}

	if p == nil {
		return
	}

	ue.mu.Lock()
	if !p.modifying {
		ue.mu.Unlock()
		return
	}

	p.dnConfig = p.pendingDNConfig
	p.sessAmbrDLBps = p.pendingSessAmbrDLBps
	p.sessAmbrULBps = p.pendingSessAmbrULBps
	p.qci = p.pendingQCI
	p.arp = p.pendingARP
	clearPendingModifyLocked(p)
	ue.mu.Unlock()

	logger.MmeLog.Info("EPS bearer modified in place",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.String("imsi", ue.imsi), zap.String("apn", p.apn))
}

// onModifyBearerReject abandons the modification when the UE rejects it
// (TS 24.301 §6.4.2.4), leaving the stored config stale so the backstop retries.
func (m *MME) onModifyBearerReject(ue *UeContext, plain []byte) {
	m.stopNASGuard(ue)

	p := m.defaultPDN(ue)
	if rej, err := eps.ParseModifyEPSBearerContextReject(plain); err == nil {
		if named := m.lookupPDN(ue, rej.EPSBearerIdentity); named != nil {
			p = named
		}
	}

	if p != nil {
		ue.mu.Lock()
		clearPendingModifyLocked(p)
		ue.mu.Unlock()
	}

	logger.MmeLog.Warn("UE rejected EPS bearer modification",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.String("imsi", ue.imsi))
}

// onDeactivateBearerAccept finalises an EPS bearer deactivation. A deactivation
// triggered by a UE PDN disconnect releases only that PDN connection and leaves
// the UE connected (TS 24.301 §6.5.2). A deactivation with reactivation requested
// for the default bearer instead releases the S1 context so the UE re-attaches
// and picks up the new data-network configuration (TS 24.301 §6.4.4.2).
func (m *MME) onDeactivateBearerAccept(ctx context.Context, ue *UeContext, plain []byte) {
	m.stopNASGuard(ue)

	p := m.defaultPDN(ue)
	if accept, err := eps.ParseDeactivateEPSBearerContextAccept(plain); err == nil {
		if named := m.lookupPDN(ue, accept.EPSBearerIdentity); named != nil {
			p = named
		}
	}

	if p == nil {
		return
	}

	// Only reactivating the attach (first) PDN's default bearer detaches the UE so
	// it re-attaches with the new configuration (TS 24.301 §6.4.4.2). A PDN
	// disconnect, or a reactivation of an additional PDN, releases just that PDN
	// connection and leaves the UE connected.
	ue.mu.Lock()

	releaseOnly := p.ebi != ue.defaultEBI || p.disconnecting

	ue.mu.Unlock()

	if releaseOnly {
		logger.MmeLog.Info("PDN connection released",
			zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.String("imsi", ue.imsi), zap.String("apn", p.apn))
		m.releasePDN(ue, p)

		return
	}

	ue.mu.Lock()
	p.deactivating = false
	ue.mu.Unlock()

	ue.emmState.store(EMMDeregistered)
	m.releaseAllSessions(ue)

	logger.MmeLog.Info("EPS bearer deactivated for reactivation; UE will re-attach",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.String("imsi", ue.imsi))
	m.releaseUEContext(ctx, ue, causeNASNormalRelease)
}
