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
// subscriber's default-bearer QoS. *db.Database satisfies it.
type bearerStore interface {
	GetSubscriber(ctx context.Context, imsi string) (*db.Subscriber, error)
	GetProfileByID(ctx context.Context, id string) (*db.Profile, error)
	GetDefaultPolicyByProfile(ctx context.Context, profileID string) (*db.Policy, error)
	ListPoliciesByProfile(ctx context.Context, profileID string) ([]db.Policy, error)
	GetDataNetworkByID(ctx context.Context, id string) (*db.DataNetwork, error)
	GetOperator(ctx context.Context) (*db.Operator, error)
	// NodeID is the cluster node identity, used to make each HA node's MME Code
	// (and hence its GUMMEI) distinct.
	NodeID() int
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

// CommitBearerModification commits a PDN connection's pending in-place
// modification, reporting false (a no-op) if no modification was in flight
// (TS 24.301 §6.4.2.3).
func (ue *UeContext) CommitBearerModification(p *PdnConnection) bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if !p.Modifying {
		return false
	}

	p.DnConfig = p.PendingDNConfig
	p.SessAmbrDLBps = p.PendingSessAmbrDLBps
	p.SessAmbrULBps = p.PendingSessAmbrULBps
	p.Qci = p.PendingQCI
	p.Arp = p.PendingARP
	ClearPendingModifyLocked(p)

	return true
}

// ClearPendingModify clears a PDN connection's in-flight modification
// bookkeeping (TS 24.301 §6.4.2.4).
func (ue *UeContext) ClearPendingModify(p *PdnConnection) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ClearPendingModifyLocked(p)
}

// BearerReleaseOnly reports whether deactivating p releases only that PDN
// connection (an additional PDN, or a disconnect) without detaching the UE
// (TS 24.301 §6.4.4.2/§6.5.2).
func (ue *UeContext) BearerReleaseOnly(p *PdnConnection) bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return p.Ebi != ue.DefaultEBI || p.Disconnecting
}

func (ue *UeContext) ClearDeactivating(p *PdnConnection) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	p.Deactivating = false
}
