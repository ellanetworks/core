// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"slices"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
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
// it (TS 24.301 §6.4.2.3) and dropping them otherwise (§6.4.2.4). It reports
// false (a no-op) if no modification was in flight.
func (m *MME) ConcludeBearerModification(ctx context.Context, ue *UeContext, p *PdnConnection, accepted bool) bool {
	ue.mu.Lock()
	mod := p.Modifying
	p.Modifying = nil

	if mod != nil && accepted {
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
	}

	ref := p.SessionRef
	ue.mu.Unlock()

	if mod == nil {
		return false
	}

	m.Session.CommitEPSBearerModification(ctx, ref, accepted)

	return true
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
