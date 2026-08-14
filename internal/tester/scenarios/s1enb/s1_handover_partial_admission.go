// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const (
	s1PartialIMSI = "001017271246685"
	s1PartialDNN  = "handover-second-pdn"
	s1PartialPool = "10.48.0.0/16"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/s1_handover_partial_admission",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1HandoverPartialAdmission,
		Fixture:   fixtureS1HandoverPartialAdmission,
	})
}

func fixtureS1HandoverPartialAdmission(_ scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		DataNetworks: []scenarios.DataNetworkSpec{
			{Name: s1PartialDNN, IPv4Pool: s1PartialPool, DNS: scenarios.DefaultDNS, MTU: scenarios.DefaultMTU},
		},
		Policies: []scenarios.PolicySpec{
			{
				Name:                "s1-handover-second-pdn",
				ProfileName:         scenarios.DefaultProfileName,
				SliceName:           scenarios.DefaultSliceName,
				DataNetworkName:     s1PartialDNN,
				SessionAmbrUplink:   "30 Mbps",
				SessionAmbrDownlink: "60 Mbps",
				Var5qi:              9,
				Arp:                 15,
			},
		},
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(s1PartialIMSI, "")},
	}
}

func runS1HandoverPartialAdmission(_ context.Context, env scenarios.Env, _ any) error {
	pair, closePair, err := startS1HandoverPair(env, s1PartialIMSI)
	if err != nil {
		return err
	}

	defer closePair()

	second, err := pair.Source.OpenPDN(pair.UE, pair.Attached.MMEUES1APID, pair.Attached.ENBUES1APID,
		s1PartialDNN, uint8(eps.PDNTypeIPv4), s1FailureTimeout)
	if err != nil {
		return fmt.Errorf("open the second PDN connection: %w", err)
	}

	if second.ERABID == pair.Attached.ERABID {
		return fmt.Errorf("the second PDN reused E-RAB %d", second.ERABID)
	}

	req, err := pair.requireHandover()
	if err != nil {
		return err
	}

	if len(req.ERABToBeSetup) != 2 {
		return fmt.Errorf("the Handover Request carried %d E-RABs, want both bearers", len(req.ERABToBeSetup))
	}

	targetENBUEID := pair.Target.AllocateENBUEID()
	refusal := s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkRadioResourcesNotAvailable}

	if _, err := pair.Target.SendHandoverRequestAcknowledgePartial(targetENBUEID, int64(req.MMEUES1APID),
		[]s1ap.ERABID{pair.Attached.ERABID}, []s1ap.ERABID{second.ERABID}, refusal); err != nil {
		return fmt.Errorf("admit one E-RAB at the target eNB: %w", err)
	}

	cmd, err := pair.Source.WaitForHandoverCommand(pair.Attached.ENBUES1APID, s1FailureTimeout)
	if err != nil {
		return fmt.Errorf("await Handover Command: %w", err)
	}

	if len(cmd.ERABToRelease) != 1 {
		return fmt.Errorf("the Handover Command told the source to release %d E-RABs, want the one the target refused", len(cmd.ERABToRelease))
	}

	if cmd.ERABToRelease[0].ERABID != second.ERABID {
		return fmt.Errorf("the Handover Command released E-RAB %d, want the refused %d", cmd.ERABToRelease[0].ERABID, second.ERABID)
	}

	if err := pair.Source.SendENBStatusTransfer(pair.Attached.MMEUES1APID, pair.Attached.ENBUES1APID); err != nil {
		return fmt.Errorf("send eNB Status Transfer: %w", err)
	}

	if _, err := pair.Target.WaitForMMEStatusTransfer(targetENBUEID, s1FailureTimeout); err != nil {
		return fmt.Errorf("await MME Status Transfer: %w", err)
	}

	if err := pair.Target.SendHandoverNotify(targetENBUEID, int64(req.MMEUES1APID)); err != nil {
		return fmt.Errorf("send Handover Notify: %w", err)
	}

	if _, err := pair.Source.WaitForUEContextReleaseCommand(pair.Attached.ENBUES1APID, s1FailureTimeout); err != nil {
		return errors.New("the source eNB was not released after the UE moved")
	}

	return nil
}
