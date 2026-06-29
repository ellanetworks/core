// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package ha holds core-tester scenarios that exercise multi-core (HA)
// behaviour from the RAN side.
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
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

// failoverMarker is printed to stdout after the phase-1 flow completes,
// signalling the orchestrator (integration test) that the primary core
// can now be killed. Must match the value the test scans for.
const failoverMarker = "PHASE1_DONE"

// failoverTimeout caps the wait for the primary's SCTP association to drop
// and the gNB to pick a new active peer. Kernel-level SCTP drop detection
// on a killed container is typically sub-second; 60s is a generous bound.
const failoverTimeout = 60 * time.Second

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "ha/failover_connectivity_5g",
		BindFlags: func(fs *pflag.FlagSet) any {
			return struct{}{}
		},
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runFailoverConnectivity(ctx, env)
		},
		Fixture: func(env scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
			}
		},
	})
}

func runFailoverConnectivity(ctx context.Context, env scenarios.Env) error {
	if len(env.CoreN2Addresses) < 2 {
		return fmt.Errorf("ha/failover_connectivity_5g requires at least 2 core addresses; got %d", len(env.CoreN2Addresses))
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
		Name:            "Ella-Core-Tester-HA",
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    g.N2Address,
		GnbN3Address:    g.N3Address,
	})
	if err != nil {
		return fmt.Errorf("start gNB: %w", err)
	}

	defer gNodeB.Close()

	if _, err := gNodeB.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentNGSetupResponse,
		2*time.Second,
	); err != nil {
		return fmt.Errorf("phase1: NG Setup Response: %w", err)
	}

	primaryPeer := gNodeB.ActivePeerAddress()
	logger.Logger.Info("phase1: active peer set", zap.String("peer", primaryPeer))

	if err := registerAndPing(
		ctx,
		gNodeB,
		int64(scenarios.DefaultRANUENGAPID),
		"ellaha0",
	); err != nil {
		return fmt.Errorf("phase1: %w", err)
	}

	logger.Logger.Info("phase1: connectivity verified", zap.String("peer", primaryPeer))

	fmt.Println(failoverMarker)

	_ = os.Stdout.Sync()

	waitCtx, cancel := context.WithTimeout(ctx, failoverTimeout)
	newPeer, err := gNodeB.WaitForActivePeerChange(waitCtx)

	cancel()

	if err != nil {
		return fmt.Errorf("wait for peer change: %w", err)
	}

	if newPeer == "" {
		return fmt.Errorf("all peers exhausted after primary failure")
	}

	if newPeer == primaryPeer {
		return fmt.Errorf("active peer unchanged after signalled failover (%s)", newPeer)
	}

	logger.Logger.Info(
		"phase2: active peer switched",
		zap.String("from", primaryPeer),
		zap.String("to", newPeer),
	)

	// Consume the new peer's NG Setup Response so its AMF is handshaken and
	// ready before UE signalling begins.
	if _, err := gNodeB.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentNGSetupResponse,
		10*time.Second,
	); err != nil {
		return fmt.Errorf("phase2: wait for NG Setup Response on new peer: %w", err)
	}

	// Registration bumps the subscriber's sequenceNumber, a Raft-replicated
	// write. Killing the leader starts a heartbeat-timeout → election cycle
	// of a few seconds, during which forwarded writes return
	// ErrLeadershipLost and NAS sends Registration Reject. Retry with backoff
	// until the new leader settles.
	const (
		phase2Deadline = 30 * time.Second
		phase2Backoff  = 2 * time.Second
	)

	phase2Start := time.Now()

	var phase2Err error

	for attempt := 0; time.Since(phase2Start) < phase2Deadline; attempt++ {
		phase2Err = registerAndPing(
			ctx,
			gNodeB,
			int64(scenarios.DefaultRANUENGAPID)+int64(1+attempt),
			fmt.Sprintf("ellaha%d", 1+attempt),
		)
		if phase2Err == nil {
			break
		}

		logger.Logger.Warn(
			"phase2 attempt failed; waiting for leadership to settle",
			zap.Int("attempt", attempt),
			zap.String("peer", gNodeB.ActivePeerAddress()),
			zap.Error(phase2Err),
		)

		select {
		case <-time.After(phase2Backoff):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if phase2Err != nil {
		return fmt.Errorf("phase2 after %s: %w", phase2Deadline, phase2Err)
	}

	logger.Logger.Info("phase2: connectivity verified on new peer", zap.String("peer", newPeer))

	return nil
}

// registerAndPing wraps common.RegisterAndPing with the default subscriber.
// A fresh RAN-UE-NGAP-ID and tunnel name per call keep retried attempts from
// colliding with stale gNB-local context.
func registerAndPing(ctx context.Context, gNodeB *gnb.GnodeB, ranUENGAPID int64, tunInterfaceName string) error {
	return common.RegisterAndPing(ctx, &common.RegisterAndPingOpts{
		GNB:              gNodeB,
		RANUENGAPID:      ranUENGAPID,
		PDUSessionID:     scenarios.DefaultPDUSessionID,
		IMSI:             scenarios.DefaultIMSI,
		TunInterfaceName: tunInterfaceName,
	})
}
