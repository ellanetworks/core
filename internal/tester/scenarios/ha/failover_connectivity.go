// Package ha holds core-tester scenarios that exercise multi-core (HA)
// behaviour from the RAN side.
package ha

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

// PDUSessionType mirrors the constant in scenarios/ue. Redefined here
// because this package does not import scenarios/ue.
const pduSessionType = nasMessage.PDUSessionTypeIPv4

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

	// Phase 2 will trigger AUSF to bump the subscriber's sequenceNumber,
	// which is a Raft-replicated write and therefore must land on the
	// current leader. When we killed the previous leader (core-1), the
	// surviving nodes need to run a heartbeat-timeout → election cycle
	// before any write can succeed. Typical election time under 3-node
	// compose is 5-8 s. Retry the full register-and-ping cycle until the
	// cluster settles; each attempt uses a fresh RAN-UE-NGAP-ID and
	// tunnel name so stale gNB-local context from a failed attempt
	// doesn't collide with the retry.
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
			"phase2 attempt failed; will retry until raft settles",
			zap.Int("attempt", attempt),
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

// registerAndPing drives one UE through Initial Registration, PDU Session
// Establishment, GTP tunnel setup, and a ping through the tunnel. It
// reuses the default subscriber (scenarios.DefaultIMSI, etc.) for both
// phases — the fixture provisions it once.
func registerAndPing(ctx context.Context, gNodeB *gnb.GnodeB, ranUENGAPID int64, tunInterfaceName string) error {
	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: pduSessionType,
		Msin:           scenarios.DefaultIMSI[5:],
		K:              scenarios.DefaultKey,
		OpC:            scenarios.DefaultOPC,
		Amf:            scenarios.DefaultAMF,
		Sqn:            scenarios.DefaultSequenceNumber,
		Mcc:            scenarios.DefaultMCC,
		Mnc:            scenarios.DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.NullScheme,
			PublicKeyID:      "0",
		},
		RoutingIndicator: scenarios.DefaultRoutingIndicator,
		DNN:              scenarios.DefaultDNN,
		Sst:              scenarios.DefaultSST,
		Sd:               scenarios.DefaultSD,
		IMEISV:           scenarios.DefaultIMEISV,
		UeSecurityCapability: testutil.GetUESecurityCapability(&testutil.UeSecurityCapability{
			Integrity: testutil.IntegrityAlgorithms{
				Nia2: true,
			},
			Ciphering: testutil.CipheringAlgorithms{
				Nea0: true,
				Nea2: true,
			},
		}),
	})
	if err != nil {
		return fmt.Errorf("create UE: %w", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	}); err != nil {
		return fmt.Errorf("initial registration: %w", err)
	}

	if _, err := newUE.WaitForPDUSession(scenarios.DefaultPDUSessionID, 5*time.Second); err != nil {
		return fmt.Errorf("wait UE PDU session: %w", err)
	}

	uePduSession := newUE.GetPDUSession(scenarios.DefaultPDUSessionID)
	ueIP := uePduSession.UEIP + "/16"

	gnbPDUSession, err := gNodeB.WaitForPDUSession(ranUENGAPID, int64(scenarios.DefaultPDUSessionID), 5*time.Second)
	if err != nil {
		return fmt.Errorf("wait gNB PDU session: %w", err)
	}

	if _, err := gNodeB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP,
		UpfIP:            gnbPDUSession.UpfAddress,
		TunInterfaceName: tunInterfaceName,
		ULteid:           gnbPDUSession.ULTeid,
		DLteid:           gnbPDUSession.DLTeid,
		MTU:              uePduSession.MTU,
		QFI:              uePduSession.QFI,
	}); err != nil {
		return fmt.Errorf("create GTP tunnel %q: %w", tunInterfaceName, err)
	}

	// #nosec G204 -- ping is fixed; tunInterfaceName is internally derived; PingDestination is a test constant
	cmd := exec.CommandContext(ctx, "ping",
		"-I", tunInterfaceName,
		scenarios.DefaultPingDestination,
		"-c", "3",
		"-W", "1",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ping via %s: %w\noutput:\n%s", tunInterfaceName, err, string(out))
	}

	logger.Logger.Debug(
		"ping ok",
		zap.String("interface", tunInterfaceName),
		zap.String("destination", scenarios.DefaultPingDestination),
	)

	return nil
}
