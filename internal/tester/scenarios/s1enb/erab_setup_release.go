// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const (
	erabProfile    = "s1enb-erab-profile"
	erabSecondDNN  = "erabims"
	erabSecondPool = "10.56.0.0/16"
	erabIMSI       = "001017271246623"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/erab_setup_release",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBERABSetupRelease,
		Fixture:   fixtureS1ENBERABSetupRelease,
	})
}

func fixtureS1ENBERABSetupRelease(_ scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Profiles: []scenarios.ProfileSpec{
			{Name: erabProfile, UeAmbrUplink: "300 Mbps", UeAmbrDownlink: "300 Mbps"},
		},
		DataNetworks: []scenarios.DataNetworkSpec{
			{Name: erabSecondDNN, IPv4Pool: erabSecondPool, DNS: scenarios.DefaultDNS, MTU: scenarios.DefaultMTU},
		},
		Policies: []scenarios.PolicySpec{
			{
				Name: "s1enb-erab-default", ProfileName: erabProfile, SliceName: scenarios.DefaultSliceName,
				DataNetworkName: scenarios.DefaultDNN, SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps",
				Var5qi: 9, Arp: 15,
			},
			{
				Name: "s1enb-erab-second", ProfileName: erabProfile, SliceName: scenarios.DefaultSliceName,
				DataNetworkName: erabSecondDNN, SessionAmbrUplink: "30 Mbps", SessionAmbrDownlink: "60 Mbps",
				Var5qi: 5, Arp: 15,
			},
		},
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(erabIMSI, erabProfile)},
	}
}

func runS1ENBERABSetupRelease(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(erabIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	res, err := e.Attach(ue, attachTimeout)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	pdn, err := e.OpenPDN(ue, res.MMEUES1APID, res.ENBUES1APID, erabSecondDNN, uint8(eps.PDNTypeIPv4), attachTimeout)
	if err != nil {
		return fmt.Errorf("open second PDN: %w", err)
	}

	if err := assertERABSetup(res, pdn); err != nil {
		return err
	}

	rel, err := e.DisconnectPDN(ue, res.MMEUES1APID, res.ENBUES1APID, uint8(pdn.ERABID), releaseTimeout)
	if err != nil {
		return fmt.Errorf("disconnect second PDN: %w", err)
	}

	return assertERABRelease(pdn, rel)
}

func assertERABSetup(res *s1enb.AttachResult, pdn *s1enb.PDNResult) error {
	if pdn.ERABID == res.ERABID {
		return fmt.Errorf("second PDN reused E-RAB ID %d; the default bearer already holds it", pdn.ERABID)
	}

	if pdn.ERABQCI != 5 {
		return fmt.Errorf("E-RAB Setup Request QCI = %d, want the policy's 5 (the NAS QoS said %d)", pdn.ERABQCI, pdn.QCI)
	}

	if pdn.ARP != 15 {
		return fmt.Errorf("E-RAB Setup Request ARP priority = %d, want 15", pdn.ARP)
	}

	const wantAmbr = 300 * mbpsToBps

	if pdn.UEAmbrDownlinkBps != wantAmbr || pdn.UEAmbrUplinkBps != wantAmbr {
		return fmt.Errorf("E-RAB Setup Request UE-AMBR = %d/%d bps, want the profile's %d/%d re-signalled to the eNB",
			pdn.UEAmbrDownlinkBps, pdn.UEAmbrUplinkBps, wantAmbr, wantAmbr)
	}

	if pdn.SessAmbrDownlinkBps != 60*mbpsToBps || pdn.SessAmbrUplinkBps != 30*mbpsToBps {
		return fmt.Errorf("second PDN Session-AMBR = %d/%d bps, want 60/30 Mbps",
			pdn.SessAmbrDownlinkBps, pdn.SessAmbrUplinkBps)
	}

	if pdn.APN != erabSecondDNN {
		return fmt.Errorf("second PDN APN = %q, want %q", pdn.APN, erabSecondDNN)
	}

	return nil
}

func assertERABRelease(pdn *s1enb.PDNResult, rel *s1enb.PDNReleaseResult) error {
	if rel.ERABID != pdn.ERABID {
		return fmt.Errorf("E-RAB Release Command released E-RAB %d, want the %d the PDN disconnect named", rel.ERABID, pdn.ERABID)
	}

	if rel.Cause.Group != s1ap.CauseGroupNAS || rel.Cause.Value != s1ap.CauseNASNormalRelease {
		return fmt.Errorf("E-RAB Release Command cause = group %d value %d, want NAS normal-release for a UE-requested PDN disconnect",
			rel.Cause.Group, rel.Cause.Value)
	}

	return nil
}
