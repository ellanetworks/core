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

// failoverMarker is printed to stdout after phase 1 to signal the orchestrator it
// can kill the primary core. Must match the token the test scans for.
const failoverMarker = "PHASE1_DONE"

// failoverTimeout caps the wait for the primary's SCTP association to drop and
// the eNB to pick a new active MME. Kernel-level SCTP drop detection on a
// killed container is typically sub-second; 60s is a generous bound.
const failoverTimeout = 60 * time.Second

const failoverIMSI = "001017271246606"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ha/failover_connectivity_4g",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runS1ENBFailoverConnectivity(ctx, env)
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(failoverIMSI, "")},
			}
		},
	})
}

// runS1ENBFailoverConnectivity is a two-phase HA scenario:
//
//  1. Attach a UE on the eNB's primary MME, establish a bearer, verify
//     connectivity, then emit the phase-1 marker on stdout so the orchestrator
//     can kill the primary.
//  2. Wait for the eNB to fail over to a new MME, attach a fresh UE there, and
//     verify connectivity again.
func runS1ENBFailoverConnectivity(ctx context.Context, env scenarios.Env) error {
	if len(env.CoreN2Addresses) < 2 {
		return fmt.Errorf("ha/failover_connectivity_4g requires at least 2 core addresses; got %d", len(env.CoreN2Addresses))
	}

	mmeAddrs := make([]string, 0, len(env.CoreN2Addresses))

	for _, a := range env.CoreN2Addresses {
		s1mme, err := s1mmeAddress(a)
		if err != nil {
			return err
		}

		mmeAddrs = append(mmeAddrs, s1mme)
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
		Name: "Ella-Core-Tester-S1eNB-HA", CoreS1MMEAddresses: mmeAddrs,
		ENBAddress: g.N2Address, ENBN3Address: g.N3Address, EnableDatapath: true,
	})
	if err != nil {
		return fmt.Errorf("start eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	primaryPeer := e.ActiveMMEAddress()
	logger.Logger.Info("phase1: active MME set", zap.String("peer", primaryPeer))

	if err := attachAndPing(ctx, e, e.NewUE(failoverIMSI, k, opc), "s1enbha0"); err != nil {
		return fmt.Errorf("phase1: %w", err)
	}

	logger.Logger.Info("phase1: connectivity verified", zap.String("peer", primaryPeer))

	// Stdout is mirrored to the Go test; this token triggers the primary kill.
	fmt.Println(failoverMarker)

	_ = os.Stdout.Sync()

	// When the primary is killed, SCTP read errors in the eNB receiver promote
	// the next peer.
	waitCtx, cancel := context.WithTimeout(ctx, failoverTimeout)
	newPeer, err := e.WaitForActivePeerChange(waitCtx)

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

	logger.Logger.Info("phase2: active MME switched", zap.String("from", primaryPeer), zap.String("to", newPeer))

	// The eNB promotes by dialing + sending an S1 Setup Request; the response
	// lands via the new receiver goroutine. Consuming it here ensures the new
	// MME is handshaken and ready to accept UE signalling.
	if _, err := e.WaitForMessage(s1enb.Successful, s1ap.ProcS1Setup, 10*time.Second); err != nil {
		return fmt.Errorf("phase2: wait for S1 Setup Response on new MME: %w", err)
	}

	// The re-attach bumps the subscriber's sequenceNumber, a Raft write that
	// needs a leader. Killing the primary triggers an election (a few seconds);
	// until it settles, writes fail with a leadership error and the attach is
	// rejected, so retry with backoff. Each attempt uses a fresh UE and tunnel
	// name so stale eNB-local context does not collide with the retry.
	const (
		phase2Deadline = 30 * time.Second
		phase2Backoff  = 2 * time.Second
	)

	phase2Start := time.Now()

	var phase2Err error

	for attempt := 0; time.Since(phase2Start) < phase2Deadline; attempt++ {
		phase2Err = attachAndPing(ctx, e, e.NewUE(failoverIMSI, k, opc), fmt.Sprintf("s1enbha%d", 1+attempt))
		if phase2Err == nil {
			break
		}

		logger.Logger.Warn(
			"phase2 attempt failed; waiting for leadership to settle",
			zap.Int("attempt", attempt),
			zap.String("peer", e.ActiveMMEAddress()),
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

	logger.Logger.Info("phase2: connectivity verified on new MME", zap.String("peer", newPeer))

	return nil
}
