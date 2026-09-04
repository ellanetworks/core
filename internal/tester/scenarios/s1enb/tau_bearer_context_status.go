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
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const bearerStatusTun = "s1enbbs0"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/tau_bearer_context_status",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBTAUBearerContextStatus,
		Fixture:   multiPDNFixture,
	})
}

func runS1ENBTAUBearerContextStatus(ctx context.Context, env scenarios.Env, _ any) error {
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

	res, err := e.Attach(ue, attachTimeout)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	pdn, err := e.OpenPDN(ue, res.MMEUES1APID, res.ENBUES1APID, multiPDNEnterpriseDNN, uint8(eps.PDNTypeIPv4), 15*time.Second)
	if err != nil {
		return fmt.Errorf("open second PDN connection: %w", err)
	}

	kept, dropped := res.ERABID, pdn.ERABID

	if err := e.ReleaseContext(res.MMEUES1APID, res.ENBUES1APID, s1enb.CauseUserInactivity, releaseTimeout); err != nil {
		return fmt.Errorf("release the connection before the tracking area update: %w", err)
	}

	var reported nas.EPSBearerContextStatus

	reported.Active[kept] = true

	tau, err := e.TrackingAreaUpdateWithBearerStatus(ue, res.GUTI, &reported, attachTimeout)
	if err != nil {
		return fmt.Errorf("tracking area update reporting E-RAB %d inactive: %w", dropped, err)
	}

	if err := assertReportedBearerStatus(tau.BearerStatus, kept, dropped); err != nil {
		return err
	}

	sr, err := e.ServiceRequest(ue, tau.GUTI, releaseTimeout)
	if err != nil {
		return fmt.Errorf("service request for the surviving PDN connection: %w", err)
	}

	if err := e.AddTunnel(&s1enb.TunnelOpts{
		UEIPv4: res.UEIPv4 + "/16", UpfAddress: sr.UpfAddress,
		ULTEID: sr.ULTEID, DLTEID: sr.DLTEID, TunInterfaceName: bearerStatusTun,
	}); err != nil {
		return fmt.Errorf("add GTP tunnel for the surviving PDN connection: %w", err)
	}

	defer e.CloseTunnel(sr.DLTEID)

	time.Sleep(500 * time.Millisecond)

	if err := probe.Run(ctx, probe.ICMP, bearerStatusTun, scenarios.DefaultPingDestination, scenarios.DefaultProbePort, false); err != nil {
		return fmt.Errorf("ping on the surviving PDN connection after the tracking area update: %w", err)
	}

	return e.Detach(ue, sr.MMEUES1APID, sr.ENBUES1APID, releaseTimeout)
}

func assertReportedBearerStatus(status *nas.EPSBearerContextStatus, kept, dropped s1ap.ERABID) error {
	if status == nil {
		return fmt.Errorf("the TRACKING AREA UPDATE ACCEPT carried no EPS bearer context status, so the UE cannot tell which bearers the MME kept (TS 24.301 §5.5.3.2.4)")
	}

	if !status.Active[kept] {
		return fmt.Errorf("EPS bearer context status reports EBI %d inactive, want it active: the UE reported it active", kept)
	}

	if status.Active[dropped] {
		return fmt.Errorf("EPS bearer context status reports EBI %d active, want it inactive: the UE reported it inactive and the MME must deactivate it locally (TS 24.301 §5.5.3.2.4)", dropped)
	}

	return nil
}
