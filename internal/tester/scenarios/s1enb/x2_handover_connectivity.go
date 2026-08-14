// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const (
	x2ConnIMSI      = "001017271246680"
	x2ConnSourceTun = "s1enbx2src0"
	x2ConnTargetTun = "s1enbx2tgt0"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/x2_handover_connectivity",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBX2HandoverConnectivity,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers:         []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(x2ConnIMSI, "")},
				AssertUsageForIMSIs: []string{x2ConnIMSI},
			}
		},
	})
}

// X2 handover completes MME-side via a PATH SWITCH REQUEST (TS 36.413 §8.4.4);
// the post-switch ping from the target's tunnel proves the MME reprogrammed the
// UPF downlink to the target eNB.
func runS1ENBX2HandoverConnectivity(ctx context.Context, env scenarios.Env, _ any) error {
	if len(env.GNBs) < 2 {
		return fmt.Errorf("x2_handover_connectivity requires at least 2 eNB radios, got %d", len(env.GNBs))
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

	ue := source.NewUE(x2ConnIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	res, err := source.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach on source eNB: %w", err)
	}

	sourceTunnel, err := handoverTunnelOpts(env, res, res.DLTEID, x2ConnSourceTun)
	if err != nil {
		return err
	}

	if err := source.AddTunnel(sourceTunnel); err != nil {
		return fmt.Errorf("add source GTP tunnel: %w", err)
	}

	// Let the UPF program the downlink endpoint before pinging.
	if err := awaitHandoverTunnelReady(env, x2ConnSourceTun); err != nil {
		return err
	}

	if err := handoverProbe(ctx, env, x2ConnSourceTun); err != nil {
		return fmt.Errorf("ping before handover via source eNB: %w", err)
	}

	targetENBUEID := target.AllocateENBUEID()

	dlTEID, err := target.SendPathSwitchRequest(targetENBUEID, res.MMEUES1APID, res.ERABID, ue.S1APSecurityCapabilities())
	if err != nil {
		return fmt.Errorf("send Path Switch Request: %w", err)
	}

	if _, err := target.WaitForMessage(targetENBUEID, s1enb.Successful, s1ap.ProcPathSwitchRequest, 10*time.Second); err != nil {
		return fmt.Errorf("await Path Switch Request Acknowledge: %w", err)
	}

	source.CloseTunnel(res.DLTEID)

	// The UPF now forwards downlink to the target's DL TEID; the uplink endpoint
	// (UPF address + UL TEID) is unchanged by the path switch.
	targetTunnel, err := handoverTunnelOpts(env, res, dlTEID, x2ConnTargetTun)
	if err != nil {
		return err
	}

	if err := target.AddTunnel(targetTunnel); err != nil {
		return fmt.Errorf("add target GTP tunnel: %w", err)
	}

	defer target.CloseTunnel(dlTEID)

	// Let the MME's ModifyEPSSession reprogram the UPF downlink before pinging.
	if err := awaitHandoverTunnelReady(env, x2ConnTargetTun); err != nil {
		return err
	}

	if err := handoverProbe(ctx, env, x2ConnTargetTun); err != nil {
		return fmt.Errorf("ping after X2 path switch via target eNB (UPF downlink not switched?): %w", err)
	}

	return nil
}
