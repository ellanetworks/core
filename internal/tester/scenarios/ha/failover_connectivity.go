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
		Name: "ha/failover_connectivity",
		BindFlags: func(fs *pflag.FlagSet) any {
			return struct{}{}
		},
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runFailoverConnectivity(ctx, env)
		},
		Fixture: func() scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
			}
		},
	})
}

// runFailoverConnectivity is a two-phase scenario:
//
//  1. Register the UE on the gNB's primary peer (CoreN2Addresses[0]),
//     establish a PDU session, verify connectivity by pinging through the
//     tunnel. Emit the phase-1 marker on stdout so the orchestrator can
//     kill the primary.
//  2. Wait for the gNB to fail over to a new peer. Register a fresh UE
//     (new RAN-UE-NGAP-ID) on the new peer, establish a PDU session,
//     verify connectivity again.
//
// Exits 0 only if both phases succeed.
func runFailoverConnectivity(ctx context.Context, env scenarios.Env) error {
	if len(env.CoreN2Addresses) < 2 {
		return fmt.Errorf("ha/failover_connectivity requires at least 2 core addresses; got %d", len(env.CoreN2Addresses))
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

	// Phase 1: register + connectivity on the primary peer.
	if err := registerAndPing(
		ctx,
		gNodeB,
		int64(scenarios.DefaultRANUENGAPID),
		"ellaha0",
	); err != nil {
		return fmt.Errorf("phase1: %w", err)
	}

	logger.Logger.Info("phase1: connectivity verified", zap.String("peer", primaryPeer))

	// Signal the orchestrator. Stdout is mirrored back to the Go test; a
	// simple substring match on this token triggers the kill.
	fmt.Println(failoverMarker)

	_ = os.Stdout.Sync()

	// Wait for the gNB to switch to a different active peer. Triggered
	// when the orchestrator kills the primary core: SCTP read errors in
	// the gNB's receiver, which promotes the next peer in the list.
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

	// Wait for the new peer's NG Setup Response. gNB promotes by dialing
	// + sending NGSetupRequest; the response lands in receivedFrames via
	// the new receiver goroutine. Consuming it here ensures the new AMF
	// is handshaken and ready to accept UE signalling.
	if _, err := gNodeB.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentNGSetupResponse,
		10*time.Second,
	); err != nil {
		return fmt.Errorf("phase2: wait for NG Setup Response on new peer: %w", err)
	}

	// Phase 2 triggers AUSF to bump the subscriber's sequenceNumber, a
	// Raft-replicated write. Since Core now forwards Propose calls
	// follower→leader in-process, ANY surviving peer can service the
	// Registration Request: writes issued against a follower land on
	// the current leader via /cluster/internal/propose.
	//
	// What we still need to wait for: the election itself. Killing the
	// previous leader starts a heartbeat-timeout → election cycle (a
	// few seconds). Until a new leader is elected, forwarded writes
	// return ErrLeadershipLost and the NAS layer sends Registration
	// Reject. We retry against the same peer with a small backoff
	// until the leader settles.
	//
	// Each attempt uses a fresh RAN-UE-NGAP-ID and tunnel name so stale
	// gNB-local context from a failed attempt doesn't collide with the
	// retry.
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

// registerAndPing wraps common.RegisterAndPing with the default
// subscriber. Phase 2 reuses the IMSI under a fresh RAN-UE-NGAP-ID
// and tunnel name to avoid colliding with phase 1's gNB state.
func registerAndPing(ctx context.Context, gNodeB *gnb.GnodeB, ranUENGAPID int64, tunInterfaceName string) error {
	return common.RegisterAndPing(ctx, &common.RegisterAndPingOpts{
		GNB:              gNodeB,
		RANUENGAPID:      ranUENGAPID,
		PDUSessionID:     scenarios.DefaultPDUSessionID,
		IMSI:             scenarios.DefaultIMSI,
		TunInterfaceName: tunInterfaceName,
	})
}
