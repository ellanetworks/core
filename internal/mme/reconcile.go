// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"net/netip"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
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
	for _, ue := range m.ConnectedUEs() {
		m.ReconcileUE(ctx, ue)
	}
}

// ReconcileUE reconciles every PDN connection of a UE against the current
// data-network configuration. Only a registered UE with an active S1 connection
// is signalled; an idle UE is signalled when it returns to ECM-CONNECTED
// (reconcileBearer on the ICS Response) or by the next backstop sweep.
func (m *MME) ReconcileUE(ctx context.Context, ue *UeContext) {
	// ue.active is freed concurrently by a release goroutine, and reconciliation is
	// deferred while an S1 handover is in flight (an E-RAB Modify or Release would
	// collide with the handover's bearer signalling, TS 36.413 §8.4.1.2); the next
	// sweep re-converges the UE.
	ueConn, ready := m.ReconcileReady(ue)
	if !ready {
		return
	}

	for _, p := range m.SnapshotPDNs(ue) {
		m.reconcileBearer(ctx, ue, ueConn, p)
	}
}

// ClearPendingModifyLocked clears a PDN connection's in-flight modification
// bookkeeping. The caller holds ue.mu.
func ClearPendingModifyLocked(p *PdnConnection) {
	p.Modifying = false
	p.PendingDNConfig = ""
	p.PendingSessAmbrDLBps = 0
	p.PendingSessAmbrULBps = 0
	p.PendingQCI = 0
	p.PendingARP = 0
}

// reconcileBearer reconciles a single PDN connection against its current policy
// and data-network configuration.
func (m *MME) reconcileBearer(ctx context.Context, ue *UeContext, ueConn *UeConn, p *PdnConnection) {
	// Snapshot the connection's mutable policy state under the lock so a NAS
	// handler or the NAS-guard timer does not mutate the in-flight flags or the
	// stored config while the reconciler reads them.
	ue.mu.Lock()

	busy := p.Deactivating || p.Modifying
	curDNConfig := p.DnConfig
	curSessAmbrDLBps, curSessAmbrULBps := p.SessAmbrDLBps, p.SessAmbrULBps
	curQCI, curARP := p.Qci, p.Arp

	ue.mu.Unlock()

	if busy {
		return
	}

	// A framed-route change cannot be adopted in place: TS 23.501 §5.6.14 requires
	// re-establishment. Checked before the QoS diff so a framed-only change still
	// reactivates (framed routes are absent from the data-network fingerprint).
	framedChanged, err := m.Session.FramedRoutesChanged(ctx, p.SessionRef)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("reconcile: failed to check framed routes; deferring to next sweep",
			zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn), zap.Error(err))

		return
	}

	if framedChanged {
		logger.From(ctx, ueConn.Log).Info("framed routes changed; reactivating EPS bearer",
			zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn))
		m.reactivateBearer(ctx, ue, p)

		return
	}

	// The UE IP is fixed for the PDN connection lifetime (TS 23.401 §5.3.1.2.1);
	// a reservation change requires reactivation, not in-place modification.
	staticChanged, err := m.Session.StaticIPChanged(ctx, p.SessionRef)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("reconcile: failed to check static IP; deferring to next sweep",
			zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn), zap.Error(err))

		return
	}

	if staticChanged {
		logger.From(ctx, ueConn.Log).Info("static IP changed; reactivating EPS bearer",
			zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn))
		m.reactivateBearer(ctx, ue, p)

		return
	}

	qos, err := ResolveQoSByAPN(ctx, m, ue.IMSI(), p.Apn)
	if err != nil {
		// The subscriber's profile does not bind the APN: the subscription does
		// not authorize this PDN connection, so deactivate it (TS 23.401
		// §5.4.4.1), symmetric with the 5G release on an unresolvable policy. Other
		// errors are transient (DB/infra); skip and let the backstop retry.
		if errors.Is(err, ErrUnknownAPN) {
			logger.From(ctx, ueConn.Log).Info("APN no longer authorized; reactivating EPS bearer",
				zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn))
			m.reactivateBearer(ctx, ue, p)

			return
		}

		logger.From(ctx, logger.MmeLog).Warn("reconcile: failed to resolve QoS for APN; deferring to next sweep",
			zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn), zap.Error(err))

		return
	}

	newFingerprint := qos.DnFingerprint()
	dnChanged := newFingerprint != curDNConfig

	ambrChanged := qos.SessAmbrDL.Bps() != curSessAmbrDLBps ||
		qos.SessAmbrUL.Bps() != curSessAmbrULBps

	qosChanged := qos.QCI != curQCI || qos.ARP != curARP

	if !dnChanged && !ambrChanged && !qosChanged {
		return
	}

	// An IP-pool or MTU change cannot be adopted in place; reactivate so the UE
	// re-establishes (the new bearer also picks up the new QoS/Session-AMBR).
	if dnChanged && !dnsOnlyChange(curDNConfig, newFingerprint) {
		logger.From(ctx, ueConn.Log).Info("data-network configuration changed; reactivating EPS bearer",
			zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn))
		m.reactivateBearer(ctx, ue, p)

		return
	}

	logger.From(ctx, ueConn.Log).Info("policy/data-network changed; modifying EPS bearer in place",
		zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn),
		zap.Bool("dns", dnChanged), zap.Bool("session-ambr", ambrChanged), zap.Bool("qos", qosChanged))
	m.modifyBearer(ctx, ue, ueConn, p, qos, dnChanged, ambrChanged, qosChanged)
}

func dnsOnlyChange(oldFingerprint, newFingerprint string) bool {
	const fields = 5

	o := strings.Split(oldFingerprint, "|")
	n := strings.Split(newFingerprint, "|")

	if len(o) != fields || len(n) != fields {
		return false
	}

	if o[2] == n[2] {
		return false
	}

	for i := range fields {
		if i != 2 && o[i] != n[i] {
			return false
		}
	}

	return true
}

// modifyBearer updates an active default bearer in place with a single MODIFY EPS
// BEARER CONTEXT REQUEST (TS 24.301 §6.4.2): a changed DNS server in the Protocol
// Configuration Options (TS 24.008 §10.5.6.3) and/or the per-APN Session-AMBR
// (§9.9.4.2). The new values are committed only when the UE accepts, so an aborted
// modification leaves the stored config stale for the backstop to retry. The
// Session-AMBR is also pushed to the UPF QER so the data plane enforces it.
func (m *MME) modifyBearer(ctx context.Context, ue *UeContext, ueConn *UeConn, p *PdnConnection, qos *EpsQoS, includeDNS, includeAMBR, includeQoS bool) {
	req := &eps.ModifyEPSBearerContextRequest{
		EPSBearerIdentity: eps.EPSBearerIdentity(p.Ebi),
		PTI:               0,
	}

	if includeQoS {
		req.NewEPSQoS = &eps.EPSQoS{QCI: qos.QCI}
	}

	var (
		dns      netip.Addr
		dnsValid bool
	)

	refreshMappedQoS := (includeQoS || includeAMBR) && p.Snssai != nil && p.PDUSessionID != 0

	if includeDNS || refreshMappedQoS {
		var (
			dnsServers  [][]byte
			ipv4LinkMTU uint16
		)

		if includeDNS {
			if parsed, err := netip.ParseAddr(qos.DNS); err == nil {
				dns, dnsValid = parsed, true
				dnsServers = nas.DNSServers(dns)
			}

			if p.PdnType == eps.PDNTypeIPv4 || p.PdnType == eps.PDNTypeIPv4v6 {
				ipv4LinkMTU = qos.MTU
			}
		}

		pco := nas.NewProtocolConfigurationOptions(dnsServers, ipv4LinkMTU)

		if refreshMappedQoS {
			mapped, err := MappedFiveGSQoSRefresh(p.Ebi, qos)
			if err != nil {
				logger.From(ctx, logger.MmeLog).Error("failed to encode the mapped 5GS QoS parameters; deferring EPS bearer modification to the next reconcile",
					zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn), zap.Error(err))

				return
			}

			pco.Containers = append(pco.Containers, mapped...)
		}

		// TS 24.301 §8.3.18.9 and §8.3.18.13
		if ue.UsesEPCO(p) {
			req.ExtendedProtocolConfigurationOptions = &pco
		} else {
			req.ProtocolConfigurationOptions = &pco
		}
	}

	if includeAMBR {
		// Update the UPF QER (the enforcement point) before signalling the AMBR, and
		// abort on failure: signalling anyway commits the new AMBR on UE-accept while
		// the UPF stays behind, and reconcile then sees no diff to retry.
		if err := m.Session.UpdateEPSSessionAMBR(ctx, p.SessionRef, qos.SessAmbrUL, qos.SessAmbrDL); err != nil {
			logger.From(ctx, logger.MmeLog).Error("failed to update UPF Session-AMBR; deferring EPS bearer modification to the next reconcile",
				zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn), zap.Error(err))

			return
		}

		apnAMBR, err := eps.APNAMBRFromKbps(qos.SessAmbrDL.Bps()/1000, qos.SessAmbrUL.Bps()/1000)
		if err != nil {
			logger.From(ctx, logger.MmeLog).Error("failed to encode APN-AMBR",
				zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn), zap.Error(err))

			return
		}

		req.APNAMBR = &apnAMBR
	}

	ue.mu.Lock()

	if p.Deactivating || p.Modifying {
		ue.mu.Unlock()

		return
	}

	p.Modifying = true
	p.PendingDNConfig = qos.DnFingerprint()
	p.PendingSessAmbrDLBps = qos.SessAmbrDL.Bps()
	p.PendingSessAmbrULBps = qos.SessAmbrUL.Bps()
	p.PendingQCI = qos.QCI
	p.PendingARP = qos.ARP

	if dnsValid {
		p.Dns = dns
	}
	ue.mu.Unlock()

	plain, err := req.MarshalBinary()
	if err != nil {
		ue.mu.Lock()
		ClearPendingModifyLocked(p)
		ue.mu.Unlock()

		logger.From(ctx, logger.MmeLog).Error("failed to build Modify EPS Bearer Context Request",
			zap.String("imsi", ue.IMSI()), zap.Error(err))

		return
	}

	write := func(wire []byte) error {
		// DNS and/or Session-AMBR only: no radio change, so the NAS message is sent
		// standalone in a Downlink NAS Transport (TS 23.401 §5.4.3).
		ueConn.SendDownlinkNASTransport(ctx, wire)

		return nil
	}

	if includeQoS {
		// A QCI/ARP change reconfigures the radio bearer, so the NAS message is
		// piggybacked in an S1AP E-RAB Modify Request (TS 36.413 §8.2.2).
		write = func(wire []byte) error {
			m.sendERABModify(ctx, ueConn, p, qos, wire)

			return nil
		}
	}

	if err := ueConn.SendProtected(plain, eps.SHTIntegrityProtectedCiphered, write); err != nil {
		ue.mu.Lock()
		ClearPendingModifyLocked(p)
		ue.mu.Unlock()

		ReportProtectFailure(ctx, ueConn, "Modify EPS Bearer Context Request", err)

		return
	}

	m.ArmESMGuardAbortOnly(ue, p, "Modify EPS Bearer Context Request", plain, eps.SHTIntegrityProtectedCiphered, func() {
		ue.mu.Lock()
		ClearPendingModifyLocked(p)
		ue.mu.Unlock()
	})
}

// sendERABModify reconfigures the UE's default-bearer radio QoS with an S1AP
// E-RAB MODIFY REQUEST (TS 36.413 §8.2.2): the new E-RAB-level QoS (QCI, ARP) for
// the eNB, carrying the MODIFY EPS BEARER CONTEXT REQUEST piggybacked in the
// NAS-PDU for the UE. Completion is the NAS Modify Accept, not the E-RAB Modify
// Response, so this does not block on it.
func (m *MME) sendERABModify(ctx context.Context, ueConn *UeConn, p *PdnConnection, qos *EpsQoS, naspdu []byte) {
	req := &s1ap.ERABModifyRequest{
		ERABToBeModified: []s1ap.ERABToBeModifiedItemBearerModReq{{
			ERABID: s1ap.ERABID(p.Ebi),
			QoS: s1ap.ERABLevelQoSParameters{
				QCI: s1ap.QCI(qos.QCI),
				ARP: BearerARP(qos.ARP),
			},
			NASPDU: s1ap.NASPDU(naspdu),
		}},
	}

	if err := ueConn.SendERABModify(ctx, req); err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to send E-RAB Modify Request", zap.Error(err))
		return
	}
}

// reactivateBearer asks the UE to re-establish its PDN connection by deactivating
// the default bearer with ESM cause #39 "reactivation requested" (TS 24.301
// §6.4.4.2). The request is guarded and retransmitted until the UE answers with
// DEACTIVATE EPS BEARER CONTEXT ACCEPT.
func (m *MME) reactivateBearer(ctx context.Context, ue *UeContext, p *PdnConnection) {
	m.DeactivateBearer(ctx, ue, p, eps.ESMCauseReactivationRequested, 0, false)
}
