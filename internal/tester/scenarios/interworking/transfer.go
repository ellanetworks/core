// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:    "interworking/transfer_5gs_to_eps",
		Run:     func(ctx context.Context, env scenarios.Env, _ any) error { return run5GSToEPS(ctx, env) },
		Fixture: fixture,
	})
	scenarios.Register(scenarios.Scenario{
		Name:    "interworking/transfer_eps_to_5gs",
		Run:     func(ctx context.Context, env scenarios.Env, _ any) error { return runEPSTo5GS(ctx, env) },
		Fixture: fixture,
	})
}

// TS 23.502 §4.11.2.2 step 13.
func run5GSToEPS(ctx context.Context, env scenarios.Env) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	e, err := startENB(env)
	if err != nil {
		return err
	}

	defer func() { _ = e.Close() }()

	fiveGUE, err := newFiveGUE(gNodeB, fgs.RequestTypeInitialRequest)
	if err != nil {
		return err
	}

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: transferPDUSessionID,
		UE:           fiveGUE,
	}); err != nil {
		return fmt.Errorf("initial registration over 5GS: %w", err)
	}

	before := fiveGUE.GetPDUSession(transferPDUSessionID)
	if before.UEIP == "" {
		return fmt.Errorf("5GS establishment assigned no address")
	}

	if err := pingOverN3(ctx, gNodeB, fiveGUE, "iw5g0"); err != nil {
		return fmt.Errorf("before the move: %w", err)
	}

	logger.Logger.Info("established over 5GS", zap.String("ue-ip", before.UEIP))

	epsUE, err := newEPSUE(e)
	if err != nil {
		return err
	}

	epsUE.TransferPDUSession(transferPDUSessionID)

	res, err := e.Attach(epsUE, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach over EPS with request type handover: %w", err)
	}

	if res.UEIPv4 != before.UEIP {
		return fmt.Errorf("address after the move to EPS = %s, want the one held on 5GS, %s", res.UEIPv4, before.UEIP)
	}

	if err := pingOverS1U(ctx, e, res, "iw4g0"); err != nil {
		return fmt.Errorf("after the move to EPS: %w", err)
	}

	logger.Logger.Info("moved 5GS → EPS with the address preserved", zap.String("ue-ip", res.UEIPv4))

	return nil
}

// checkSNSSAIContainer verifies the S-NSSAI the network returned for the PDN
// connection: the value part of an S-NSSAI IE followed by the PLMN it relates to
// (TS 24.008 §10.5.6.3 container 001BH, TS 24.501 §9.11.2.8).
func checkSNSSAIContainer(content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("the network returned no S-NSSAI container, so the connection cannot be mapped to a slice in 5GS")
	}

	ie, err := (models.Snssai{Sst: scenarios.DefaultSST, Sd: scenarios.DefaultSD}).NAS()
	if err != nil {
		return err
	}

	value, err := ie.MarshalBinary()
	if err != nil {
		return err
	}

	want, err := nas.NewSNSSAIContainer(value, nas.PLMN{MCC: scenarios.DefaultMCC, MNC: scenarios.DefaultMNC})
	if err != nil {
		return err
	}

	if !bytes.Equal(content, want.Content) {
		return fmt.Errorf("S-NSSAI container = %x, want %x", content, want.Content)
	}

	return nil
}

// TS 23.502 §4.11.2.3 step 9.
func runEPSTo5GS(ctx context.Context, env scenarios.Env) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	e, err := startENB(env)
	if err != nil {
		return err
	}

	defer func() { _ = e.Close() }()

	epsUE, err := newEPSUE(e)
	if err != nil {
		return err
	}

	epsUE.AllocatePDUSessionID(transferPDUSessionID)

	res, err := e.Attach(epsUE, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach over EPS: %w", err)
	}

	if res.UEIPv4 == "" {
		return fmt.Errorf("EPS attach assigned no address")
	}

	if err := checkSNSSAIContainer(res.SNSSAIContainer); err != nil {
		return err
	}

	if err := pingOverS1U(ctx, e, res, "iw4g1"); err != nil {
		return fmt.Errorf("before the move: %w", err)
	}

	logger.Logger.Info("established over EPS", zap.String("ue-ip", res.UEIPv4))

	fiveGUE, err := newFiveGUE(gNodeB, fgs.RequestTypeExistingPDUSession)
	if err != nil {
		return err
	}

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: transferPDUSessionID,
		UE:           fiveGUE,
	}); err != nil {
		return fmt.Errorf("registration over 5GS with request type existing PDU session: %w", err)
	}

	after := fiveGUE.GetPDUSession(transferPDUSessionID)
	if after.UEIP == "" {
		return fmt.Errorf("the move to 5GS produced no session")
	}

	if after.UEIP != res.UEIPv4 {
		return fmt.Errorf("address after the move to 5GS = %s, want the one held on EPS, %s", after.UEIP, res.UEIPv4)
	}

	if err := pingOverN3(ctx, gNodeB, fiveGUE, "iw5g1"); err != nil {
		return fmt.Errorf("after the move to 5GS: %w", err)
	}

	logger.Logger.Info("moved EPS → 5GS with the address preserved", zap.String("ue-ip", after.UEIP))

	return nil
}

func pingOverN3(ctx context.Context, gNodeB *gnb.GnodeB, u *ue.UE, tunIface string) error {
	session := u.GetPDUSession(transferPDUSessionID)
	if session.UEIP == "" {
		return fmt.Errorf("UE holds no PDU session %d", transferPDUSessionID)
	}

	gnbSession, err := gNodeB.WaitForPDUSession(ranUENGAPID, int64(transferPDUSessionID), 5*time.Second)
	if err != nil {
		return fmt.Errorf("await gNB PDU session: %w", err)
	}

	if _, err := gNodeB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             session.UEIP + "/16",
		UpfIP:            gnbSession.UpfAddress,
		TunInterfaceName: tunIface,
		ULteid:           gnbSession.ULTeid,
		DLteid:           gnbSession.DLTeid,
		MTU:              session.MTU,
		QFI:              session.QFI,
	}); err != nil {
		return fmt.Errorf("create N3 tunnel %q: %w", tunIface, err)
	}

	defer func() { _ = gNodeB.CloseTunnel(gnbSession.DLTeid) }()

	// Let the UPF program the downlink endpoint before probing.
	time.Sleep(500 * time.Millisecond)

	if err := probe.Run(ctx, probe.ICMP, tunIface, scenarios.DefaultPingDestination, scenarios.DefaultProbePort, false); err != nil {
		return fmt.Errorf("ping via %s: %w", tunIface, err)
	}

	return nil
}

func pingOverS1U(ctx context.Context, e *s1enb.ENB, res *s1enb.AttachResult, tunIface string) error {
	if err := e.AddTunnel(&s1enb.TunnelOpts{
		UEIPv4:           res.UEIPv4 + "/16",
		UpfAddress:       res.UpfAddress,
		ULTEID:           res.ULTEID,
		DLTEID:           res.DLTEID,
		TunInterfaceName: tunIface,
	}); err != nil {
		return fmt.Errorf("create S1-U tunnel %q: %w", tunIface, err)
	}

	defer e.CloseTunnel(res.DLTEID)

	time.Sleep(500 * time.Millisecond)

	if err := probe.Run(ctx, probe.ICMP, tunIface, scenarios.DefaultPingDestination, scenarios.DefaultProbePort, false); err != nil {
		return fmt.Errorf("ping via %s: %w", tunIface, err)
	}

	return nil
}
