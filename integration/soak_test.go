// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

const (
	soakComposeDir  = "compose/ha-5g-multi-gnb/"
	soakComposeFile = "compose.yaml"
	soakWarmUp      = 2 * time.Minute
	soakSettle      = 30 * time.Second
)

var soakTrackedMetrics = []string{
	"go_goroutines",
	"go_threads",
	"process_resident_memory_bytes",
	"process_open_fds",
	"app_sessions",
	"app_registered_subscribers",
	"app_ip_addresses_allocated",
	"app_upf_sessions_pending_cleanup",
	"app_connected_radios",
}

type soakNode struct {
	name    string
	service string
	apiURL  string
	coreN2  string
	tester  string
	gnbN2   string
	gnbN3   string
	imsi    string
}

type soakSample struct {
	at        time.Time
	phase     string
	heapInuse float64
	heapAlloc float64
	metrics   map[string]float64
}

type soakSeries struct {
	mu      sync.Mutex
	samples [][]soakSample
}

func (s *soakSeries) add(node int, sample soakSample) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.samples[node] = append(s.samples[node], sample)
}

func (s *soakSeries) phase(node int, phase string, after time.Time) []soakSample {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []soakSample

	for _, sm := range s.samples[node] {
		if sm.phase == phase && !sm.at.Before(after) {
			out = append(out, sm)
		}
	}

	return out
}

func TestIntegrationSoakHA(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" || os.Getenv("ELLA_SOAK") == "" {
		t.Skip("set INTEGRATION=1 and ELLA_SOAK=1 to run the HA soak test")
	}

	duration := soakEnvDuration(t, "ELLA_SOAK_DURATION", 10*time.Minute)
	interval := soakEnvDuration(t, "ELLA_SOAK_SAMPLE_INTERVAL", 10*time.Second)
	uesPerNode := soakEnvInt(t, "ELLA_SOAK_UES_PER_NODE", 20)

	if duration < 2*soakWarmUp {
		t.Fatalf("ELLA_SOAK_DURATION must be at least %v", 2*soakWarmUp)
	}

	ctx, cancel := context.WithTimeout(context.Background(), duration+20*time.Minute)
	defer cancel()

	outDir := soakOutputDir(t)
	t.Logf("soak artifacts in %s", outDir)

	nodes := []soakNode{
		{name: "node1", service: "ella-core-1", apiURL: "http://10.100.0.11:5002", coreN2: "10.100.0.11:38412", tester: "ella-core-tester-1", gnbN2: "10.100.0.21", gnbN3: "10.3.0.21", imsi: "001010000100001"},
		{name: "node2", service: "ella-core-2", apiURL: "http://10.100.0.12:5002", coreN2: "10.100.0.12:38412", tester: "ella-core-tester-2", gnbN2: "10.100.0.22", gnbN3: "10.3.0.22", imsi: "001010000200001"},
		{name: "node3", service: "ella-core-3", apiURL: "http://10.100.0.13:5002", coreN2: "10.100.0.13:38412", tester: "ella-core-tester-3", gnbN2: "10.100.0.23", gnbN3: "10.3.0.23", imsi: "001010000300001"},
	}

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	services := make([]string, 0, len(nodes)+1)
	for _, n := range nodes {
		services = append(services, n.tester)
	}

	services = append(services, "router")

	adminToken, nodeClients, err := bringUpHA3GPPCluster(t, ctx, dc, soakComposeDir, bringUpHA3GPPClusterOpts{
		ExtraServices: services,
		LogLevel:      "info",
		OTLPEndpoint:  os.Getenv("ELLA_SOAK_OTLP_ENDPOINT"),
	})
	if err != nil {
		t.Fatalf("bring up cluster: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()

		for _, n := range nodes {
			logs, err := dc.ComposeLogs(cleanupCtx, soakComposeDir, n.service)
			if err != nil {
				t.Logf("collect %s logs: %v", n.service, err)
				continue
			}

			_ = os.WriteFile(filepath.Join(outDir, n.service+".log"), []byte(logs), 0o644)
		}

		dc.ComposeDownWithFile(cleanupCtx, soakComposeDir, soakComposeFile)
	})

	haClient, err := client.New(&client.Config{BaseURLs: []string{nodes[0].apiURL, nodes[1].apiURL, nodes[2].apiURL}})
	if err != nil {
		t.Fatalf("HA client: %v", err)
	}

	haClient.SetToken(adminToken)

	if err := configureNATAndRoute(ctx, nodeClients); err != nil {
		t.Fatalf("configure NAT and route: %v", err)
	}

	fx := fixture.New(t, ctx, haClient)
	fx.OperatorDefault()
	fx.Profile(fixture.DefaultProfileSpec())
	fx.Slice(fixture.DefaultSliceSpec())
	fx.DataNetwork(fixture.DefaultDataNetworkSpec())
	fx.Policy(fixture.DefaultPolicySpec())

	var subs scenarios.FixtureSpec

	for _, n := range nodes {
		for i := range uesPerNode {
			imsi, err := offsetIMSI15(n.imsi, i)
			if err != nil {
				t.Fatalf("IMSI: %v", err)
			}

			subs.Subscribers = append(subs.Subscribers, scenarios.SubscriberSpec{
				IMSI:           imsi,
				Key:            scenarios.DefaultKey,
				OPc:            scenarios.DefaultOPC,
				SequenceNumber: scenarios.DefaultSequenceNumber,
				ProfileName:    scenarios.DefaultProfileName,
			})
		}
	}

	fx.Apply(subs)

	sampler := &soakSampler{
		http:  &http.Client{Timeout: 20 * time.Second},
		token: adminToken,
		nodes: nodes,
	}

	series := &soakSeries{samples: make([][]soakSample, len(nodes))}

	time.Sleep(soakSettle)

	baseline := sampler.quiescent(ctx, t, series, "baseline", 3)
	sampler.dumpProfiles(ctx, t, outDir, "baseline")

	containers := make([]string, len(nodes))

	for i, n := range nodes {
		container, err := dc.ResolveComposeContainer(ctx, "ha-5g-multi-gnb", n.tester)
		if err != nil {
			t.Fatalf("resolve %s: %v", n.tester, err)
		}

		containers[i] = container
	}

	loadStart := time.Now()

	samplerCtx, stopSampler := context.WithCancel(ctx)
	samplerDone := make(chan struct{})

	go func() {
		defer close(samplerDone)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-samplerCtx.Done():
				return
			case <-ticker.C:
				sampler.sampleAll(samplerCtx, t, series, "load")
			}
		}
	}()

	churnCtx, stopChurn := context.WithCancel(ctx)
	churn := &soakChurn{}
	churnDone := make(chan struct{})

	go func() {
		defer close(churnDone)

		churn.run(churnCtx, haClient)
	}()

	var (
		wg      sync.WaitGroup
		errMu   sync.Mutex
		testErr []string

		probes, probesLost, probesLate atomic.Int64
	)

	for i, n := range nodes {
		container := containers[i]

		argv := []string{
			"core-tester", "run", "gnb/soak",
			"--ella-core-n2-address", n.coreN2,
			"--gnb", fmt.Sprintf("soak-%s,n2=%s,n3=%s", n.name, n.gnbN2, n.gnbN3),
			"--ues", strconv.Itoa(uesPerNode),
			"--start-imsi", n.imsi,
			"--duration", duration.String(),
		}

		wg.Go(func() {
			f, err := os.Create(filepath.Join(outDir, n.tester+".log"))
			if err != nil {
				errMu.Lock()

				testErr = append(testErr, fmt.Sprintf("%s: %v", n.tester, err))
				errMu.Unlock()

				return
			}

			defer func() { _ = f.Close() }()

			out, execErr := dc.Exec(ctx, container, argv, false, duration+5*time.Minute, f)

			for line := range strings.SplitSeq(out, "\n") {
				if strings.Contains(line, "soak cycle summary") || strings.Contains(line, "soak complete") {
					t.Logf("%s: %s", n.tester, soakTrimLog(line))
				}

				if _, summary, ok := strings.Cut(line, "soak complete\t"); ok {
					var c struct {
						Probes int64 `json:"probes"`
						Lost   int64 `json:"probes_lost"`
						Late   int64 `json:"probes_late"`
					}

					if err := json.Unmarshal([]byte(summary), &c); err == nil {
						probes.Add(c.Probes)
						probesLost.Add(c.Lost)
						probesLate.Add(c.Late)
					}
				}
			}

			if execErr != nil {
				errMu.Lock()

				testErr = append(testErr, fmt.Sprintf("%s: %v\n%s", n.tester, execErr, soakTail(out, 30)))
				errMu.Unlock()
			}
		})
	}

	wg.Wait()

	stopChurn()
	<-churnDone
	stopSampler()
	<-samplerDone

	if sent := probes.Load(); sent > 0 {
		t.Logf("user-plane probes: %d sent, %d lost (%.2f%%), %d late (%.2f%%)", sent,
			probesLost.Load(), 100*float64(probesLost.Load())/float64(sent), probesLate.Load(), 100*float64(probesLate.Load())/float64(sent))
	} else {
		t.Errorf("no user-plane probe summary from the testers")
	}

	t.Logf("API churn: %d subscriber create/update/delete rounds, %d errors", churn.rounds.Load(), churn.errors.Load())

	if len(testErr) > 0 {
		t.Errorf("%d of %d soak testers failed:\n%s", len(testErr), len(nodes), strings.Join(testErr, "\n\n"))
	}

	if n := churn.errors.Load(); n > 0 {
		t.Errorf("API churn hit %d errors, first: %v", n, churn.firstErr.Load())
	}

	if err := sampler.waitDrained(ctx, 2*time.Minute); err != nil {
		t.Errorf("core did not drain after the soak: %v", err)
	}

	time.Sleep(soakSettle)

	final := sampler.quiescent(ctx, t, series, "final", 3)
	sampler.dumpProfiles(ctx, t, outDir, "final")

	soakWriteCSV(t, outDir, nodes, series)

	leaderIdx, leader, err := findLeader(ctx, nodeClients)
	if err != nil {
		t.Errorf("find leader after soak: %v", err)
	} else {
		t.Logf("leader after soak: %s", nodes[leaderIdx].name)

		if state, err := leader.GetAutopilotState(ctx); err != nil {
			t.Errorf("autopilot state after soak: %v", err)
		} else if !state.Healthy || len(state.Servers) != len(nodes) {
			t.Errorf("autopilot after soak: healthy=%v servers=%d, want healthy with %d servers", state.Healthy, len(state.Servers), len(nodes))
		}

		if index, err := leaderAppliedIndex(ctx, leader); err != nil {
			t.Errorf("leader applied index after soak: %v", err)
		} else if err := waitForFollowerConvergence(ctx, nodeClients, index); err != nil {
			t.Errorf("followers did not converge after soak: %v", err)
		} else {
			t.Logf("raft applied index after soak: %d, followers converged", index)
		}
	}

	if err := waitForAllNodesReady(ctx, nodeClients); err != nil {
		t.Errorf("nodes not ready after soak: %v", err)
	}

	soakAssert(t, nodes, series, baseline, final, loadStart.Add(soakWarmUp))
}

func soakAssert(t *testing.T, nodes []soakNode, series *soakSeries, baseline, final []soakSample, warm time.Time) {
	t.Helper()

	const mib = 1 << 20

	for i, n := range nodes {
		b, f := baseline[i], final[i]

		load := series.phase(i, "load", warm)
		if len(load) < 8 {
			t.Errorf("%s: only %d load samples after warm-up; cannot judge trends", n.name, len(load))
			continue
		}

		q := len(load) / 4
		early, late := load[:q], load[len(load)-q:]

		heapEarly := soakMedian(early, func(s soakSample) float64 { return s.heapInuse })
		heapLate := soakMedian(late, func(s soakSample) float64 { return s.heapInuse })
		gorEarly := soakMedian(early, func(s soakSample) float64 { return s.metrics["go_goroutines"] })
		gorLate := soakMedian(late, func(s soakSample) float64 { return s.metrics["go_goroutines"] })
		rssEarly := soakMedian(early, func(s soakSample) float64 { return s.metrics["process_resident_memory_bytes"] })
		rssLate := soakMedian(late, func(s soakSample) float64 { return s.metrics["process_resident_memory_bytes"] })
		heapSlope := soakSlope(load, func(s soakSample) float64 { return s.heapInuse }) * 3600 / mib
		gorSlope := soakSlope(load, func(s soakSample) float64 { return s.metrics["go_goroutines"] }) * 3600

		t.Logf("%s: heap in use (after GC) %.1f MiB idle -> %.1f/%.1f MiB under load (early/late, %+.1f MiB/h) -> %.1f MiB idle",
			n.name, b.heapInuse/mib, heapEarly/mib, heapLate/mib, heapSlope, f.heapInuse/mib)
		t.Logf("%s: goroutines %.0f idle -> %.0f/%.0f under load (%+.0f/h) -> %.0f idle; RSS %.0f -> %.0f/%.0f -> %.0f MiB; fds %.0f -> %.0f; threads %.0f -> %.0f",
			n.name, b.metrics["go_goroutines"], gorEarly, gorLate, gorSlope, f.metrics["go_goroutines"],
			b.metrics["process_resident_memory_bytes"]/mib, rssEarly/mib, rssLate/mib, f.metrics["process_resident_memory_bytes"]/mib,
			b.metrics["process_open_fds"], f.metrics["process_open_fds"], b.metrics["go_threads"], f.metrics["go_threads"])

		if grow := heapLate - heapEarly; grow > max(16*mib, 0.15*heapEarly) {
			t.Errorf("%s: heap in use grew %.1f MiB under steady load (%.1f -> %.1f MiB)", n.name, grow/mib, heapEarly/mib, heapLate/mib)
		}

		if grow := gorLate - gorEarly; grow > max(25, 0.1*gorEarly) {
			t.Errorf("%s: goroutines grew by %.0f under steady load (%.0f -> %.0f)", n.name, grow, gorEarly, gorLate)
		}

		if grow := f.metrics["go_goroutines"] - b.metrics["go_goroutines"]; grow > 15 {
			t.Errorf("%s: %.0f more goroutines idle after the soak than before (%.0f -> %.0f); compare goroutine-*-%s.txt",
				n.name, grow, b.metrics["go_goroutines"], f.metrics["go_goroutines"], n.name)
		}

		if grow := f.heapInuse - b.heapInuse; grow > max(48*mib, b.heapInuse) {
			t.Errorf("%s: heap in use idle after the soak %.1f MiB above the baseline (%.1f -> %.1f MiB); compare heap-*-%s.pb.gz",
				n.name, grow/mib, b.heapInuse/mib, f.heapInuse/mib, n.name)
		}

		if grow := f.metrics["process_open_fds"] - b.metrics["process_open_fds"]; grow > 10 {
			t.Errorf("%s: %.0f more open file descriptors idle after the soak than before (%.0f -> %.0f)",
				n.name, grow, b.metrics["process_open_fds"], f.metrics["process_open_fds"])
		}

		for _, m := range []string{"app_sessions", "app_registered_subscribers", "app_ip_addresses_allocated", "app_upf_sessions_pending_cleanup"} {
			if v := f.metrics[m]; v != 0 {
				t.Errorf("%s: %s is %.0f after the soak, want 0", n.name, m, v)
			}
		}

		for k, v := range f.metrics {
			if !strings.HasPrefix(k, "app_upf_bpf_map_entries{") {
				continue
			}

			if base := b.metrics[k]; v != base {
				t.Logf("%s: %s %.0f -> %.0f", n.name, k, base, v)
			}
		}
	}
}

type soakSampler struct {
	http  *http.Client
	token string
	nodes []soakNode
}

func (s *soakSampler) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+s.token)

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}

	return body, nil
}

func (s *soakSampler) sample(ctx context.Context, node int, phase string) (soakSample, error) {
	n := s.nodes[node]

	heap, err := s.get(ctx, n.apiURL+"/api/v1/pprof/heap?gc=1&debug=1")
	if err != nil {
		return soakSample{}, err
	}

	metrics, err := s.get(ctx, n.apiURL+"/api/v1/metrics")
	if err != nil {
		return soakSample{}, err
	}

	sm := soakSample{at: time.Now(), phase: phase, metrics: soakParseMetrics(metrics)}

	for line := range strings.SplitSeq(string(heap), "\n") {
		if v, ok := strings.CutPrefix(line, "# HeapInuse = "); ok {
			sm.heapInuse, _ = strconv.ParseFloat(strings.TrimSpace(v), 64)
		}

		if v, ok := strings.CutPrefix(line, "# HeapAlloc = "); ok {
			sm.heapAlloc, _ = strconv.ParseFloat(strings.TrimSpace(v), 64)
		}
	}

	if sm.heapInuse == 0 {
		return soakSample{}, fmt.Errorf("%s: no HeapInuse in the heap profile", n.name)
	}

	return sm, nil
}

func (s *soakSampler) sampleAll(ctx context.Context, t *testing.T, series *soakSeries, phase string) []soakSample {
	out := make([]soakSample, len(s.nodes))

	var wg sync.WaitGroup

	for i := range s.nodes {
		wg.Go(func() {
			sm, err := s.sample(ctx, i, phase)
			if err != nil {
				if ctx.Err() == nil {
					t.Logf("sample %s: %v", s.nodes[i].name, err)
				}

				return
			}

			series.add(i, sm)
			out[i] = sm
		})
	}

	wg.Wait()

	return out
}

func (s *soakSampler) quiescent(ctx context.Context, t *testing.T, series *soakSeries, phase string, n int) []soakSample {
	t.Helper()

	best := make([]soakSample, len(s.nodes))

	for range n {
		for i, sm := range s.sampleAll(ctx, t, series, phase) {
			if sm.metrics == nil {
				continue
			}

			heap := sm.heapInuse
			if best[i].metrics != nil {
				heap = min(heap, best[i].heapInuse)
			}

			if best[i].metrics == nil || sm.metrics["go_goroutines"] < best[i].metrics["go_goroutines"] {
				best[i] = sm
			}

			best[i].heapInuse = heap
		}

		time.Sleep(5 * time.Second)
	}

	for i, b := range best {
		if b.metrics == nil {
			t.Fatalf("no %s sample from %s", phase, s.nodes[i].name)
		}
	}

	return best
}

func (s *soakSampler) waitDrained(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		var busy []string

		for _, n := range s.nodes {
			body, err := s.get(ctx, n.apiURL+"/api/v1/metrics")
			if err != nil {
				busy = append(busy, fmt.Sprintf("%s: %v", n.name, err))
				continue
			}

			m := soakParseMetrics(body)
			for _, k := range []string{"app_sessions", "app_registered_subscribers", "app_upf_sessions_pending_cleanup"} {
				if m[k] != 0 {
					busy = append(busy, fmt.Sprintf("%s %s=%.0f", n.name, k, m[k]))
				}
			}
		}

		if len(busy) == 0 {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("%s", strings.Join(busy, ", "))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (s *soakSampler) dumpProfiles(ctx context.Context, t *testing.T, dir, phase string) {
	t.Helper()

	for _, n := range s.nodes {
		for _, p := range []struct{ path, file string }{
			{"/api/v1/pprof/goroutine?debug=1", fmt.Sprintf("goroutine-%s-%s.txt", phase, n.name)},
			{"/api/v1/pprof/heap?gc=1", fmt.Sprintf("heap-%s-%s.pb.gz", phase, n.name)},
			{"/api/v1/metrics", fmt.Sprintf("metrics-%s-%s.txt", phase, n.name)},
		} {
			body, err := s.get(ctx, n.apiURL+p.path)
			if err != nil {
				t.Logf("dump %s: %v", p.file, err)
				continue
			}

			if err := os.WriteFile(filepath.Join(dir, p.file), body, 0o644); err != nil {
				t.Logf("write %s: %v", p.file, err)
			}
		}
	}
}

type soakChurn struct {
	rounds   atomic.Int64
	errors   atomic.Int64
	firstErr atomic.Value
}

func (c *soakChurn) fail(err error) {
	c.errors.Add(1)
	c.firstErr.CompareAndSwap(nil, err.Error())
}

func (c *soakChurn) run(ctx context.Context, cl *client.Client) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		imsi := fmt.Sprintf("00101999%07d", i%10000000)

		if err := cl.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
			Imsi:           imsi,
			Key:            scenarios.DefaultKey,
			OPc:            scenarios.DefaultOPC,
			SequenceNumber: scenarios.DefaultSequenceNumber,
			ProfileName:    scenarios.DefaultProfileName,
		}); err != nil {
			if ctx.Err() == nil {
				c.fail(fmt.Errorf("create %s: %w", imsi, err))
			}

			continue
		}

		if err := cl.UpdateSubscriber(ctx, imsi, &client.UpdateSubscriberOptions{
			ProfileName: scenarios.DefaultProfileName,
			Description: fmt.Sprintf("soak round %d", i),
		}); err != nil && ctx.Err() == nil {
			c.fail(fmt.Errorf("update %s: %w", imsi, err))
		}

		if _, err := cl.ListSubscribers(ctx, &client.ListSubscribersParams{Page: 1, PerPage: 25}); err != nil && ctx.Err() == nil {
			c.fail(fmt.Errorf("list subscribers: %w", err))
		}

		if err := cl.DeleteSubscriber(context.WithoutCancel(ctx), &client.DeleteSubscriberOptions{ID: imsi}); err != nil {
			c.fail(fmt.Errorf("delete %s: %w", imsi, err))
		}

		c.rounds.Add(1)
	}
}

func soakParseMetrics(body []byte) map[string]float64 {
	out := map[string]float64{}

	sc := bufio.NewScanner(strings.NewReader(string(body)))
	sc.Buffer(make([]byte, 1<<20), 1<<20)

	for sc.Scan() {
		line := sc.Text()
		if line == "" || line[0] == '#' {
			continue
		}

		i := strings.LastIndexByte(line, ' ')
		if i < 0 {
			continue
		}

		v, err := strconv.ParseFloat(line[i+1:], 64)
		if err != nil {
			continue
		}

		series := line[:i]
		name, _, _ := strings.Cut(series, "{")

		if strings.HasPrefix(name, "app_upf_bpf_map_entries") {
			out[series] = v
		}

		if slices.Contains(soakTrackedMetrics, name) {
			out[name] += v
		}
	}

	return out
}

func soakMedian(samples []soakSample, f func(soakSample) float64) float64 {
	vals := make([]float64, len(samples))
	for i, s := range samples {
		vals[i] = f(s)
	}

	slices.Sort(vals)

	return vals[len(vals)/2]
}

func soakSlope(samples []soakSample, f func(soakSample) float64) float64 {
	if len(samples) < 2 {
		return 0
	}

	t0 := samples[0].at

	var sx, sy, sxx, sxy float64

	for _, s := range samples {
		x := s.at.Sub(t0).Seconds()
		y := f(s)
		sx += x
		sy += y
		sxx += x * x
		sxy += x * y
	}

	n := float64(len(samples))

	d := n*sxx - sx*sx
	if d == 0 {
		return 0
	}

	return (n*sxy - sx*sy) / d
}

func soakWriteCSV(t *testing.T, dir string, nodes []soakNode, series *soakSeries) {
	t.Helper()

	f, err := os.Create(filepath.Join(dir, "samples.csv"))
	if err != nil {
		t.Logf("samples.csv: %v", err)
		return
	}

	defer func() { _ = f.Close() }()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := append([]string{"time", "node", "phase", "heap_inuse", "heap_alloc"}, soakTrackedMetrics...)
	_ = w.Write(header)

	series.mu.Lock()
	defer series.mu.Unlock()

	for i, n := range nodes {
		for _, s := range series.samples[i] {
			row := []string{
				s.at.Format(time.RFC3339), n.name, s.phase,
				strconv.FormatFloat(s.heapInuse, 'f', 0, 64), strconv.FormatFloat(s.heapAlloc, 'f', 0, 64),
			}

			for _, m := range soakTrackedMetrics {
				row = append(row, strconv.FormatFloat(s.metrics[m], 'f', -1, 64))
			}

			_ = w.Write(row)
		}
	}
}

func soakOutputDir(t *testing.T) string {
	t.Helper()

	root := os.Getenv("INTEGRATION_LOG_DIR")
	if root == "" {
		root = os.TempDir()
	}

	dir := filepath.Join(root, fmt.Sprintf("soak-%s", time.Now().Format("20060102-150405")))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}

	return dir
}

func soakEnvDuration(t *testing.T, key string, def time.Duration) time.Duration {
	t.Helper()

	v := os.Getenv(key)
	if v == "" {
		return def
	}

	d, err := time.ParseDuration(v)
	if err != nil {
		t.Fatalf("%s: %v", key, err)
	}

	return d
}

func soakEnvInt(t *testing.T, key string, def int) int {
	t.Helper()

	v := os.Getenv(key)
	if v == "" {
		return def
	}

	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		t.Fatalf("%s must be a positive integer", key)
	}

	return n
}

func soakTail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	return strings.Join(lines, "\n")
}

func soakTrimLog(line string) string {
	if i := strings.Index(line, "soak "); i >= 0 {
		return line[i:]
	}

	return line
}
