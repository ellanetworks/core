// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/integration/suites"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"golang.org/x/sync/errgroup"
)

const (
	scaleProvisionWorkers = 16
	scaleStartIMSI        = 1017000000001
	scaleMaxRadios        = 150
	scaleLargeUEIPv4Pool  = "10.44.0.0/14"
	scaleLargePoolAbove   = 65000
	scaleDiskOverlay      = "disk.yaml"
)

var (
	scaleSubscribers = scaleEnvInt("ELLA_SCALE_SUBSCRIBERS", 1000)
	scaleRadios      = scaleEnvInt("ELLA_SCALE_RADIOS", 10)
	scaleConcurrency = scaleEnvInt("ELLA_SCALE_CONCURRENCY", 128)
	scaleRATs        = scaleEnvString("ELLA_SCALE_RATS", "5g,4g")
)

func scaleEnvInt(name string, def int) int {
	v, err := strconv.Atoi(os.Getenv(name))
	if err != nil || v <= 0 {
		return def
	}

	return v
}

func scaleEnvString(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}

	return def
}

func TestIntegrationScale1k(t *testing.T) {
	suites.Require(t, suites.Scale)

	if scaleRadios > scaleMaxRadios {
		t.Fatalf("ELLA_SCALE_RADIOS = %d, at most %d", scaleRadios, scaleMaxRadios)
	}

	ctx := context.Background()

	var overlays []string
	if os.Getenv("ELLA_CORE_DATA_DIR") != "" {
		overlays = append(overlays, scaleDiskOverlay)
	}

	env := setupTesterEnv(ctx, t, overlays...)
	env.ScenarioTimeout = 5*time.Minute + time.Duration(scaleSubscribers)*10*time.Millisecond
	env.Quiet = scaleSubscribers > 1000

	dataNetwork := fixture.DefaultDataNetworkSpec()
	if scaleSubscribers > scaleLargePoolAbove {
		dataNetwork.IPv4Pool = scaleLargeUEIPv4Pool

		routeScalePoolToCore(ctx, t, env)
	}

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(dataNetwork)
	baseline.Policy(fixture.DefaultPolicySpec())

	provisionScaleSubscribers(ctx, t)

	radios := addScaleRadioAddresses(ctx, t, env)

	logScaleMemory(t, "idle")

	for _, tc := range []struct{ name, scenario string }{
		{"5g", "gnb/scale"},
		{"4g", "s1enb/scale"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(scaleRATs, tc.name) {
				t.Skipf("ELLA_SCALE_RATS = %q", scaleRATs)
			}

			tr := registerScenarioTest(fmt.Sprintf("%s/%dk", tc.scenario, scaleSubscribers/1000))

			args := []string{
				"--ues", strconv.Itoa(scaleSubscribers),
				"--start-imsi", scaleIMSI(0),
				"--concurrency", strconv.Itoa(scaleConcurrency),
				"--probe-every", strconv.Itoa(max(50, scaleSubscribers/100)),
				"--timeout", "2m",
			}

			for i, addr := range radios {
				args = append(args, "--gnb", fmt.Sprintf("scale-%s%d,n2=%s,n3=%s", tc.name, i, addr, addr))
			}

			peak := sampleScaleMemory()
			out := env.RunScenario(ctx, t, tc.scenario, tr, args...)
			rss, maps := peak()

			for line := range strings.SplitSeq(out, "\n") {
				for _, summary := range []string{"deregistration complete\t", "registration complete\t", "user plane probes complete\t", "over capacity rejected\t"} {
					if i := strings.Index(line, summary); i >= 0 {
						t.Log(line[i:])

						break
					}
				}
			}

			t.Logf("peak memory: process RSS %d MiB, BPF maps %d MiB", rss>>20, maps>>20)
		})
	}
}

func routeScalePoolToCore(ctx context.Context, t *testing.T, env *testerEnv) {
	t.Helper()

	router, err := env.dc.ResolveComposeContainer(ctx, "core-tester", "router")
	if err != nil {
		t.Fatalf("resolve router container: %v", err)
	}

	argv := []string{"ip", "route", "replace", scaleLargeUEIPv4Pool, "dev", "n6", "via", "10.6.0.2"}
	if _, err := env.dc.Exec(ctx, router, argv, false, 30*time.Second, nil); err != nil {
		t.Fatalf("route %s to the core: %v", scaleLargeUEIPv4Pool, err)
	}
}

func scaleMemory() (rss, maps uint64, err error) {
	body, err := scrapeMetrics(APIAddress())
	if err != nil {
		return 0, 0, err
	}

	sc := bufio.NewScanner(bytes.NewReader(body))
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)

	for sc.Scan() {
		line := sc.Text()

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		v, perr := strconv.ParseFloat(fields[len(fields)-1], 64)
		if perr != nil {
			continue
		}

		switch {
		case strings.HasPrefix(line, "process_resident_memory_bytes "):
			rss = uint64(v)
		case strings.HasPrefix(line, "app_upf_bpf_map_memory_bytes{"):
			maps += uint64(v)
		}
	}

	return rss, maps, sc.Err()
}

func logScaleMemory(t *testing.T, label string) {
	t.Helper()

	rss, maps, err := scaleMemory()
	if err != nil {
		t.Logf("%s memory: %v", label, err)
		return
	}

	t.Logf("%s memory: process RSS %d MiB, BPF maps %d MiB", label, rss>>20, maps>>20)
}

func sampleScaleMemory() func() (rss, maps uint64) {
	var (
		mu                sync.Mutex
		peakRSS, peakMaps uint64
		stop              = make(chan struct{})
		done              = make(chan struct{})
	)

	go func() {
		defer close(done)

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			if rss, maps, err := scaleMemory(); err == nil {
				mu.Lock()
				peakRSS = max(peakRSS, rss)
				peakMaps = max(peakMaps, maps)
				mu.Unlock()
			}

			select {
			case <-stop:
				return
			case <-ticker.C:
			}
		}
	}()

	return func() (uint64, uint64) {
		close(stop)
		<-done

		mu.Lock()
		defer mu.Unlock()

		return peakRSS, peakMaps
	}
}

func scaleIMSI(i int) string {
	return fmt.Sprintf("%015d", scaleStartIMSI+i)
}

func provisionScaleSubscribers(ctx context.Context, t *testing.T) {
	t.Helper()

	c := scaleProvisioningClient(ctx, t)
	start := time.Now()

	var g errgroup.Group

	g.SetLimit(scaleProvisionWorkers)

	for i := range scaleSubscribers {
		g.Go(func() error {
			spec := scenarios.DefaultSubscriberWith(scaleIMSI(i), "")

			return c.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
				Imsi:           spec.IMSI,
				Key:            spec.Key,
				SequenceNumber: spec.SequenceNumber,
				ProfileName:    spec.ProfileName,
				OPc:            spec.OPc,
			})
		})
	}

	if err := g.Wait(); err != nil {
		t.Fatalf("provision %d subscribers: %v", scaleSubscribers, err)
	}

	elapsed := time.Since(start)
	t.Logf("provisioned %d subscribers in %s (%.0f/s)", scaleSubscribers, elapsed.Round(time.Millisecond), float64(scaleSubscribers)/elapsed.Seconds())

	t.Cleanup(func() {
		c := scaleProvisioningClient(ctx, t)

		var g errgroup.Group

		g.SetLimit(scaleProvisionWorkers)

		for i := range scaleSubscribers {
			g.Go(func() error {
				return c.DeleteSubscriber(ctx, &client.DeleteSubscriberOptions{ID: scaleIMSI(i)})
			})
		}

		if err := g.Wait(); err != nil {
			t.Logf("cleanup: delete scale subscribers: %v", err)
		}
	})
}

func scaleProvisioningClient(ctx context.Context, t *testing.T) *client.Client {
	t.Helper()

	c, err := client.New(&client.Config{BaseURL: APIAddress()})
	if err != nil {
		t.Fatalf("ella client: %v", err)
	}

	if err := c.Login(ctx, &client.LoginOptions{Email: "admin@ellanetworks.com", Password: "admin"}); err != nil {
		t.Fatalf("login: %v", err)
	}

	return c
}

func addScaleRadioAddresses(ctx context.Context, t *testing.T, env *testerEnv) []string {
	t.Helper()

	addrs := make([]string, scaleRadios)

	for i := range scaleRadios {
		addrs[i] = fmt.Sprintf("10.3.0.%d", 100+i)

		argv := []string{"ip", "addr", "replace", addrs[i] + "/24", "dev", "n3"}
		if _, err := env.dc.Exec(ctx, env.TesterContainer, argv, false, 30*time.Second, nil); err != nil {
			t.Fatalf("add radio address %s: %v", addrs[i], err)
		}
	}

	return addrs
}
