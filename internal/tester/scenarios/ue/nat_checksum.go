// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const natChecksumIMSI = "001010000000042"

// natChecksumDefaultSizes spans small and large (>512 B) L4 datagrams so the
// capture-verify test checks egress checksums across a range of sizes.
const natChecksumDefaultSizes = "16,500,800,1300"

type natChecksumParams struct {
	PayloadBytes string
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "ue/nat_checksum",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &natChecksumParams{PayloadBytes: natChecksumDefaultSizes}
			fs.StringVar(&p.PayloadBytes, "probe-payload-bytes", p.PayloadBytes,
				"comma-separated L4 payload sizes to sweep for TCP and UDP probes")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runNATChecksum(ctx, env, params.(*natChecksumParams))
		},
		Fixture: func(scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{
					scenarios.DefaultSubscriberWith(natChecksumIMSI, ""),
				},
			}
		},
	})
}

func parsePayloadSizes(csv string) ([]int, error) {
	parts := strings.Split(csv, ",")
	sizes := make([]int, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid payload size %q: %w", p, err)
		}

		if n < 0 {
			return nil, fmt.Errorf("payload size must be non-negative, got %d", n)
		}

		sizes = append(sizes, n)
	}

	if len(sizes) == 0 {
		return nil, fmt.Errorf("no payload sizes provided")
	}

	return sizes, nil
}

// runNATChecksum brings up a single UE + PDU session, then sweeps TCP and
// UDP probes of varying payload size through source_nat. It asserts only
// that the probes complete; the egress L4 checksum is verified out-of-band
// by the integration test, which captures the post-NAT frames on N6.
func runNATChecksum(ctx context.Context, env scenarios.Env, params *natChecksumParams) error {
	sizes, err := parsePayloadSizes(params.PayloadBytes)
	if err != nil {
		return err
	}

	subs, err := buildSubscribers(1, natChecksumIMSI)
	if err != nil {
		return fmt.Errorf("could not build subscriber config: %v", err)
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
		Name:            "Ella-Core-Tester",
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    g.N2Address,
		GnbN3Address:    g.N3Address,
	})
	if err != nil {
		return fmt.Errorf("error starting gNB: %v", err)
	}

	defer gNodeB.Close()

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive NG Setup Response: %v", err)
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)
	tunInterfaceName := gtpInterfaceNamePrefix + "0"

	newUE, err := newDefaultUE(gNodeB, subs[0].IMSI[5:], subs[0].Key, subs[0].OPc, subs[0].SequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	uePDUSession, err := newUE.WaitForPDUSession(scenarios.DefaultPDUSessionID, 5*time.Second)
	if err != nil {
		return fmt.Errorf("timeout waiting for PDU session: %v", err)
	}

	uePduSession := newUE.GetPDUSession(scenarios.DefaultPDUSessionID)
	ueIP := uePduSession.UEIP + "/16"

	gnbPDUSession, err := gNodeB.WaitForPDUSession(ranUENGAPID, int64(scenarios.DefaultPDUSessionID), 5*time.Second)
	if err != nil {
		return fmt.Errorf("could not get PDU Session for RAN UE NGAP ID %d: %v", ranUENGAPID, err)
	}

	_, err = gNodeB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP,
		UpfIP:            gnbPDUSession.UpfAddress,
		TunInterfaceName: tunInterfaceName,
		ULteid:           gnbPDUSession.ULTeid,
		DLteid:           gnbPDUSession.DLTeid,
		MTU:              uePDUSession.MTU,
		QFI:              uePduSession.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create GTP tunnel (name: %s, DL TEID: %d): %v", tunInterfaceName, gnbPDUSession.DLTeid, err)
	}

	defer func() { _ = gNodeB.CloseTunnel(gnbPDUSession.DLTeid) }()

	dst := env.PingDestination()

	for _, size := range sizes {
		payload := makeProbePayload(size)

		if err := sendTCPProbe(ctx, tunInterfaceName, dst, scenarios.DefaultProbePort, probeAttemptCount, probeAttemptTimeout, payload); err != nil {
			return fmt.Errorf("tcp probe (payload=%d) failed: %v", size, err)
		}

		if err := sendUDPProbe(ctx, tunInterfaceName, dst, scenarios.DefaultProbePort, probeAttemptCount, probeAttemptTimeout, payload); err != nil {
			return fmt.Errorf("udp probe (payload=%d) failed: %v", size, err)
		}

		logger.Logger.Debug("nat_checksum probe sweep step complete",
			zap.Int("payload_bytes", size),
			zap.String("destination", dst),
		)
	}

	return nil
}
