// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const (
	multiPDNIMSI          = "001017271246616"
	multiPDNProfile       = "multi-pdn-profile"
	multiPDNEnterpriseDNN = "enterprise"
	multiPDNTun1          = "s1enbmp0"
	multiPDNTun2          = "s1enbmp1"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/connectivity_multi_pdn",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBMultiPDN,
		Fixture:   multiPDNFixture,
	})
}

// multiPDNFixture provisions a profile with two policies — the default APN
// (internet) and a second APN (enterprise, its own IP pool) — the 4G counterpart
// of the 5G gnb/connectivity_multi_pdu_session fixture. 4G has no S-NSSAI, so both
// policies sit on the default slice; the MME resolves them by APN.
func multiPDNFixture(_ scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Profiles: []scenarios.ProfileSpec{
			{Name: multiPDNProfile, UeAmbrUplink: "500 Mbps", UeAmbrDownlink: "500 Mbps"},
		},
		DataNetworks: []scenarios.DataNetworkSpec{
			{Name: multiPDNEnterpriseDNN, IPv4Pool: "10.46.0.0/16", DNS: "8.8.4.4", MTU: scenarios.DefaultMTU},
		},
		Policies: []scenarios.PolicySpec{
			{
				Name:                "multi-pdn-default",
				ProfileName:         multiPDNProfile,
				SliceName:           scenarios.DefaultSliceName,
				DataNetworkName:     scenarios.DefaultDNN,
				SessionAmbrUplink:   "100 Mbps",
				SessionAmbrDownlink: "100 Mbps",
				Var5qi:              9,
				Arp:                 15,
			},
			{
				Name:                "multi-pdn-enterprise",
				ProfileName:         multiPDNProfile,
				SliceName:           scenarios.DefaultSliceName,
				DataNetworkName:     multiPDNEnterpriseDNN,
				SessionAmbrUplink:   "30 Mbps",
				SessionAmbrDownlink: "60 Mbps",
				Var5qi:              7,
				Arp:                 15,
			},
		},
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(multiPDNIMSI, multiPDNProfile)},
	}
}

// runS1ENBMultiPDN attaches a UE (default APN), opens a second PDN connection to
// another APN, and verifies user-plane connectivity on both with distinct UE IPs
// — the 4G counterpart of gnb/connectivity_multi_pdu_session. It then disconnects
// the second PDN and detaches, leaving the UE on its first PDN until detach.
func runS1ENBMultiPDN(ctx context.Context, env scenarios.Env, _ any) error {
	s1mme, err := s1mmeAddress(env.FirstCore())
	if err != nil {
		return err
	}

	k, opc, err := defaultKeyAndOPc()
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
		Name: "Ella-Core-Tester-S1eNB", CoreS1MMEAddress: s1mme,
		ENBAddress: g.N2Address, ENBN3Address: g.N3Address, EnableDatapath: true,
	})
	if err != nil {
		return fmt.Errorf("start eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(multiPDNIMSI, k, opc)

	res, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if res.UEIPv4 == "" {
		return fmt.Errorf("attach assigned no IPv4 address")
	}

	if err := e.AddTunnel(&s1enb.TunnelOpts{
		UEIPv4: res.UEIPv4 + "/16", UpfAddress: res.UpfAddress,
		ULTEID: res.ULTEID, DLTEID: res.DLTEID, TunInterfaceName: multiPDNTun1,
	}); err != nil {
		return fmt.Errorf("add GTP tunnel (default APN): %w", err)
	}

	defer e.CloseTunnel(res.DLTEID)

	time.Sleep(500 * time.Millisecond)

	if err := pingVia(ctx, multiPDNTun1); err != nil {
		return fmt.Errorf("ping on default APN: %w", err)
	}

	logger.GnbLogger.Info("default APN connectivity verified; opening second PDN connection",
		zap.String("apn", multiPDNEnterpriseDNN))

	pdn, err := e.OpenPDN(ue, res.MMEUES1APID, res.ENBUES1APID, multiPDNEnterpriseDNN, eps.PDNTypeIPv4, 15*time.Second)
	if err != nil {
		return fmt.Errorf("open second PDN connection: %w", err)
	}

	if pdn.UEIPv4 == "" {
		return fmt.Errorf("second PDN connection assigned no IPv4 address")
	}

	if pdn.UEIPv4 == res.UEIPv4 {
		return fmt.Errorf("second PDN connection reused the first PDN's IPv4 address %s", pdn.UEIPv4)
	}

	if err := e.AddTunnel(&s1enb.TunnelOpts{
		UEIPv4: pdn.UEIPv4 + "/16", UpfAddress: pdn.UpfAddress,
		ULTEID: pdn.ULTEID, DLTEID: pdn.DLTEID, TunInterfaceName: multiPDNTun2,
	}); err != nil {
		return fmt.Errorf("add GTP tunnel (second APN): %w", err)
	}

	defer e.CloseTunnel(pdn.DLTEID)

	time.Sleep(500 * time.Millisecond)

	if err := pingVia(ctx, multiPDNTun2); err != nil {
		return fmt.Errorf("ping on second APN: %w", err)
	}

	logger.GnbLogger.Info("second PDN connectivity verified; disconnecting it",
		zap.String("apn", multiPDNEnterpriseDNN), zap.String("ue-ip", pdn.UEIPv4))

	if err := e.DisconnectPDN(ue, res.MMEUES1APID, res.ENBUES1APID, uint8(pdn.ERABID), 10*time.Second); err != nil {
		return fmt.Errorf("disconnect second PDN connection: %w", err)
	}

	// The default APN must still work after the second PDN is disconnected.
	if err := pingVia(ctx, multiPDNTun1); err != nil {
		return fmt.Errorf("ping on default APN after second-PDN disconnect: %w", err)
	}

	if err := e.Detach(ue, res.MMEUES1APID, res.ENBUES1APID, 10*time.Second); err != nil {
		return fmt.Errorf("detach: %w", err)
	}

	logger.GnbLogger.Info("multi-PDN connectivity scenario completed")

	return nil
}

// pingVia pings the default N6 destination through the given tunnel interface.
func pingVia(ctx context.Context, iface string) error {
	cmd := exec.CommandContext(ctx, "ping", "-I", iface, scenarios.DefaultPingDestination, "-c", "3", "-W", "2") // #nosec G204 -- fixed ping; interface and destination are test config
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ping %s via %s failed: %v\n%s", scenarios.DefaultPingDestination, iface, err, string(out))
	}

	return nil
}
