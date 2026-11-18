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
	"github.com/ellanetworks/core/internal/tester/scenarios/common"
	"github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const (
	partitionSettle       = 10 * time.Second
	partitionDeregTimeout = 60 * time.Second
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ha/partition_release_5g",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runPartitionRelease5G(ctx, env)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
			}
		},
	})
}

func runPartitionRelease5G(ctx context.Context, env scenarios.Env) error {
	if len(env.CoreN2Addresses) == 0 {
		return fmt.Errorf("ha/partition_release_5g requires at least one core address")
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
		Name:            "Ella-Core-Tester-Partition",
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

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	u, err := common.RegisterAndPingUE(ctx, &common.RegisterAndPingOpts{
		GNB:              gNodeB,
		RANUENGAPID:      ranUENGAPID,
		PDUSessionID:     scenarios.DefaultPDUSessionID,
		IMSI:             scenarios.DefaultIMSI,
		TunInterfaceName: "ellapart0",
	})
	if err != nil {
		return fmt.Errorf("phase1: %w", err)
	}

	logger.Logger.Info("phase1: UE registered", zap.String("peer", gNodeB.ActivePeerAddress()))

	fmt.Println(failoverMarker)

	_ = os.Stdout.Sync()

	select {
	case <-time.After(partitionSettle):
	case <-ctx.Done():
		return ctx.Err()
	}

	if err := gNodeB.Deregister(u, ranUENGAPID, partitionDeregTimeout); err != nil {
		return fmt.Errorf("phase2: deregister on the partitioned core: %w", err)
	}

	logger.Logger.Info("phase2: UE deregistered on the partitioned core")

	return nil
}
