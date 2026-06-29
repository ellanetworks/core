// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

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

// s1apSecurityCapabilities maps a UE's EPS NAS algorithm support to the S1AP UE
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
