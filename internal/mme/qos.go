// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
)

// EpsQoS is the default-bearer QoS resolved from a subscriber's profile/policy.
type EpsQoS struct {
	PolicyID string // policy DB ID, so the UPF binds the session to its network rules
	QCI      byte
	ARP      byte // priority level (1-15)
	APN      string
	// AMBRDL/UL is the profile UE-AMBR, signaled as the S1AP UE Aggregate
	// Maximum Bit Rate — the per-UE aggregate across all non-GBR bearers.
	AMBRDL models.BitRate
	AMBRUL models.BitRate
	// SessAmbrUL/DL is the policy per-APN Session-AMBR, enforced by the UPF QER
	// and signaled to the UE as the APN-AMBR (TS 24.301 §9.9.4.2, §8.3.6.7).
	// Distinct from the UE-AMBR above.
	SessAmbrUL models.BitRate
	SessAmbrDL models.BitRate
	IPv4Pool   string // data-network pools; non-empty enables that IP family
	IPv6Pool   string
	DNS        string // data-network DNS server, advertised to the UE via PCO
	MTU        uint16
	Allow4G    bool
	// Snssai is the slice the policy binds the PDN connection to, signalled to the
	// UE in the PCO S-NSSAI container (TS 23.501 §5.15.7.1).
	Snssai models.Snssai
}

// ResolveQoS maps the subscriber's profile → policy → data network to the EPS
// default-bearer QoS. With no S-NSSAI in 4G, the profile's first policy is the
// default bearer.
func ResolveQoS(ctx context.Context, m *MME, imsi string) (*EpsQoS, error) {
	sub, err := m.Bearer.GetSubscriber(ctx, imsi)
	if err != nil {
		return nil, fmt.Errorf("get subscriber: %w", err)
	}

	profile, err := m.Bearer.GetProfileByID(ctx, sub.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}

	// The 4G default bearer uses the profile's default data-network binding (the
	// default APN, TS 23.401).
	pol, err := m.Bearer.GetDefaultPolicyByProfile(ctx, sub.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("get default policy: %w", err)
	}

	return qosForPolicy(ctx, m, profile, pol)
}

// ErrUnknownAPN reports that the subscriber's profile has no policy bound to a
// data network with the requested APN, so the PDN connection cannot be
// authorised (TS 24.301 ESM cause #27).
var ErrUnknownAPN = fmt.Errorf("mme: requested APN not in subscriber profile")

// ResolveQoSByAPN resolves the EPS QoS for a UE-requested APN by finding the
// subscriber's profile policy whose data network carries that name. It returns
// ErrUnknownAPN when no policy matches, so an unauthorised PDN connectivity
// request is rejected (TS 24.301 §6.5.1.4).
func ResolveQoSByAPN(ctx context.Context, m *MME, imsi, apn string) (*EpsQoS, error) {
	sub, err := m.Bearer.GetSubscriber(ctx, imsi)
	if err != nil {
		return nil, fmt.Errorf("get subscriber: %w", err)
	}

	profile, err := m.Bearer.GetProfileByID(ctx, sub.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}

	policies, err := m.Bearer.ListPoliciesByProfile(ctx, sub.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}

	for i := range policies {
		dn, err := m.Bearer.GetDataNetworkByID(ctx, policies[i].DataNetworkID)
		if err != nil {
			return nil, fmt.Errorf("get data network: %w", err)
		}

		if dn.Name == apn {
			snssai, err := sliceSnssai(ctx, m, policies[i].SliceID)
			if err != nil {
				return nil, err
			}

			return qosForPolicyDN(profile, &policies[i], dn, snssai)
		}
	}

	return nil, ErrUnknownAPN
}

func qosForPolicy(ctx context.Context, m *MME, profile *db.Profile, pol *db.Policy) (*EpsQoS, error) {
	dn, err := m.Bearer.GetDataNetworkByID(ctx, pol.DataNetworkID)
	if err != nil {
		return nil, fmt.Errorf("get data network: %w", err)
	}

	snssai, err := sliceSnssai(ctx, m, pol.SliceID)
	if err != nil {
		return nil, err
	}

	return qosForPolicyDN(profile, pol, dn, snssai)
}

func sliceSnssai(ctx context.Context, m *MME, sliceID string) (models.Snssai, error) {
	slice, err := m.Bearer.GetNetworkSliceByID(ctx, sliceID)
	if err != nil {
		return models.Snssai{}, fmt.Errorf("get network slice: %w", err)
	}

	out := models.Snssai{Sst: slice.Sst}
	if slice.Sd != nil {
		out.Sd = *slice.Sd
	}

	return out, nil
}

// The configured rates are parsed here, at the edge of the policy layer, so
// nothing downstream handles the text form.
func qosForPolicyDN(profile *db.Profile, pol *db.Policy, dn *db.DataNetwork, snssai models.Snssai) (*EpsQoS, error) {
	ueAmbrDL, err := models.ParseBitRate(profile.UeAmbrDownlink)
	if err != nil {
		return nil, fmt.Errorf("profile UE-AMBR downlink: %w", err)
	}

	ueAmbrUL, err := models.ParseBitRate(profile.UeAmbrUplink)
	if err != nil {
		return nil, fmt.Errorf("profile UE-AMBR uplink: %w", err)
	}

	sessAmbrUL, err := models.ParseBitRate(pol.SessionAmbrUplink)
	if err != nil {
		return nil, fmt.Errorf("policy Session-AMBR uplink: %w", err)
	}

	sessAmbrDL, err := models.ParseBitRate(pol.SessionAmbrDownlink)
	if err != nil {
		return nil, fmt.Errorf("policy Session-AMBR downlink: %w", err)
	}

	return &EpsQoS{
		PolicyID:   pol.ID,
		QCI:        byte(pol.Var5qi), // 5QI↔QCI align for the standardized values
		ARP:        byte(pol.Arp),
		APN:        dn.Name,
		AMBRDL:     ueAmbrDL,
		AMBRUL:     ueAmbrUL,
		SessAmbrUL: sessAmbrUL,
		SessAmbrDL: sessAmbrDL,
		IPv4Pool:   dn.IPv4Pool,
		IPv6Pool:   dn.IPv6Pool,
		DNS:        dn.DNS,
		MTU:        uint16(dn.MTU),
		Allow4G:    profile.Allow4G,
		Snssai:     snssai,
	}, nil
}

// DnFingerprint summarises the parameters delivered to the UE at bearer setup
// (IP pools, DNS, MTU, and the slice). A change between attach and a later
// reconcile means the UE's bearer must be re-established to pick it up.
func (q *EpsQoS) DnFingerprint() string {
	return fmt.Sprintf("%s|%s|%s|%d|%d-%s", q.IPv4Pool, q.IPv6Pool, q.DNS, q.MTU, q.Snssai.Sst, q.Snssai.Sd)
}

// ResolveAttachQoS resolves the default-bearer QoS for an attaching UE. It honours
// a UE-requested APN (TS 24.301 §6.5.1.3) by selecting the policy bound to that data
// network, and falls back to the profile's default policy when no APN is requested.
func ResolveAttachQoS(ctx context.Context, m *MME, ue *UeContext) (*EpsQoS, error) {
	ctx, span := Tracer.Start(ctx, "mme/resolve_attach_qos")
	defer span.End()

	if ue.RequestedAPN != "" {
		return ResolveQoSByAPN(ctx, m, ue.IMSI(), ue.RequestedAPN)
	}

	return ResolveQoS(ctx, m, ue.IMSI())
}
