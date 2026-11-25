// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/suites"
)

func TestIntegration5GUERANSIM(t *testing.T) {
	suites.Require(t, suites.Datapath5G)

	// UERANSIM does not correctly handle dual-stack NGAP TransportLayerAddress,
	// so skip this integration until upstream dual-stack support is fixed.
	if DetectIPFamily() == DualStack {
		t.Skip("skipping UERANSIM integration in dual-stack mode")
	}

	// UERANSIM does not support IPv6 PDU session type so we skip this test.
	if DetectIPFamily() == IPv6Only {
		t.Skip("skipping UERANSIM integration in IPv6 mode")
	}

	testCases := []struct {
		name string
		nat  bool
	}{
		{
			name: "Nat disabled",
			nat:  false,
		},
		{
			name: "Nat enabled",
			nat:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			dockerClient, err := NewDockerClient()
			if err != nil {
				t.Fatalf("failed to create docker client: %v", err)
			}

			defer func() {
				err := dockerClient.Close()
				if err != nil {
					t.Fatalf("failed to close docker client: %v", err)
				}
			}()

			dockerClient.ComposeCleanup(ctx)

			err = dockerClient.ComposeUpWithFile(ctx, "compose/ueransim/", ComposeFile())
			if err != nil {
				t.Fatalf("failed to bring up compose: %v", err)
			}

			t.Log("deployed ella core")

			clientConfig := &client.Config{
				BaseURL: APIAddress(),
			}

			ellaClient, err := client.New(clientConfig)
			if err != nil {
				t.Fatalf("failed to create ella client: %v", err)
			}

			err = waitForEllaCoreReady(ctx, ellaClient)
			if err != nil {
				t.Fatalf("failed to wait for ella core to be ready: %v", err)
			}

			t.Log("ella core is ready")

			routes := []RouteConfig{
				{
					Destination: "8.8.8.8/32",
					Gateway:     N6RouterIPv4Address(),
					Interface:   "n6",
					Metric:      0,
				},
			}

			if DetectIPFamily() == IPv6Only {
				routes = []RouteConfig{
					{
						Destination: "2001:4860:4860::8888/128",
						Gateway:     N6RouterIPv6Address(),
						Interface:   "n6",
						Metric:      0,
					},
				}
			}

			err = configureEllaCore(ctx, ellaClient, EllaCoreConfig{
				Networking: NetworkingConfig{
					NAT:    tc.nat,
					Routes: routes,
				},
				Subscribers: []SubscriberConfig{
					{
						Imsi:           "001019756139935",
						Key:            "0eefb0893e6f1c2855a3a244c6db1277",
						OPc:            "98da19bbc55e2a5b53857d10557b1d26",
						SequenceNumber: "000000000022",
						ProfileName:    "default",
					},
				},
			})
			if err != nil {
				t.Fatalf("failed to configure Ella Core: %v", err)
			}

			t.Log("configured Ella Core")

			ueransimContainerName, err := dockerClient.ResolveComposeContainer(ctx, "ueransim", "ueransim")
			if err != nil {
				t.Fatalf("failed to resolve ueransim container: %v", err)
			}

			_, err = dockerClient.Exec(
				ctx, ueransimContainerName,
				[]string{
					"sh", "-c",
					`nohup bin/nr-gnb --config /gnb.yaml >/tmp/gnb.log 2>&1 </dev/null & echo $! >/tmp/gnb.pid`,
				},
				true,
				5*time.Second,
				logWriter{t},
			)
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}

			t.Log("started pebble gnb")

			if err := waitForPatternInContainer(ctx, dockerClient, ueransimContainerName,
				`/tmp/gnb.log`, `NG Setup procedure is successful`, 15*time.Second, 200*time.Millisecond); err != nil {
				t.Fatalf("Gnb never became ready: %v", err)
			}

			t.Log("GNB reported ready")

			_, err = dockerClient.Exec(
				ctx, ueransimContainerName,
				[]string{
					"sh", "-c",
					`nohup bin/nr-ue --config /ue.yaml >/tmp/ue.log 2>&1 </dev/null & echo $! >/tmp/ue.pid`,
				},
				true,
				5*time.Second,
				logWriter{t},
			)
			if err != nil {
				t.Fatalf("failed to start UE: %v", err)
			}

			t.Log("started UERANSIM UE")

			ueReadyPattern := `TUN interface\[uesimtun0, 10\.45\.0\.1\] is up`

			if DetectIPFamily() == IPv6Only {
				ueReadyPattern = `TUN interface\[uesimtun0, fd45::\d+\] is up`
			}

			if err := waitForPatternInContainer(ctx, dockerClient, ueransimContainerName,
				`/tmp/ue.log`, ueReadyPattern, 15*time.Second, 200*time.Millisecond); err != nil {
				t.Fatalf("UE never became ready: %v", err)
			}

			t.Log("UE reported ready")

			result, err := dockerClient.Exec(ctx, ueransimContainerName, []string{"ip", "a"}, false, 5*time.Second, logWriter{t})
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}

			t.Logf("UERANSIM result: %s", result)

			Assert(t, strings.Contains(result, "uesimtun0"), "'uesimtun0' interface not found in 'ip a' output")

			pingDest := N6RouterIPv4Address()
			pingCmd := "ping"

			if DetectIPFamily() == IPv6Only {
				pingDest = N6RouterIPv6Address()
				pingCmd = "ping6"
			}

			result, err = dockerClient.Exec(ctx, ueransimContainerName, []string{pingCmd, "-I", "uesimtun0", pingDest, "-c", "3"}, false, 10*time.Second, logWriter{t})
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}

			t.Logf("UERANSIM ping result: %s", result)

			Assert(t, strings.Contains(result, "3 packets transmitted, 3 received"), fmt.Sprintf("expected 3/3 ping packets received, but got: %s", result))
			Assert(t, strings.Contains(result, "0% packet loss"), fmt.Sprintf("expected 0%% packet loss, but got: %s", result))

			err = dockerClient.CopyFileToContainer(ctx, ueransimContainerName, "network_test.py", "/network_test.py")
			if err != nil {
				t.Fatalf("failed to copy testing script: %v", err)
			}

			t.Logf("testing script copied")

			result, err = dockerClient.Exec(
				ctx,
				ueransimContainerName,
				[]string{"python3", "/network_test.py", "--dev", "uesimtun0", "--dest", pingDest},
				false,
				5*time.Second,
				logWriter{t},
			)
			if err != nil {
				t.Fatalf("networking test suite failed: %v", err)
			}

			t.Logf("Network tester results: %s", result)

			_, err = dockerClient.Exec(ctx, ueransimContainerName, []string{"ip", "link", "set", "uesimtun0", "mtu", "1500"}, false, 5*time.Second, logWriter{t})
			if err != nil {
				t.Fatalf("failed to raise the UE MTU: %v", err)
			}

			result, err = dockerClient.Exec(ctx, ueransimContainerName, []string{pingCmd, "-I", "uesimtun0", "-M", "dont", "-s", "1472", pingDest, "-c", "3"}, false, 10*time.Second, logWriter{t})
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}

			t.Logf("UERANSIM large ping result: %s", result)

			Assert(t, strings.Contains(result, "3 packets transmitted, 3 received"), fmt.Sprintf("expected 3/3 1500-byte ping packets received over a 1500-byte N3, but got: %s", result))

			routerContainerName, err := dockerClient.ResolveComposeContainer(ctx, "ueransim", "router")
			if err != nil {
				t.Fatalf("failed to resolve router container: %v", err)
			}

			_, err = dockerClient.Exec(ctx, routerContainerName, []string{"sh", "-c", `nohup timeout 30 python3 -c '
import socket
s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
s.setsockopt(socket.IPPROTO_IP, 10, 0)
s.bind(("", 5001))
while True:
    d, a = s.recvfrom(2048)
    s.sendto(b"x" * 1472, a)
' >/tmp/udp1500.log 2>&1 </dev/null &`}, true, 5*time.Second, logWriter{t})
			if err != nil {
				t.Fatalf("failed to start the 1500-byte UDP responder: %v", err)
			}

			result, err = dockerClient.Exec(ctx, ueransimContainerName, []string{"python3", "-c", `
import socket, sys
s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
s.setsockopt(socket.SOL_SOCKET, socket.SO_BINDTODEVICE, b"uesimtun0")
s.settimeout(1)
for _ in range(10):
    s.sendto(b"x", (sys.argv[1], 5001))
    try:
        print("received", len(s.recv(2048)))
        break
    except socket.timeout:
        pass
`, pingDest}, false, 20*time.Second, logWriter{t})
			if err != nil {
				t.Fatalf("failed to exec command in pod: %v", err)
			}

			Assert(t, strings.Contains(result, "received 1472"), fmt.Sprintf("expected a 1500-byte UDP reply, sent with checksum offload, to reach the UE over a 1500-byte N3, but got: %s", result))

			dockerClient.ComposeDownWithFile(ctx, "compose/ueransim", ComposeFile())
		})
	}
}
