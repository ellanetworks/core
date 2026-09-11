// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const drainIMSI = "001017271246607"

const drainSignalTimeout = 60 * time.Second

var causeLoadBalancingTAURequired = s1ap.Cause{
	Group: s1ap.CauseGroupRadioNetwork,
	Value: s1ap.CauseRadioNetworkLoadBalancingTAURequired,
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ha/drain_4g",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(_ context.Context, env scenarios.Env, _ any) error {
			return runS1ENBDrain(env)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(drainIMSI, "")},
			}
		},
	})
}

func runS1ENBDrain(env scenarios.Env) error {
	if len(env.CoreN2Addresses) == 0 {
		return fmt.Errorf("ha/drain_4g requires at least one core address")
	}

	primary, err := s1mmeAddress(env.CoreN2Addresses[0])
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

	g := env.FirstGNB()

	e, err := s1enb.Start(&s1enb.StartOpts{
		ENBID: uint32(enbID), MCC: scenarios.DefaultMCC, MNC: scenarios.DefaultMNC, TAC: scenarios.DefaultTAC,
		Name: "Ella-Core-Tester-S1eNB-Drain", CoreS1MMEAddresses: []string{primary},
		ENBAddress: g.N2Address, ENBN3Address: g.N3Address, EnableDatapath: true,
	})
	if err != nil {
		return fmt.Errorf("start eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(drainIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	res, err := e.Attach(ue, attachTimeout)
	if err != nil {
		return fmt.Errorf("phase1: attach: %w", err)
	}

	logger.Logger.Info("phase1: UE attached", zap.String("peer", primary))

	fmt.Println(failoverMarker)

	_ = os.Stdout.Sync()

	update, err := e.WaitForMMEConfigurationUpdate(drainSignalTimeout)
	if err != nil {
		return fmt.Errorf("phase2: await MME Configuration Update: %w", err)
	}

	if update.RelativeMMECapacity == nil {
		return fmt.Errorf("phase2: MME Configuration Update carried no Relative MME Capacity")
	}

	if *update.RelativeMMECapacity != 0 {
		return fmt.Errorf("phase2: Relative MME Capacity = %d, want 0 so the eNB stops selecting this MME",
			*update.RelativeMMECapacity)
	}

	logger.Logger.Info("phase2: MME advertised a zero weight factor")

	if err := e.SendMMEConfigurationUpdateAcknowledge(); err != nil {
		return fmt.Errorf("phase2: acknowledge MME Configuration Update: %w", err)
	}

	cmd, err := e.WaitForUEContextReleaseCommand(res.ENBUES1APID, drainSignalTimeout)
	if err != nil {
		return fmt.Errorf("phase2: await UE Context Release Command: %w", err)
	}

	if cmd.Cause == nil {
		return fmt.Errorf("phase2: UE Context Release Command carried no cause, want %+v", causeLoadBalancingTAURequired)
	}

	if *cmd.Cause != causeLoadBalancingTAURequired {
		return fmt.Errorf("phase2: release cause = %+v, want load-balancing-tau-required %+v; the UE would keep its GUMMEI and come back",
			*cmd.Cause, causeLoadBalancingTAURequired)
	}

	logger.Logger.Info("phase2: UE released for load re-balancing")

	return e.SendUEContextReleaseComplete(int64(cmd.UES1APIDs.MMEUES1APID), int64(cmd.UES1APIDs.ENBUES1APID))
}
