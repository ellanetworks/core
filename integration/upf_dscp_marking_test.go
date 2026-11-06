// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/integration/suites"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

const (
	dscpPcapPath        = "/tmp/dscpcap.pcap"
	dscpProbeSourcePort = 24000
)

func runDSCPMarking(t *testing.T, ranPrefix string) {
	t.Helper()

	fp := familyParams(DetectIPFamily(), ranPrefix)

	ctx := context.Background()
	env := setupTesterEnv(ctx, t)

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())

	signalling := fixture.DefaultPolicySpec()
	signalling.Var5qi = 5

	baseline.Policy(signalling)

	sc, ok := scenarios.Get(fp.scenarioAllowed)
	Assert(t, ok, fmt.Sprintf("scenario %q not registered", fp.scenarioAllowed))

	var spec scenarios.FixtureSpec
	if sc.Fixture != nil {
		spec = sc.Fixture(buildScenariosEnv(env))
	}

	fixture.New(t, ctx, env.Client).Apply(spec)

	if _, err := env.dc.Exec(ctx, env.TesterContainer,
		[]string{
			"tcpdump", "-Z", "root", "-i", "n3", "-p", "-U", "-s", "0", "-w", dscpPcapPath,
			"udp", "port", "2152",
		},
		true, 30*time.Second, nil); err != nil {
		t.Fatalf("start tcpdump on the tester: %v", err)
	}

	time.Sleep(2 * time.Second)

	pp := protocolParams(fp.family, "icmp")
	tr := globalReporter.Start(fp.scenarioAllowed)
	QuietLog(t, tr, "running connectivity scenario for DSCP marking")
	env.RunScenario(ctx, t, fp.scenarioAllowed, tr, scenarioRunArgs(fp.scenarioAllowed, spec, pp, dscpProbeSourcePort)...)
	finishScenarioTest(t, tr)

	time.Sleep(1 * time.Second)

	localPcap := filepath.Join(t.TempDir(), "dscpcap.pcap")

	cpCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cpCmd := exec.CommandContext(cpCtx, "docker", "cp", env.TesterContainer+":"+dscpPcapPath, localPcap)
	if out, err := cpCmd.CombinedOutput(); err != nil {
		t.Fatalf("docker cp pcap: %v\n%s", err, string(out))
	}

	verifyDSCPMarkingPcap(t, localPcap, net.ParseIP(env.GNBs[0].N3Address))
}

func verifyDSCPMarkingPcap(t *testing.T, path string, gnbN3 net.IP) {
	t.Helper()

	f, err := os.Open(path) // #nosec G304 -- test-controlled temp path
	if err != nil {
		t.Fatalf("open pcap: %v", err)
	}

	defer f.Close() //nolint:errcheck

	r, err := pcapgo.NewReader(f)
	if err != nil {
		t.Fatalf("new pcap reader: %v", err)
	}

	const (
		gtpTPDU       = 0xFF
		signallingToS = 0xA0
	)

	var downlink int

	seen := map[string]int{}

	src := gopacket.NewPacketSource(r, r.LinkType())
	for pkt := range src.Packets() {
		var (
			tos    byte
			dst    net.IP
			srcIP  net.IP
			isUDPv bool
			isUDP6 bool
		)

		if l := pkt.Layer(layers.LayerTypeIPv4); l != nil {
			ip4, ok := l.(*layers.IPv4)
			if !ok {
				continue
			}

			tos, dst, srcIP = ip4.TOS, ip4.DstIP, ip4.SrcIP
			isUDPv = ip4.Protocol == layers.IPProtocolUDP
		} else if l := pkt.Layer(layers.LayerTypeIPv6); l != nil {
			ip6, ok := l.(*layers.IPv6)
			if !ok {
				continue
			}

			tos, dst, srcIP = ip6.TrafficClass, ip6.DstIP, ip6.SrcIP
			isUDP6 = ip6.NextHeader == layers.IPProtocolUDP
		} else {
			continue
		}

		if !isUDPv && !isUDP6 {
			continue
		}

		udpLayer := pkt.Layer(layers.LayerTypeUDP)
		if udpLayer == nil {
			continue
		}

		udp, ok := udpLayer.(*layers.UDP)
		if !ok {
			continue
		}

		payload := udp.Payload

		gtpType := byte(0)
		if len(payload) > 1 {
			gtpType = payload[1]
		}

		seen[fmt.Sprintf("%s -> %s tos=%#04x gtp=%#04x", srcIP, dst, tos, gtpType)]++

		if !dst.Equal(gnbN3) {
			continue
		}

		if len(payload) < 8 || payload[1] != gtpTPDU {
			continue
		}

		downlink++

		if tos != signallingToS {
			t.Errorf("downlink G-PDU with outer ToS %#04x, want the signalling marking %#04x", tos, signallingToS)
		}
	}

	if downlink < probeRoundtrips {
		for s, n := range seen {
			t.Logf("captured %d x %s", n, s)
		}

		t.Fatalf("captured %d downlink G-PDUs, want at least %d", downlink, probeRoundtrips)
	}
}

// TS 23.501 §5.8.2.7
func TestIntegration5GUPFDSCPMarking(t *testing.T) {
	suites.Require(t, suites.Datapath5G)

	runDSCPMarking(t, "gnb")
}

// TS 29.244 §5.4.13
func TestIntegration4GUPFDSCPMarking(t *testing.T) {
	suites.Require(t, suites.Datapath4G)

	runDSCPMarking(t, "s1enb")
}
