// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

const (
	natChecksumScenario    = "gnb/nat_checksum"
	natChecksumPostNATSrc  = "10.6.0.2"
	natChecksumL4Threshold = 512
	natChecksumPcapPath    = "/tmp/natcap.pcap"
)

// TestIntegration5GUPFNATChecksum drives TCP and UDP probes of varying payload size through
// source_nat with NAT enabled, captures the post-NAT frames on the router's N6
// receive side, and verifies each frame's L4 checksum with an independent
// computation.
//
// The checksum is verified on the wire rather than by the echo succeeding: on
// a veth/bridge fabric the receiver may accept a corrupt checksum
// (CHECKSUM_UNNECESSARY), so connectivity alone would not surface a bad one.
// Sizes span >512 B so a size-dependent checksum error would be caught.
func TestIntegration5GUPFNATChecksum(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	// source_nat is the path under test; IPv6 is not NATed, so this applies
	// only when N6 carries IPv4.
	if DetectIPFamily() == IPv6Only {
		t.Skip("nat checksum test exercises IPv4 source_nat; skipping in IPv6-only mode")
	}

	ctx := context.Background()
	env := setupTesterEnv(ctx, t)

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	// source_nat, which rewrites the source address/port and the L4 checksum,
	// only runs when NAT (masquerade) is enabled. Enable it and restore after.
	natOrig, err := env.Client.GetNATInfo(ctx)
	if err != nil {
		t.Fatalf("get nat info: %v", err)
	}

	if err := env.Client.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{Enabled: true}); err != nil {
		t.Fatalf("enable nat: %v", err)
	}

	t.Cleanup(func() {
		_ = env.Client.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{Enabled: natOrig.Enabled})
	})

	sc, ok := scenarios.Get(natChecksumScenario)
	Assert(t, ok, fmt.Sprintf("scenario %q not registered", natChecksumScenario))

	fx := fixture.New(t, ctx, env.Client)
	fx.Apply(sc.Fixture(buildScenariosEnv(env)))

	// Capture post-NAT uplink frames on the router's N6 rx before driving
	// the probes. tcpdump -U flushes per packet, so copying the file after
	// the scenario yields every frame seen; the privileged router container
	// is torn down at test end, which stops tcpdump.
	router, err := env.dc.ResolveComposeContainer(ctx, "core-tester", "router")
	if err != nil {
		t.Fatalf("resolve router container: %v", err)
	}

	filter := fmt.Sprintf("ip and dst host %s and dst port %d", N6RouterIPv4Address(), scenarios.DefaultProbePort)
	if _, err := env.dc.Exec(ctx, router,
		[]string{"tcpdump", "-i", "n6", "-p", "-U", "-s", "0", "-w", natChecksumPcapPath, filter},
		true, 30*time.Second, nil); err != nil {
		t.Fatalf("start tcpdump on router: %v", err)
	}

	// Let tcpdump open the capture socket before traffic flows.
	time.Sleep(2 * time.Second)

	tr := globalReporter.Start(natChecksumScenario)
	QuietLog(t, tr, "running nat checksum scenario")
	env.RunScenario(ctx, t, natChecksumScenario, tr, "--probe-payload-bytes", "16,500,800,1300")
	finishScenarioTest(t, tr)

	// Let tcpdump flush the final packets before snapshotting the file.
	time.Sleep(1 * time.Second)

	localPcap := filepath.Join(t.TempDir(), "natcap.pcap")

	cpCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cpCmd := exec.CommandContext(cpCtx, "docker", "cp", router+":"+natChecksumPcapPath, localPcap)
	if out, err := cpCmd.CombinedOutput(); err != nil {
		t.Fatalf("docker cp pcap: %v\n%s", err, string(out))
	}

	verifyNATChecksumPcap(t, localPcap)
}

func verifyNATChecksumPcap(t *testing.T, path string) {
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

	var (
		total   int
		largest int
		natSeen bool
		bad     []string
	)

	src := gopacket.NewPacketSource(r, r.LinkType())
	for pkt := range src.Packets() {
		ip4l := pkt.Layer(layers.LayerTypeIPv4)
		if ip4l == nil {
			continue
		}

		ip4, _ := ip4l.(*layers.IPv4)

		proto := uint8(ip4.Protocol)
		if proto != uint8(layers.IPProtocolTCP) && proto != uint8(layers.IPProtocolUDP) {
			continue
		}

		l4 := ip4.Payload
		if len(l4) < 8 {
			continue
		}

		total++

		if len(l4) > largest {
			largest = len(l4)
		}

		if ip4.SrcIP.String() == natChecksumPostNATSrc {
			natSeen = true
		}

		if !l4ChecksumValid(ip4.SrcIP, ip4.DstIP, proto, l4) {
			bad = append(bad, fmt.Sprintf("%s %s->%s l4len=%d stored_cksum=0x%04x",
				protoName(proto), ip4.SrcIP, ip4.DstIP, len(l4), storedL4Checksum(proto, l4)))
		}
	}

	if total == 0 {
		t.Fatalf("no post-NAT TCP/UDP frames captured to %s:%d; scenario, NAT, or capture wiring is broken",
			N6RouterIPv4Address(), scenarios.DefaultProbePort)
	}

	Assert(t, natSeen, fmt.Sprintf("captured %d frames but none sourced from %s; source_nat did not run (NAT disabled?)",
		total, natChecksumPostNATSrc))

	if largest <= natChecksumL4Threshold {
		t.Fatalf("captured %d frames but largest L4 datagram was %d bytes (<= %d); no >512 B datagram was exercised",
			total, largest, natChecksumL4Threshold)
	}

	if len(bad) > 0 {
		t.Fatalf("found %d/%d post-NAT frames with incorrect L4 checksums:\n  %s",
			len(bad), total, strings.Join(bad, "\n  "))
	}

	t.Logf("verified %d post-NAT TCP/UDP frames (largest L4 = %d bytes); all checksums correct", total, largest)
}

// l4ChecksumValid sums the IPv4 pseudo-header plus the full L4 segment
// (including its stored checksum); a correct checksum folds to 0xffff. A
// "disabled" UDP checksum of 0 would fold to a mismatch, but the probe always
// sends a real checksum, so that case does not occur here.
func l4ChecksumValid(src, dst net.IP, proto uint8, l4 []byte) bool {
	sum := pseudoHeaderSum(src, dst, proto, len(l4))
	sum += sumWords(l4)

	return foldChecksum(sum) == 0xFFFF
}

func pseudoHeaderSum(src, dst net.IP, proto uint8, l4len int) uint32 {
	s4 := src.To4()
	d4 := dst.To4()

	s := uint32(binary.BigEndian.Uint16(s4[0:2])) + uint32(binary.BigEndian.Uint16(s4[2:4]))
	s += uint32(binary.BigEndian.Uint16(d4[0:2])) + uint32(binary.BigEndian.Uint16(d4[2:4]))
	s += uint32(proto)
	s += uint32(l4len) //nolint:gosec // l4len is a packet length, always small

	return s
}

func sumWords(b []byte) uint32 {
	var s uint32

	for i := 0; i+1 < len(b); i += 2 {
		s += uint32(binary.BigEndian.Uint16(b[i : i+2]))
	}

	if len(b)%2 == 1 {
		s += uint32(b[len(b)-1]) << 8
	}

	return s
}

func foldChecksum(s uint32) uint16 {
	for s>>16 != 0 {
		s = (s & 0xFFFF) + (s >> 16)
	}

	return uint16(s) //nolint:gosec // folded value fits in 16 bits by construction
}

func storedL4Checksum(proto uint8, l4 []byte) uint16 {
	off := 16 // TCP checksum offset
	if proto == uint8(layers.IPProtocolUDP) {
		off = 6
	}

	if len(l4) < off+2 {
		return 0
	}

	return binary.BigEndian.Uint16(l4[off : off+2])
}

func protoName(proto uint8) string {
	if proto == uint8(layers.IPProtocolUDP) {
		return "udp"
	}

	return "tcp"
}
