// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const natChecksumIMSI = "001017271246614"

// natChecksumDefaultSizes spans small and large (>512 B) L4 datagrams so the
// capture-verify test checks egress checksums across a range of sizes.
const natChecksumDefaultSizes = "16,500,800,1300"

const natChecksumTunIface = "s1enbnatck0"

type natChecksumParams struct {
	PayloadBytes string
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "s1enb/nat_checksum",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &natChecksumParams{PayloadBytes: natChecksumDefaultSizes}
			fs.StringVar(&p.PayloadBytes, "probe-payload-bytes", p.PayloadBytes,
				"comma-separated L4 payload sizes to sweep for TCP and UDP probes")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runS1ENBNATChecksum(ctx, env, params.(*natChecksumParams))
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

// runS1ENBNATChecksum attaches a single 4G UE + default bearer, then sweeps TCP
// and UDP probes of varying payload size through source_nat. It asserts only that
// the probes complete; the egress L4 checksum is verified out-of-band by the
// integration test, which captures the post-NAT frames on N6. The 4G counterpart
// of gnb/nat_checksum. source_nat is IPv4-only, so this scenario is IPv4-only.
func runS1ENBNATChecksum(ctx context.Context, env scenarios.Env, params *natChecksumParams) error {
	sizes, err := parsePayloadSizes(params.PayloadBytes)
	if err != nil {
		return err
	}

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	s1mme, err := s1mmeAddress(env.FirstCore())
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
		Name: s1enbName, CoreS1MMEAddress: s1mme,
		ENBAddress: g.N2Address, ENBN3Address: g.N3Address, EnableDatapath: true,
	})
	if err != nil {
		return fmt.Errorf("start eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(natChecksumIMSI, k, opc)

	res, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if res.UEIPv4 == "" {
		return fmt.Errorf("attach assigned no IPv4 address")
	}

	if err := e.AddTunnel(&s1enb.TunnelOpts{
		UEIPv4:           res.UEIPv4 + "/16",
		UpfAddress:       res.UpfAddress,
		ULTEID:           res.ULTEID,
		DLTEID:           res.DLTEID,
		TunInterfaceName: natChecksumTunIface,
	}); err != nil {
		return fmt.Errorf("add GTP tunnel: %w", err)
	}

	defer e.CloseTunnel(res.DLTEID)

	// Let the UPF program the downlink endpoint before probing.
	time.Sleep(500 * time.Millisecond)

	dst := scenarios.DefaultPingDestination

	for _, size := range sizes {
		payload := probe.MakePayload(size)

		if err := probe.SendTCP(ctx, natChecksumTunIface, dst, scenarios.DefaultProbePort, probe.AttemptCount, probe.AttemptTimeout, payload); err != nil {
			return fmt.Errorf("tcp probe (payload=%d) failed: %w", size, err)
		}

		if err := probe.SendUDP(ctx, natChecksumTunIface, dst, scenarios.DefaultProbePort, probe.AttemptCount, probe.AttemptTimeout, payload); err != nil {
			return fmt.Errorf("udp probe (payload=%d) failed: %w", size, err)
		}

		logger.Logger.Debug("nat_checksum probe sweep step complete",
			zap.Int("payload_bytes", size),
			zap.String("destination", dst),
		)
	}

	return nil
}
