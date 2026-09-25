// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

type policyChange struct {
	SessionAmbrUplink   string
	SessionAmbrDownlink string
	Var5qi              int32
	Arp                 int32
	PreemptCap          models.PreemptionCapability
	PreemptVuln         models.PreemptionVulnerability
	DNS                 string
	MTU                 uint16
	IPv4Pool            string
	IPv6Pool            string
}

func currentPolicy(s *smf.SMF, ref string) *smf.Policy {
	sc := s.GetSession(ref)
	if sc == nil {
		return &smf.Policy{}
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.PolicyData == nil {
		return &smf.Policy{}
	}

	policy := *sc.PolicyData

	return &policy
}

func reconcileWithPolicy(ctx context.Context, s *smf.SMF, pcf *fakePCF, ref string, change *policyChange) error {
	policy := currentPolicy(s, ref)

	if change != nil {
		if change.SessionAmbrUplink != "" {
			policy.Ambr.Uplink = models.MustParseBitRate(change.SessionAmbrUplink)
		}

		if change.SessionAmbrDownlink != "" {
			policy.Ambr.Downlink = models.MustParseBitRate(change.SessionAmbrDownlink)
		}

		policy.QosData.Var5qi = change.Var5qi
		policy.QosData.Arp = &models.Arp{PriorityLevel: change.Arp, PreemptCap: change.PreemptCap, PreemptVuln: change.PreemptVuln}

		if change.DNS != "" {
			policy.DNS = net.ParseIP(change.DNS)
		}

		if change.MTU != 0 {
			policy.MTU = change.MTU
		}

		if change.IPv4Pool != "" {
			policy.IPv4Pool = change.IPv4Pool
		}

		if change.IPv6Pool != "" {
			policy.IPv6Pool = change.IPv6Pool
		}
	}

	pcf.mu.Lock()
	pcf.policy = policy
	pcf.err = nil
	pcf.mu.Unlock()

	return s.ReconcileSession(ctx, ref)
}

func reconcileSliceMismatch(ctx context.Context, s *smf.SMF, pcf *fakePCF, ref string) error {
	pcf.mu.Lock()
	pcf.err = smf.ErrNoPolicyMatch
	pcf.mu.Unlock()

	return s.ReconcileSession(ctx, ref)
}
