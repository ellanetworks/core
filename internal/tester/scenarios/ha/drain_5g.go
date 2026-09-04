// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ha

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const drainSignalTimeout = 60 * time.Second

const noReleaseWindow = 10 * time.Second

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ha/drain_5g",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runDrain5G(ctx, env)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
			}
		},
	})
}

func runDrain5G(ctx context.Context, env scenarios.Env) error {
	if len(env.CoreN2Addresses) == 0 {
		return fmt.Errorf("ha/drain_5g requires at least one core address")
	}

	g := env.FirstGNB()

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:           scenarios.DefaultGNBID,
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             scenarios.DefaultDNN,
		TAC:             scenarios.DefaultTAC,
		Name:            "Ella-Core-Tester-Drain",
		CoreN2Addresses: env.CoreN2Addresses[:1],
		GnbN2Address:    g.N2Address,
		GnbN3Address:    g.N3Address,
	})
	if err != nil {
		return fmt.Errorf("start gNB: %w", err)
	}

	defer gNodeB.Close()

	if _, err := gNodeB.WaitForMessage(gnb.Successful, ngap.ProcNGSetup, 5*time.Second); err != nil {
		return fmt.Errorf("phase1: NG Setup Response: %w", err)
	}

	if err := registerAndPing(ctx, gNodeB, int64(scenarios.DefaultRANUENGAPID), "elladrain0"); err != nil {
		return fmt.Errorf("phase1: %w", err)
	}

	logger.Logger.Info("phase1: UE registered", zap.String("peer", gNodeB.ActivePeerAddress()))

	fmt.Println(failoverMarker)

	_ = os.Stdout.Sync()

	update, err := gNodeB.WaitForAMFConfigurationUpdate(drainSignalTimeout)
	if err != nil {
		return fmt.Errorf("phase2: await AMF Configuration Update: %w", err)
	}

	if update.RelativeAMFCapacity == nil {
		return fmt.Errorf("phase2: AMF Configuration Update carried no Relative AMF Capacity")
	}

	if *update.RelativeAMFCapacity != 0 {
		return fmt.Errorf("phase2: Relative AMF Capacity = %d, want 0 so the gNB stops selecting this AMF",
			*update.RelativeAMFCapacity)
	}

	logger.Logger.Info("phase2: AMF advertised a zero weight factor")

	if err := gNodeB.SendAMFConfigurationUpdateAcknowledge(); err != nil {
		return fmt.Errorf("phase2: acknowledge AMF Configuration Update: %w", err)
	}

	ind, err := gNodeB.WaitForAMFStatusIndication(drainSignalTimeout)
	if err != nil {
		return fmt.Errorf("phase2: await AMF Status Indication: %w", err)
	}

	if len(ind.UnavailableGUAMIList) == 0 {
		return fmt.Errorf("phase2: AMF Status Indication named no unavailable GUAMI, so the gNB reselects away from no one")
	}

	logger.Logger.Info("phase2: AMF marked its GUAMI unavailable",
		zap.Int("guamis", len(ind.UnavailableGUAMIList)))

	if _, err := gNodeB.WaitForMessage(gnb.Initiating, ngap.ProcUEContextRelease, noReleaseWindow); err == nil {
		return fmt.Errorf("phase2: the AMF released the UE on drain; TS 23.501 §5.21.2.2 leaves that migration to the 5G-AN")
	}

	logger.Logger.Info("phase2: no UE Context Release Command, as the spec requires")

	resume, err := gNodeB.WaitForAMFConfigurationUpdate(drainSignalTimeout)
	if err != nil {
		return fmt.Errorf("phase3: await AMF Configuration Update on resume: %w", err)
	}

	if resume.RelativeAMFCapacity == nil || *resume.RelativeAMFCapacity == 0 {
		return fmt.Errorf("phase3: resume carried Relative AMF Capacity %v, want a non-zero weight factor",
			resume.RelativeAMFCapacity)
	}

	if len(resume.ServedGUAMIList) == 0 {
		return fmt.Errorf("phase3: resume carried no Served GUAMI List; TS 23.502 §4.2.7.2.3 leaves the GUAMI unavailable until an AMF updates it as available")
	}

	logger.Logger.Info("phase3: AMF re-advertised its GUAMI and a non-zero weight factor",
		zap.Uint8("relative-capacity", *resume.RelativeAMFCapacity),
		zap.Int("guamis", len(resume.ServedGUAMIList)))

	return gNodeB.SendAMFConfigurationUpdateAcknowledge()
}
