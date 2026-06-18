// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"bytes"
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

// dnChangeIMSI is the subscriber the data-network-change scenarios attach.
const dnChangeIMSI = "001017271246650"

// dnChangeNewDNS is the DNS server the dns-change scenario switches to.
const dnChangeNewDNS = "1.1.1.1"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/data-network-dns-change",
		BindFlags: bindDataNetworkChangeFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runDataNetworkDNSChange(ctx, env, params.(*dataNetworkChangeParams))
		},
		Fixture: dataNetworkChangeFixture,
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/data-network-mtu-change",
		BindFlags: bindDataNetworkChangeFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runDataNetworkReactivate(ctx, env, params.(*dataNetworkChangeParams), "MTU", &client.UpdateDataNetworkOptions{
				Name:     scenarios.DefaultDNN,
				DNS:      scenarios.DefaultDNS,
				IPv4Pool: scenarios.DefaultUEIPv4Pool,
				IPv6Pool: scenarios.DefaultUEIPv6Pool,
				Mtu:      1400,
			})
		},
		Fixture: dataNetworkChangeFixture,
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/data-network-pool-change",
		BindFlags: bindDataNetworkChangeFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runDataNetworkReactivate(ctx, env, params.(*dataNetworkChangeParams), "IP pool", &client.UpdateDataNetworkOptions{
				Name:     scenarios.DefaultDNN,
				DNS:      scenarios.DefaultDNS,
				IPv4Pool: "10.47.0.0/16",
				IPv6Pool: scenarios.DefaultUEIPv6Pool,
				Mtu:      scenarios.DefaultMTU,
			})
		},
		Fixture: dataNetworkChangeFixture,
	})
}

type dataNetworkChangeParams struct {
	EllaAPIAddress string
	EllaAPIToken   string
}

func bindDataNetworkChangeFlags(fs *pflag.FlagSet) any {
	p := &dataNetworkChangeParams{}
	fs.StringVar(&p.EllaAPIAddress, "ella-api-address", "", "Ella Core API address")
	fs.StringVar(&p.EllaAPIToken, "ella-api-token", "", "Ella Core API token")

	return p
}

func dataNetworkChangeFixture(_ scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(dnChangeIMSI, "")},
	}
}

// attachAndReconfigure starts an eNB, attaches the UE, lets it settle into
// EMM-REGISTERED, applies a data-network change via the API, and registers the
// restore. It returns the eNB, the UE, and the attach result. The caller drives
// the resulting MME signalling. e.Close runs via the returned cleanup.
func attachAndReconfigure(ctx context.Context, env scenarios.Env, p *dataNetworkChangeParams, label string, mutation *client.UpdateDataNetworkOptions) (*s1enb.ENB, *s1enb.UE, *s1enb.AttachResult, func(), error) {
	if p.EllaAPIAddress == "" || p.EllaAPIToken == "" {
		return nil, nil, nil, nil, fmt.Errorf("--ella-api-address and --ella-api-token are required")
	}

	cl, err := client.New(&client.Config{BaseURL: p.EllaAPIAddress})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create Ella client: %w", err)
	}

	cl.SetToken(p.EllaAPIToken)

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	e, err := startENB(env)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("start eNB: %w", err)
	}

	ue := e.NewUE(dnChangeIMSI, k, opc)

	attach, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		_ = e.Close()

		return nil, nil, nil, nil, fmt.Errorf("attach: %w", err)
	}

	// Attach returns once ATTACH COMPLETE is sent; let the MME finish processing
	// it and settle the UE into EMM-REGISTERED before the data network changes,
	// so the change reconciles against an established bearer rather than racing
	// the attach (a UE still attaching reads the new config at bearer setup).
	time.Sleep(2 * time.Second)

	logger.GnbLogger.Info("UE attached; reconfiguring data network", zap.String("change", label))

	if err := cl.UpdateDataNetwork(ctx, mutation); err != nil {
		_ = e.Close()

		return nil, nil, nil, nil, fmt.Errorf("update data network (%s): %w", label, err)
	}

	cleanup := func() {
		_ = cl.UpdateDataNetwork(context.Background(), &client.UpdateDataNetworkOptions{
			Name:     scenarios.DefaultDNN,
			DNS:      scenarios.DefaultDNS,
			IPv4Pool: scenarios.DefaultUEIPv4Pool,
			IPv6Pool: scenarios.DefaultUEIPv6Pool,
			Mtu:      scenarios.DefaultMTU,
		})
		_ = e.Close()
	}

	return e, ue, attach, cleanup, nil
}

// runDataNetworkDNSChange verifies that a DNS change is propagated to an active
// EPS bearer in place — a MODIFY EPS BEARER CONTEXT REQUEST carrying the new DNS
// in the Protocol Configuration Options, without re-establishing the bearer
// (TS 24.301 §6.4.2). This mirrors the 5G PDU Session Modification path for a
// DNS change (ue/data-network-dns-change).
func runDataNetworkDNSChange(ctx context.Context, env scenarios.Env, p *dataNetworkChangeParams) error {
	e, ue, attach, cleanup, err := attachAndReconfigure(ctx, env, p, "DNS", &client.UpdateDataNetworkOptions{
		Name:     scenarios.DefaultDNN,
		DNS:      dnChangeNewDNS,
		IPv4Pool: scenarios.DefaultUEIPv4Pool,
		IPv6Pool: scenarios.DefaultUEIPv6Pool,
		Mtu:      scenarios.DefaultMTU,
	})
	if err != nil {
		return err
	}

	defer cleanup()

	req, err := e.ModifyBearer(ue, attach.ENBUES1APID, 20*time.Second)
	if err != nil {
		return fmt.Errorf("await bearer modification (DNS): %w", err)
	}

	dnsServers, _, err := eps.ParseProtocolConfigurationOptions(req.ProtocolConfigurationOptions)
	if err != nil {
		return fmt.Errorf("parse modification PCO: %w", err)
	}

	want := netip.MustParseAddr(dnChangeNewDNS).As4()

	found := false

	for _, dns := range dnsServers {
		if bytes.Equal(dns, want[:]) {
			found = true

			break
		}
	}

	if !found {
		return fmt.Errorf("modification PCO did not carry the new DNS %s (servers: %v)", dnChangeNewDNS, dnsServers)
	}

	logger.GnbLogger.Info("DNS change delivered in place via bearer modification")

	return nil
}

// runDataNetworkReactivate verifies that an MTU or IP-pool change — which the UE
// cannot adopt in place — deactivates the active EPS bearer with ESM cause #39
// "reactivation requested" (TS 24.301 §6.4.4.2), after which the UE re-attaches
// with the new configuration. This mirrors the 5G release-with-#39 path for
// MTU/pool (ue/data-network-{mtu,pool}-change).
func runDataNetworkReactivate(ctx context.Context, env scenarios.Env, p *dataNetworkChangeParams, label string, mutation *client.UpdateDataNetworkOptions) error {
	e, ue, attach, cleanup, err := attachAndReconfigure(ctx, env, p, label, mutation)
	if err != nil {
		return err
	}

	defer cleanup()

	req, err := e.ReactivateBearer(ue, attach.ENBUES1APID, 20*time.Second)
	if err != nil {
		return fmt.Errorf("await bearer reactivation (%s): %w", label, err)
	}

	if req.ESMCause != eps.ESMCauseReactivationRequested {
		return fmt.Errorf("ESM cause = %d, want %d (reactivation requested)", req.ESMCause, eps.ESMCauseReactivationRequested)
	}

	logger.GnbLogger.Info("bearer deactivated with reactivation requested; re-attaching", zap.String("change", label))

	// Re-attach with a fresh security context (new EPS-AKA, NAS counts reset),
	// as a real UE does after a deactivation with reactivation requested.
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	reUE := e.NewUE(dnChangeIMSI, k, opc)

	res, err := e.Attach(reUE, 15*time.Second)
	if err != nil {
		return fmt.Errorf("re-attach after reactivation (%s): %w", label, err)
	}

	if res.UEIPv4 == "" {
		return fmt.Errorf("re-attach (%s) assigned no IPv4 address", label)
	}

	// Detach the re-attached UE before the deferred restore reconfigures the data
	// network, so the restore does not reactivate a live bearer mid-cleanup.
	if err := e.Detach(reUE, res.MMEUES1APID, res.ENBUES1APID, 10*time.Second); err != nil {
		return fmt.Errorf("detach after re-attach (%s): %w", label, err)
	}

	logger.GnbLogger.Info("data-network change scenario completed", zap.String("change", label))

	return nil
}
