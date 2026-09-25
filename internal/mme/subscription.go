// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

// ErrUnknownAPN reports that the subscriber's profile has no policy bound to a
// data network with the requested APN, so the PDN connection cannot be
// authorised (TS 24.301 ESM cause #27).
var ErrUnknownAPN = models.ErrUnknownAPN

func SubscribedAPN(ctx context.Context, m *MME, imsi, requested string) (string, error) {
	sub, err := m.Bearer.GetSubscriber(ctx, imsi)
	if err != nil {
		return "", fmt.Errorf("get subscriber: %w", err)
	}

	if requested == "" {
		pol, err := m.Bearer.GetDefaultPolicyByProfile(ctx, sub.ProfileID)
		if err != nil {
			return "", fmt.Errorf("get default policy: %w", err)
		}

		dn, err := m.Bearer.GetDataNetworkByID(ctx, pol.DataNetworkID)
		if err != nil {
			return "", fmt.Errorf("get data network: %w", err)
		}

		return dn.Name, nil
	}

	policies, err := m.Bearer.ListPoliciesByProfile(ctx, sub.ProfileID)
	if err != nil {
		return "", fmt.Errorf("list policies: %w", err)
	}

	for i := range policies {
		dn, err := m.Bearer.GetDataNetworkByID(ctx, policies[i].DataNetworkID)
		if err != nil {
			return "", fmt.Errorf("get data network: %w", err)
		}

		if dn.Name == requested {
			return requested, nil
		}
	}

	return "", ErrUnknownAPN
}

func SubscribedUEAMBR(ctx context.Context, m *MME, imsi string) (models.Ambr, error) {
	sub, err := m.Bearer.GetSubscriber(ctx, imsi)
	if err != nil {
		return models.Ambr{}, fmt.Errorf("get subscriber: %w", err)
	}

	profile, err := m.Bearer.GetProfileByID(ctx, sub.ProfileID)
	if err != nil {
		return models.Ambr{}, fmt.Errorf("get profile: %w", err)
	}

	downlink, err := models.ParseBitRate(profile.UeAmbrDownlink)
	if err != nil {
		return models.Ambr{}, fmt.Errorf("profile UE-AMBR downlink: %w", err)
	}

	uplink, err := models.ParseBitRate(profile.UeAmbrUplink)
	if err != nil {
		return models.Ambr{}, fmt.Errorf("profile UE-AMBR uplink: %w", err)
	}

	return models.Ambr{Uplink: uplink, Downlink: downlink}, nil
}

// Pre-emption is fixed at shall-not-trigger / not-pre-emptable, the same pair
// the 5G path encodes: a policy carries an ARP priority column and nothing else,
// so a bearer claiming it may displace another's radio resources would be
// claiming an authorization the profile never granted.
func BearerARP(priority byte) s1ap.AllocationAndRetentionPriority {
	return s1ap.AllocationAndRetentionPriority{
		PriorityLevel:           priority,
		PreemptionCapability:    s1ap.PreemptionShallNotTrigger,
		PreemptionVulnerability: s1ap.PreemptionNotPreemptable,
	}
}
