// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"slices"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// ActiveEBIs returns the EPS bearer identities of the UE's established PDN
// connections, sorted.
func (ue *UeContext) ActiveEBIs() []uint8 {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	out := make([]uint8, 0, len(ue.Pdns))
	for ebi := range ue.Pdns {
		out = append(out, ebi)
	}

	slices.Sort(out)

	return out
}

// DefaultERABID is the EPS bearer identity of the default bearer (TS 24.301).
const DefaultERABID byte = 5

// bearerStore is the subscription-data surface the MME needs to resolve a
// subscriber's APNs and UE-AMBR. *db.Database satisfies it.
type bearerStore interface {
	GetSubscriber(ctx context.Context, imsi string) (*db.Subscriber, error)
	GetProfileByID(ctx context.Context, id string) (*db.Profile, error)
	GetDefaultPolicyByProfile(ctx context.Context, profileID string) (*db.Policy, error)
	ListPoliciesByProfile(ctx context.Context, profileID string) ([]db.Policy, error)
	GetDataNetworkByID(ctx context.Context, id string) (*db.DataNetwork, error)
	GetOperator(ctx context.Context) (*db.Operator, error)
	// NodeID is the cluster node identity, used to make each HA node's MME Code
	// (and hence its GUMMEI) distinct.
	AMFPointer() int
}

// S1apSecurityCapabilities maps a UE's EPS NAS algorithm support to the S1AP UE
// Security Capabilities the eNB selects AS algorithms from. The S1AP BIT STRING
// omits the EEA0/EIA0 (mandatory null-algorithm) bit, so the UE network
// capability octet is shifted left and placed in the high byte (TS 36.413
// §9.2.1.40, TS 33.401).
func S1apSecurityCapabilities(uecap eps.UENetworkCapability) s1ap.UESecurityCapabilities {
	return s1ap.UESecurityCapabilities{
		EncryptionAlgorithms:          uint16(uecap.EEA<<1) << 8,
		IntegrityProtectionAlgorithms: uint16(uecap.EIA<<1) << 8,
	}
}

// PDNCount returns the number of the UE's PDN connections.
func (ue *UeContext) PDNCount() int {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return len(ue.Pdns)
}

// ConcludeBearerModification ends a PDN connection's in-place modification and
// reports its outcome to the SMF, committing the new values when the UE accepted
// it (TS 24.301 §6.4.3.3) and dropping them otherwise (§6.4.3.4). A modification
// that reconfigures the radio bearer also waits for the eNB's E-RAB Modify
// Response (TS 23.401 §5.4.2.1 step 10). It reports false (a no-op) if no
// modification was in flight.
func (m *MME) ConcludeBearerModification(ctx context.Context, ue *UeContext, p *PdnConnection, accepted bool) bool {
	ue.mu.Lock()

	if p.Modifying == nil {
		ue.mu.Unlock()

		return false
	}

	if accepted && p.modifyAwaitingRadio {
		p.modifyAcceptedByUE = true
		ue.mu.Unlock()

		return true
	}

	ref := p.SessionRef
	ambr, ambrChanged := m.finishModificationLocked(ue, p, accepted)
	ue.mu.Unlock()

	m.Session.CommitEPSBearerModification(ctx, ref, accepted)

	if ambrChanged {
		m.signalUEAMBR(ctx, ue, ambr)
	}

	return true
}

func (m *MME) RadioBearerModified(ctx context.Context, ue *UeContext, ebi uint8, modified bool) {
	ue.mu.Lock()

	p := ue.Pdns[ebi]
	if p == nil || p.Modifying == nil || !p.modifyAwaitingRadio {
		ue.mu.Unlock()

		return
	}

	p.modifyAwaitingRadio = false

	if modified && !p.modifyAcceptedByUE {
		ue.mu.Unlock()

		return
	}

	p.guard.Stop()

	ref := p.SessionRef
	ambr, ambrChanged := m.finishModificationLocked(ue, p, modified)
	ue.mu.Unlock()

	m.Session.CommitEPSBearerModification(ctx, ref, modified)

	if ambrChanged {
		m.signalUEAMBR(ctx, ue, ambr)
	}
}

func (m *MME) finishModificationLocked(ue *UeContext, p *PdnConnection, accepted bool) (models.Ambr, bool) {
	mod := p.Modifying
	p.clearModificationLocked()

	if !accepted {
		return models.Ambr{}, false
	}

	before := ue.ranUEAMBRLocked()

	if mod.QoS != nil {
		p.Qci = mod.QoS.QCI
		p.Arp = mod.QoS.ARP
	}

	if mod.APNAMBR != nil {
		p.SessAmbrDLBps = mod.APNAMBR.Downlink.Bps()
		p.SessAmbrULBps = mod.APNAMBR.Uplink.Bps()
	}

	if mod.DNS.IsValid() {
		p.Dns = mod.DNS
	}

	after := ue.ranUEAMBRLocked()

	return after, after != before
}

func (m *MME) signalUEAMBR(ctx context.Context, ue *UeContext, ambr models.Ambr) {
	ueConn, ready := m.ReconcileReady(ue)
	if !ready || ueConn.ICS() != ICSCompleted {
		return
	}

	if err := ueConn.SendUEContextModification(ctx, ambr); err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to signal the UE-AMBR", zap.Error(err))
	}
}

func (m *MME) RefreshUEAMBRs(ctx context.Context) {
	for _, ue := range m.ConnectedUEs() {
		subscribed, err := SubscribedUEAMBR(ctx, m, ue.IMSI())
		if err != nil {
			logger.From(ctx, logger.MmeLog).Warn("failed to read the subscribed UE-AMBR", logger.SUPI(ue.Supi().String()), zap.Error(err))
			continue
		}

		before := ue.RANUEAMBR()

		if after := ue.SetSubscribedUEAMBR(subscribed); after != before {
			m.signalUEAMBR(ctx, ue, after)
		}
	}
}

func (p *PdnConnection) clearModificationLocked() {
	p.Modifying = nil
	p.modifyAwaitingRadio = false
	p.modifyAcceptedByUE = false
}

func (ue *UeContext) BearerReleaseOnly(p *PdnConnection) bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return len(ue.Pdns) > 1 || p.Disconnecting
}

func (ue *UeContext) BearerDeactivating(p *PdnConnection) bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return p.Deactivating
}
