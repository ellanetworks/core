// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

const (
	ue2ueStartIMSI      = "001017271248001"
	ue2ueTunIfacePrefix = "s1enbtun"
)

type ue2ueParams struct {
	ExpectSuccess bool `json:"expectSuccess"`
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "s1enb/ue2ue",
		BindFlags: func(fs *pflag.FlagSet) any {
			var p ue2ueParams
			fs.BoolVar(&p.ExpectSuccess, "expect-success", true, "expect the UDP probe to succeed")

			return &p
		},
		Run:     runS1ENBUE2UE,
		Fixture: fixtureS1ENBUE2UE,
	})
}

func fixtureS1ENBUE2UE(_ scenarios.Env) scenarios.FixtureSpec {
	imsiA := ue2ueStartIMSI
	imsiB := nthIMSI(ue2ueStartIMSI, 1)

	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{
			scenarios.DefaultSubscriberWith(imsiA, ""),
			scenarios.DefaultSubscriberWith(imsiB, ""),
		},
		AssertUsageForIMSIs: []string{imsiA, imsiB},
	}
}

func runS1ENBUE2UE(ctx context.Context, env scenarios.Env, params any) error {
	var expectSuccess bool
	if p, ok := params.(*ue2ueParams); ok {
		expectSuccess = p.ExpectSuccess
	} else {
		expectSuccess = true
	}

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
		Name: s1enbName, CoreS1MMEAddress: s1mme,
		ENBAddress: g.N2Address, ENBN3Address: g.N3Address, EnableDatapath: true,
	})
	if err != nil {
		return fmt.Errorf("start eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	imsiA := ue2ueStartIMSI
	imsiB := nthIMSI(ue2ueStartIMSI, 1)
	tunA := fmt.Sprintf("%s%d", ue2ueTunIfacePrefix, 0)
	tunB := fmt.Sprintf("%s%d", ue2ueTunIfacePrefix, 1)

	resA, err := attachAndTunnelS1(e, imsiA, k, opc, tunA)
	if err != nil {
		return fmt.Errorf("UE-A attach: %w", err)
	}

	resB, err := attachAndTunnelS1(e, imsiB, k, opc, tunB)
	if err != nil {
		return fmt.Errorf("UE-B attach: %w", err)
	}

	if resB.UEIPv4 == "" {
		return fmt.Errorf("UE-B was not assigned an IPv4 address")
	}

	probeErr := probe.UE2UE(ctx, tunA, tunB, resB.UEIPv4, scenarios.DefaultProbePort)

	e.CloseTunnel(resA.DLTEID)
	e.CloseTunnel(resB.DLTEID)

	if err := e.Detach(resA.ue, resA.mmeUES1APID, resA.enbUES1APID, releaseTimeout); err != nil {
		return fmt.Errorf("UE-A detach: %w", err)
	}

	if err := e.Detach(resB.ue, resB.mmeUES1APID, resB.enbUES1APID, releaseTimeout); err != nil {
		return fmt.Errorf("UE-B detach: %w", err)
	}

	if expectSuccess && probeErr != nil {
		return fmt.Errorf("udp UE-A -> UE-B (%s): expected success but failed: %w", resB.UEIPv4, probeErr)
	}

	if !expectSuccess && probeErr == nil {
		return fmt.Errorf("udp UE-A -> UE-B (%s): expected failure but probe succeeded", resB.UEIPv4)
	}

	return nil
}

type s1AttachResult struct {
	ue          *s1enb.UE
	UEIPv4      string
	DLTEID      uint32
	mmeUES1APID int64
	enbUES1APID int64
}

func attachAndTunnelS1(e *s1enb.ENB, imsi string, k, opc [16]byte, tunIface string) (*s1AttachResult, error) {
	ue := e.NewUE(imsi, k, opc)

	res, err := e.Attach(ue, attachTimeout)
	if err != nil {
		return nil, fmt.Errorf("attach: %w", err)
	}

	if res.UEIPv4 == "" {
		return nil, fmt.Errorf("attach assigned no IPv4 address")
	}

	if err := e.AddTunnel(&s1enb.TunnelOpts{
		UEIPv4:           res.UEIPv4 + "/16",
		UpfAddress:       res.UpfAddress,
		ULTEID:           res.ULTEID,
		DLTEID:           res.DLTEID,
		TunInterfaceName: tunIface,
	}); err != nil {
		return nil, fmt.Errorf("add GTP tunnel: %w", err)
	}

	time.Sleep(500 * time.Millisecond)

	return &s1AttachResult{
		ue:          ue,
		UEIPv4:      res.UEIPv4,
		DLTEID:      res.DLTEID,
		mmeUES1APID: res.MMEUES1APID,
		enbUES1APID: res.ENBUES1APID,
	}, nil
}
