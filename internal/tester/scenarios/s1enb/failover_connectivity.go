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

// failoverMarker is printed to stdout after the phase-1 flow completes,
// signalling the orchestrator (integration test) that the primary core can now
// be killed. Must match the value the test scans for.
const failoverMarker = "PHASE1_DONE"

// failoverTimeout caps the wait for the primary's SCTP association to drop and
// the eNB to pick a new active MME. Kernel-level SCTP drop detection on a
// killed container is typically sub-second; 60s is a generous bound.
const failoverTimeout = 60 * time.Second

const failoverIMSI = "001017271246606"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/failover_connectivity",
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

// runS1ENBFailoverConnectivity is the 4G counterpart of
// ha/failover_connectivity, a two-phase scenario:
//
//  1. Attach a UE on the eNB's primary MME (CoreS1MMEAddresses[0]), establish a
//     bearer, verify connectivity by pinging through the tunnel. Emit the
//     phase-1 marker on stdout so the orchestrator can kill the primary.
//  2. Wait for the eNB to fail over to a new MME. Attach a fresh UE on the new
//     MME, establish a bearer, verify connectivity again.
//
// Exits 0 only if both phases succeed.
func runS1ENBFailoverConnectivity(ctx context.Context, env scenarios.Env) error {
	if len(env.CoreN2Addresses) < 2 {
		return fmt.Errorf("s1enb/failover_connectivity requires at least 2 core addresses; got %d", len(env.CoreN2Addresses))
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

	// Signal the orchestrator. Stdout is mirrored back to the Go test; a simple
	// substring match on this token triggers the kill.
	fmt.Println(failoverMarker)

	_ = os.Stdout.Sync()

	// Wait for the eNB to switch to a different active MME. Triggered when the
	// orchestrator kills the primary core: SCTP read errors in the eNB's
	// receiver, which promotes the next peer in the list.
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

	// Phase 2 re-attaches a fresh UE. The attach bumps the subscriber's
	// sequenceNumber, a Raft-replicated write forwarded follower→leader
	// in-process, so any surviving peer can service it — once a new leader is
	// elected. Killing the previous leader starts a heartbeat-timeout →
	// election cycle (a few seconds); until it settles, writes return a
	// leadership error and the NAS layer rejects the attach. Retry against the
	// same peer with a small backoff until the leader settles. Each attempt
	// uses a fresh UE (fresh eNB-UE-S1AP-ID) and tunnel name so stale eNB-local
	// context from a failed attempt doesn't collide with the retry.
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
