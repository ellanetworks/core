// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

const (
	s1hoIMSI      = "001017271246681"
	s1hoSourceTun = "s1enbhosrc0"
	s1hoTargetTun = "s1enbhotgt0"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/s1_handover",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBHandover,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(s1hoIMSI, "")},
			}
		},
	})
}

// runS1ENBHandover drives a full inter-eNB S1 handover (TS 36.413 §8.4, TS 23.401
// §5.5.1.2.2): attach on the source eNB, ping, then HANDOVER REQUIRED → REQUEST →
// ACKNOWLEDGE → COMMAND → eNB STATUS TRANSFER → MME STATUS TRANSFER → NOTIFY, and
// ping again from the target's GTP tunnel. The after-ping proves the MME switched
// the UPF downlink to the target eNB only at notify. Requires two eNB radios with
// distinct N3 addresses.
func runS1ENBHandover(ctx context.Context, env scenarios.Env, _ any) error {
	if len(env.GNBs) < 2 {
		return fmt.Errorf("s1_handover requires at least 2 eNB radios, got %d", len(env.GNBs))
	}

	s1mme, err := s1mmeAddress(env.FirstCore())
	if err != nil {
		return err
	}

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	enbID, err := strconv.ParseUint(scenarios.DefaultGNBID, 16, 32)
	if err != nil {
		return fmt.Errorf("parse eNB ID %q: %w", scenarios.DefaultGNBID, err)
	}

	sourceSpec, targetSpec := env.GNBs[0], env.GNBs[1]

	source, err := s1enb.Start(&s1enb.StartOpts{
		ENBID: uint32(enbID), MCC: scenarios.DefaultMCC, MNC: scenarios.DefaultMNC, TAC: scenarios.DefaultTAC,
		Name: "Source-S1eNB", CoreS1MMEAddress: s1mme,
		ENBAddress: sourceSpec.N2Address, ENBN3Address: sourceSpec.N3Address, EnableDatapath: true,
	})
	if err != nil {
		return fmt.Errorf("start source eNB: %w", err)
	}

	defer func() { _ = source.Close() }()

	target, err := s1enb.Start(&s1enb.StartOpts{
		ENBID: uint32(enbID) + 1, MCC: scenarios.DefaultMCC, MNC: scenarios.DefaultMNC, TAC: scenarios.DefaultTAC,
		Name: "Target-S1eNB", CoreS1MMEAddress: s1mme,
		ENBAddress: targetSpec.N2Address, ENBN3Address: targetSpec.N3Address, EnableDatapath: true,
	})
	if err != nil {
		return fmt.Errorf("start target eNB: %w", err)
	}

	defer func() { _ = target.Close() }()

	ue := source.NewUE(s1hoIMSI, k, opc)

	res, err := source.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach on source eNB: %w", err)
	}

	if res.UEIPv4 == "" {
		return fmt.Errorf("attach assigned no IPv4 address")
	}

	if err := source.AddTunnel(&s1enb.TunnelOpts{
		UEIPv4: res.UEIPv4 + "/16", UpfAddress: res.UpfAddress, ULTEID: res.ULTEID, DLTEID: res.DLTEID,
		TunInterfaceName: s1hoSourceTun,
	}); err != nil {
		return fmt.Errorf("add source GTP tunnel: %w", err)
	}

	time.Sleep(500 * time.Millisecond)

	if err := probe.Run(ctx, probe.ICMP, s1hoSourceTun, scenarios.DefaultPingDestination, scenarios.DefaultProbePort, false); err != nil {
		return fmt.Errorf("ping before handover via source eNB: %w", err)
	}

	if err := source.SendHandoverRequired(res.ENBUES1APID, res.MMEUES1APID, target.GlobalENBID()); err != nil {
		return fmt.Errorf("send Handover Required: %w", err)
	}

	hoReq, err := target.WaitForHandoverRequest(10 * time.Second)
	if err != nil {
		return fmt.Errorf("await Handover Request: %w", err)
	}

	if len(hoReq.ERABToBeSetup) != 1 {
		return fmt.Errorf("handover request carried %d E-RABs, want 1", len(hoReq.ERABToBeSetup))
	}

	targetENBUEID := target.AllocateENBUEID()

	dlTEID, err := target.SendHandoverRequestAcknowledge(targetENBUEID, res.MMEUES1APID, res.ERABID)
	if err != nil {
		return fmt.Errorf("send Handover Request Acknowledge: %w", err)
	}

	if _, err := source.WaitForHandoverCommand(res.ENBUES1APID, 10*time.Second); err != nil {
		return fmt.Errorf("await Handover Command: %w", err)
	}

	if err := source.SendENBStatusTransfer(res.MMEUES1APID, res.ENBUES1APID); err != nil {
		return fmt.Errorf("send eNB Status Transfer: %w", err)
	}

	if _, err := target.WaitForMMEStatusTransfer(targetENBUEID, 10*time.Second); err != nil {
		return fmt.Errorf("await MME Status Transfer: %w", err)
	}

	// The downlink must still run via the source before notify.
	if err := probe.Run(ctx, probe.ICMP, s1hoSourceTun, scenarios.DefaultPingDestination, scenarios.DefaultProbePort, false); err != nil {
		return fmt.Errorf("ping during handover preparation via source eNB (UPF switched too early?): %w", err)
	}

	if err := target.SendHandoverNotify(targetENBUEID, res.MMEUES1APID); err != nil {
		return fmt.Errorf("send Handover Notify: %w", err)
	}

	if _, err := source.WaitForUEContextReleaseCommand(res.ENBUES1APID, 10*time.Second); err != nil {
		return fmt.Errorf("await source UE Context Release Command: %w", err)
	}

	if err := source.SendUEContextReleaseComplete(res.MMEUES1APID, res.ENBUES1APID); err != nil {
		return fmt.Errorf("send source UE Context Release Complete: %w", err)
	}

	source.CloseTunnel(res.DLTEID)

	if err := target.AddTunnel(&s1enb.TunnelOpts{
		UEIPv4: res.UEIPv4 + "/16", UpfAddress: res.UpfAddress, ULTEID: res.ULTEID, DLTEID: dlTEID,
		TunInterfaceName: s1hoTargetTun,
	}); err != nil {
		return fmt.Errorf("add target GTP tunnel: %w", err)
	}

	defer target.CloseTunnel(dlTEID)

	time.Sleep(500 * time.Millisecond)

	if err := probe.Run(ctx, probe.ICMP, s1hoTargetTun, scenarios.DefaultPingDestination, scenarios.DefaultProbePort, false); err != nil {
		return fmt.Errorf("ping after S1 handover via target eNB (UPF downlink not switched?): %w", err)
	}

	return nil
}
