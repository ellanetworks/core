// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

// TestIntegration4GUserPlane attaches a real srsUE and verifies bidirectional
// user-plane forwarding: a ping from the UE, through the S1-U GTP-U tunnel and
// the UPF, to the N6 data-network host and back. This exercises uplink GTP-U
// decap and the PSC-less downlink GTP-U encapsulation end to end — the user
// plane is broken if no replies come back.
func TestIntegration4GUserPlane(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if DetectIPFamily() != IPv4Only {
		t.Skip("4G integration runs in IPv4 mode only")
	}

	ctx := context.Background()
	dockerClient, _ := bring4GCoreUp(ctx, t, nil)

	defer dockerClient.ComposeCleanup(ctx)

	// The N6 host (routes the UE pool back via the UPF) and the UE.
	if err := dockerClient.ComposeUpServicesWithFile(ctx, "compose/srsenb/", ComposeFile(), "router"); err != nil {
		t.Fatalf("failed to start the n6 router: %v", err)
	}

	if err := dockerClient.ComposeUpServicesWithFile(ctx, "compose/srsenb/", ComposeFile(), "srsue"); err != nil {
		t.Fatalf("failed to start srsue: %v", err)
	}

	if !waitForAttach(ctx, t, dockerClient) {
		dumpLogs(ctx, t, dockerClient, "ella-core", "srsue", "srsenb")
		t.Fatal("UE did not reach EMM-REGISTERED")
	}

	srsue, err := dockerClient.ResolveComposeContainer(ctx, "srsenb", "srsue")
	if err != nil {
		t.Fatalf("failed to resolve srsue container: %v", err)
	}

	// Route the N6 host's subnet via the UE's TUN (the interface holding the
	// 10.45.x UE address) so both directions are symmetric — otherwise the kernel
	// reverse-path filter drops the reply arriving from off-subnet. Then ping the
	// N6 host: replies prove uplink decap + downlink (PSC-less) encap both work.
	out, _ := dockerClient.Exec(ctx, srsue,
		[]string{"sh", "-c", `DEV=$(ip -o -4 addr show | awk '/inet 10\.45\./{print $2; exit}'); ip route replace 10.6.0.0/24 dev "$DEV"; ping -c 5 -W 2 ` + N6RouterIPv4Address()},
		false, 25*time.Second, logWriter{t})

	if !strings.Contains(out, "bytes from") || strings.Contains(out, "100% packet loss") {
		dumpLogs(ctx, t, dockerClient, "ella-core", "srsue", "srsenb")
		t.Fatalf("no user-plane forwarding through the S1-U datapath; ping output:\n%s", out)
	}

	// Prove the traffic actually traversed the S1-U GTP-U tunnel rather than any
	// bridge path: srsenb logs every tunneled SDU. (The UE holds the first pool
	// address, 10.45.0.1; the N6 host is 10.6.0.3.) Both directions must appear.
	enbLogs, err := dockerClient.ComposeLogs(ctx, "compose/srsenb/", "srsenb")
	if err != nil {
		t.Fatalf("failed to read srsenb logs: %v", err)
	}

	if !strings.Contains(enbLogs, "10.45.0.1 > 10.6.0.3") {
		t.Fatal("no uplink S1-U GTP-U SDU for the ICMP in srsenb logs; traffic did not traverse the tunnel")
	}

	if !strings.Contains(enbLogs, "10.6.0.3 > 10.45.0.1") {
		t.Fatal("no downlink S1-U GTP-U SDU for the ICMP reply in srsenb logs; traffic did not traverse the tunnel")
	}

	t.Log("4G user-plane forwarding verified over the S1-U GTP-U tunnel (srsenb logged uplink + downlink SDUs)")
}

// TestIntegration4GUserPlaneIPv6 attaches a real srsUE requesting an IPv4v6 PDN
// type and verifies IPv6 user-plane forwarding: the UE completes SLAAC from the
// Router Advertisement we inject over the S1-U tunnel (PSC-less, per TS 38.415),
// then ping6 reaches the N6 host and back. SLAAC is broken if the UE never
// configures a global fd45:: address; forwarding is broken if no replies come.
// The S1-U transport stays IPv4 (compose-dualstack.yaml adds IPv6 on N6 only).
func TestIntegration4GUserPlaneIPv6(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if DetectIPFamily() != IPv4Only {
		t.Skip("4G integration runs with IPv4 S1-U transport only")
	}

	ctx := context.Background()
	dockerClient, ellaClient := bring4GCoreUpWithFile(ctx, t, "compose-dualstack.yaml", nil)

	defer dockerClient.ComposeCleanup(ctx)

	// Give the seeded "internet" data network an IPv6 pool so the converged
	// SMF+PGW-C negotiates IPv4v6 and allocates a /64 + SLAAC IID for the UE.
	dn, err := ellaClient.GetDataNetwork(ctx, &client.GetDataNetworkOptions{Name: "internet"})
	if err != nil {
		t.Fatalf("failed to read the internet data network: %v", err)
	}

	// An IPv4 DNS: Ella Core encodes DNS into the family-matching PCO container
	// (0x000D v4 / 0x0003 v6), but srsUE only parses the IPv4 one, so use v4 here
	// to get UE-side proof it consumed our PCO.
	const testDNS = "8.8.8.8"

	if err := ellaClient.UpdateDataNetwork(ctx, &client.UpdateDataNetworkOptions{
		Name:     dn.Name,
		IPv4Pool: dn.IPv4Pool,
		IPv6Pool: "fd45::/48",
		DNS:      testDNS,
		Mtu:      dn.Mtu,
	}); err != nil {
		t.Fatalf("failed to add an IPv6 pool to the internet data network: %v", err)
	}

	if err := dockerClient.ComposeUpServicesWithFile(ctx, "compose/srsenb/", "compose-dualstack.yaml", "router"); err != nil {
		t.Fatalf("failed to start the n6 router: %v", err)
	}

	if err := dockerClient.ComposeUpServicesWithFile(ctx, "compose/srsenb/", "compose-dualstack.yaml", "srsue"); err != nil {
		t.Fatalf("failed to start srsue: %v", err)
	}

	if !waitForAttach(ctx, t, dockerClient) {
		dumpLogs(ctx, t, dockerClient, "ella-core", "srsue", "srsenb")
		t.Fatal("UE did not reach EMM-REGISTERED")
	}

	srsue, err := dockerClient.ResolveComposeContainer(ctx, "srsenb", "srsue")
	if err != nil {
		t.Fatalf("failed to resolve srsue container: %v", err)
	}

	// Wait for SLAAC to configure a global fd45:: address on the UE's TUN (the
	// kernel sends a Router Solicitation, the UPF answers with the PSC-less RA
	// carrying the /64), route the N6 host's prefix via the TUN for a symmetric
	// reverse path, then ping6 the N6 host: replies prove the RA reached the UE
	// and IPv6 forwarding works both ways over S1-U.
	out, _ := dockerClient.Exec(ctx, srsue,
		[]string{"sh", "-c", `for i in $(seq 1 25); do DEV=$(ip -o -6 addr show scope global | awk '/inet6 fd45:/{print $2; exit}'); [ -n "$DEV" ] && break; sleep 1; done; echo "TUN=$DEV"; ip -6 addr show dev "$DEV"; ip -6 route replace fd00:6::/64 dev "$DEV"; ping6 -c 5 -W 2 ` + N6RouterIPv6Address()},
		false, 40*time.Second, logWriter{t})

	if !strings.Contains(out, "inet6 fd45:") {
		dumpLogs(ctx, t, dockerClient, "ella-core", "srsue", "srsenb")
		t.Fatalf("UE did not configure an IPv6 address via SLAAC; the RA did not reach it:\n%s", out)
	}

	if !strings.Contains(out, "bytes from") || strings.Contains(out, "100% packet loss") {
		dumpLogs(ctx, t, dockerClient, "ella-core", "srsue", "srsenb")
		t.Fatalf("no IPv6 user-plane forwarding through the S1-U datapath; ping output:\n%s", out)
	}

	// The core must advertise the data-network DNS to the UE via PCO (TS 24.008
	// §10.5.6.3) — SLAAC carries no DNS, so this is the only resolver an IPv6 UE
	// gets. srsUE logs the DNS it parses from the Activate Default's PCO, proving
	// the wire encoding is correct and a real UE consumes it.
	ueLogs, err := dockerClient.ComposeLogs(ctx, "compose/srsenb/", "srsue")
	if err != nil {
		t.Fatalf("failed to read srsue logs: %v", err)
	}

	if !strings.Contains(ueLogs, "DNS: "+testDNS) {
		dumpLogs(ctx, t, dockerClient, "ella-core", "srsue")
		t.Fatalf("UE did not receive the DNS server %s via PCO", testDNS)
	}

	// Prove the IPv6 ICMP actually traversed the S1-U GTP-U tunnel: srsenb logs
	// every tunneled SDU, so the N6 host's IPv6 must appear in its GTP-U traffic.
	enbLogs, err := dockerClient.ComposeLogs(ctx, "compose/srsenb/", "srsenb")
	if err != nil {
		t.Fatalf("failed to read srsenb logs: %v", err)
	}

	// srsenb tags inner IPv6 SDUs by family (it pretty-prints inner addresses for
	// IPv4 only), logging the tunnel endpoint and direction: "Tx S1-U SDU, UL >"
	// for uplink (UE→UPF) and "Rx S1-U SDU, ... > DL" for downlink (UPF→UE).
	var sawUplinkV6, sawDownlinkV6 bool

	for _, line := range strings.Split(enbLogs, "\n") {
		if !strings.Contains(line, "IPv6") {
			continue
		}

		if strings.Contains(line, "Tx S1-U SDU, UL >") {
			sawUplinkV6 = true
		}

		if strings.Contains(line, "Rx S1-U SDU,") && strings.Contains(line, "> DL") {
			sawDownlinkV6 = true
		}
	}

	if !sawUplinkV6 || !sawDownlinkV6 {
		t.Fatalf("IPv6 traffic did not traverse the S1-U GTP-U tunnel both ways (uplink=%v downlink=%v)", sawUplinkV6, sawDownlinkV6)
	}

	t.Log("4G IPv6 user-plane verified: SLAAC completed via the PSC-less RA and ping6 traversed the S1-U GTP-U tunnel")
}
