// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtimetest

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/testenv"
	"github.com/ellanetworks/core/internal/upf"
	"github.com/ellanetworks/core/internal/upf/engine"
	"github.com/ellanetworks/core/internal/upf/engine/enginetest"
	"github.com/ellanetworks/core/pkg/runtime"
	"github.com/prometheus/client_golang/prometheus"
)

type Core struct {
	APIAddress string
	APIToken   string
	N2Address  string
	S1Address  string
	Client     *client.Client
	Maps       *enginetest.Maps
}

type Options struct {
	Address string
}

var configureLogging sync.Once

func Start(t testing.TB, opts Options) *Core {
	t.Helper()

	configureLogging.Do(func() {
		level := os.Getenv("ELLA_TEST_LOG_LEVEL")
		if level == "" {
			level = "error"
		}

		if err := logger.ConfigureLogging(level, "stdout", "", "stdout", ""); err != nil {
			t.Fatalf("configure logging: %v", err)
		}
	})

	if opts.Address == "" {
		opts.Address = "127.0.0.1"
	}

	dir := t.TempDir()

	cfg := config.Config{
		DB: config.DB{Path: filepath.Join(dir, "core.db")},
		Interfaces: config.Interfaces{
			N2:  config.N2Interface{Address: opts.Address},
			N3:  config.N3Interface{Name: "fake-n3", Address: opts.Address, AddressExplicit: true},
			N6:  config.N6Interface{Name: "fake-n6"},
			API: config.APIInterface{Address: opts.Address},
		},
		AddressSource: fakeAddresses{
			"fake-n3": {netip.MustParsePrefix(opts.Address + "/8")},
			"fake-n6": {netip.MustParsePrefix("10.6.0.2/24"), netip.MustParsePrefix("fd00:6::2/64")},
		},
	}

	maps := enginetest.NewMaps()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	ready := make(chan runtime.Endpoints, 1)

	go func() {
		done <- runtime.Start(ctx, runtime.RuntimeConfig{
			Config:   &cfg,
			Kernel:   FakeKernel{},
			Metrics:  prometheus.NewRegistry(),
			Embedded: true,
			RaftTimeouts: runtime.RaftTimeouts{
				Heartbeat:   200 * time.Millisecond,
				Election:    200 * time.Millisecond,
				LeaderLease: 100 * time.Millisecond,
			},
			OnReady: func(e runtime.Endpoints) { ready <- e },
			Datapath: func(ctx context.Context, p runtime.DatapathParams) (runtime.Datapath, error) {
				return newDatapath(ctx, p, maps)
			},
		})
	}()

	t.Cleanup(func() {
		cancel()

		select {
		case err := <-done:
			if err != nil {
				t.Errorf("core exited with error: %v", err)
			}
		case <-time.After(testenv.Timeout(30 * time.Second)):
			t.Errorf("core did not shut down within 30s")
		}
	})

	var ep runtime.Endpoints

	select {
	case ep = <-ready:
	case err := <-done:
		t.Fatalf("core exited during startup: %v", err)
	case <-time.After(testenv.Timeout(30 * time.Second)):
		t.Fatalf("core not ready within 30s")
	}

	apiAddress := "http://" + ep.API

	cl, err := client.New(&client.Config{BaseURL: apiAddress})
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	if err := cl.Initialize(ctx, &client.InitializeOptions{Email: "admin@ellanetworks.com", Password: "admin"}); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	tok, err := cl.CreateMyAPIToken(ctx, &client.CreateAPITokenOptions{Name: "runtimetest"})
	if err != nil {
		t.Fatalf("create API token: %v", err)
	}

	cl.SetToken(tok.Token)

	return &Core{
		APIAddress: apiAddress,
		APIToken:   tok.Token,
		N2Address:  ep.N2,
		S1Address:  ep.S1AP,
		Client:     cl,
		Maps:       maps,
	}
}

func newDatapath(ctx context.Context, p runtime.DatapathParams, maps *enginetest.Maps) (runtime.Datapath, error) {
	rm, err := engine.NewFteIDResourceManager(upf.FTEIDPool)
	if err != nil {
		return nil, err
	}

	eng, err := engine.NewSessionEngine(upf.PfcpAddress, upf.PfcpNodeID, p.N3IPv4, p.N3IPv6, p.AdvertisedN3IPv4, p.AdvertisedN3IPv6, nil, rm)
	if err != nil {
		return nil, err
	}

	eng.SetMaps(maps)

	if err := eng.InitializeFiltersFromDB(ctx, p.DB); err != nil {
		return nil, err
	}

	return runtime.NewEngineDatapath(eng), nil
}

type FakeKernel struct{}

func (FakeKernel) EnableIPForwarding(context.Context) error { return nil }

func (FakeKernel) IsIPForwardingEnabled(context.Context) (bool, error) { return true, nil }

func (FakeKernel) CreateRoute(context.Context, netip.Prefix, netip.Addr, int, kernel.NetworkInterface) error {
	return nil
}

func (FakeKernel) DeleteRoute(context.Context, netip.Prefix, netip.Addr, int, kernel.NetworkInterface) error {
	return nil
}

func (FakeKernel) ReplaceRoute(context.Context, netip.Prefix, netip.Addr, int, kernel.NetworkInterface) error {
	return nil
}

func (FakeKernel) ListManagedRoutes(context.Context, kernel.NetworkInterface) ([]kernel.ManagedRoute, error) {
	return nil, nil
}

func (FakeKernel) InterfaceExists(context.Context, kernel.NetworkInterface) (bool, error) {
	return true, nil
}

func (FakeKernel) RouteExists(context.Context, netip.Prefix, netip.Addr, int, kernel.NetworkInterface) (bool, error) {
	return false, nil
}

func (FakeKernel) EnsureGatewaysOnInterfaceInNeighTable(context.Context, kernel.NetworkInterface) error {
	return nil
}

type fakeAddresses map[string][]netip.Prefix

func (f fakeAddresses) IPs(name string) ([]string, error) {
	prefixes, ok := f[name]
	if !ok {
		return nil, config.ErrNoInterfaceIP
	}

	ips := make([]string, 0, len(prefixes))
	for _, p := range prefixes {
		ips = append(ips, p.Addr().String())
	}

	return ips, nil
}

func (f fakeAddresses) IP(name string, family config.AddressFamily) (string, error) {
	for _, p := range f[name] {
		if p.Addr().Is4() == (family == config.IPv4) {
			return p.Addr().String(), nil
		}
	}

	return "", config.ErrNoInterfaceIP
}

func (f fakeAddresses) Subnets(name string) []netip.Prefix {
	subnets := make([]netip.Prefix, 0, len(f[name]))
	for _, p := range f[name] {
		subnets = append(subnets, p.Masked())
	}

	return subnets
}

func (f fakeAddresses) LinkUp(name string) bool {
	_, ok := f[name]
	return ok
}
