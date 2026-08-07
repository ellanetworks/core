// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/spf13/pflag"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "interworking/transfer_5gs_to_eps",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run:       runTransfer5GSToEPS,
		Fixture:   fixture,
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "interworking/transfer_eps_to_5gs",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run:       runTransferEPSTo5GS,
		Fixture:   fixture,
	})
}

func runTransfer5GSToEPS(ctx context.Context, env scenarios.Env, _ any) error {
	fiveGSAddress, err := establishOn5GS(ctx, env)
	if err != nil {
		return err
	}

	epsAddress, err := moveToEPS(ctx, env)
	if err != nil {
		return err
	}

	if epsAddress != fiveGSAddress {
		return fmt.Errorf("UE address after the move to EPS = %s, want the %s it held on 5GS: the session was re-established, not moved",
			epsAddress, fiveGSAddress)
	}

	return nil
}

func runTransferEPSTo5GS(ctx context.Context, env scenarios.Env, _ any) error {
	epsAddress, err := establishOnEPS(ctx, env)
	if err != nil {
		return err
	}

	fiveGSAddress, err := moveTo5GS(ctx, env)
	if err != nil {
		return err
	}

	if fiveGSAddress != epsAddress {
		return fmt.Errorf("UE address after the move to 5GS = %s, want the %s it held on EPS: the session was re-established, not moved",
			fiveGSAddress, epsAddress)
	}

	return nil
}

func establishOn5GS(ctx context.Context, env scenarios.Env) (string, error) {
	gNodeB, err := startGNB(env)
	if err != nil {
		return "", err
	}
	defer gNodeB.Close()

	newUE, err := newInterworkingUE(gNodeB)
	if err != nil {
		return "", err
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)
	gNodeB.AddUE(ranUENGAPID, newUE)

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: movedPDUSessionID,
		UE:           newUE,
	}); err != nil {
		return "", fmt.Errorf("initial registration over NR: %w", err)
	}

	address, err := probeOver5GS(ctx, env, gNodeB, newUE, ranUENGAPID)
	if err != nil {
		return "", err
	}

	return address, nil
}

func moveToEPS(ctx context.Context, env scenarios.Env) (string, error) {
	e, err := startENB(env)
	if err != nil {
		return "", err
	}
	defer func() { _ = e.Close() }()

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return "", err
	}

	epsUE := e.NewUE(interworkingIMSI, k, opc)
	epsUE.MoveSessionFromNR(movedPDUSessionID)

	res, err := e.Attach(epsUE, attachTimeout)
	if err != nil {
		return "", fmt.Errorf("attach over E-UTRAN with request type handover: %w", err)
	}

	if res.UEIPv4 == "" {
		return "", fmt.Errorf("the move to EPS assigned no IPv4 address")
	}

	if err := e.AddTunnel(&s1enb.TunnelOpts{
		UEIPv4:           res.UEIPv4 + "/16",
		UpfAddress:       res.UpfAddress,
		ULTEID:           res.ULTEID,
		DLTEID:           res.DLTEID,
		TunInterfaceName: enbTunIface,
	}); err != nil {
		return "", fmt.Errorf("add S1-U tunnel: %w", err)
	}

	time.Sleep(datapathSettle)

	if err := probe.Run(ctx, probe.ICMP, enbTunIface, env.PingDestination(), scenarios.DefaultProbePort, false); err != nil {
		return "", fmt.Errorf("ping over S1-U after the move to EPS: %w", err)
	}

	return res.UEIPv4, nil
}

func establishOnEPS(ctx context.Context, env scenarios.Env) (string, error) {
	e, err := startENB(env)
	if err != nil {
		return "", err
	}
	defer func() { _ = e.Close() }()

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return "", err
	}

	epsUE := e.NewUE(interworkingIMSI, k, opc)
	epsUE.AnnounceN1Mode(movedPDUSessionID)

	res, err := e.Attach(epsUE, attachTimeout)
	if err != nil {
		return "", fmt.Errorf("attach over E-UTRAN: %w", err)
	}

	if res.UEIPv4 == "" {
		return "", fmt.Errorf("attach assigned no IPv4 address")
	}

	if err := e.AddTunnel(&s1enb.TunnelOpts{
		UEIPv4:           res.UEIPv4 + "/16",
		UpfAddress:       res.UpfAddress,
		ULTEID:           res.ULTEID,
		DLTEID:           res.DLTEID,
		TunInterfaceName: enbTunIface,
	}); err != nil {
		return "", fmt.Errorf("add S1-U tunnel: %w", err)
	}

	time.Sleep(datapathSettle)

	if err := probe.Run(ctx, probe.ICMP, enbTunIface, env.PingDestination(), scenarios.DefaultProbePort, false); err != nil {
		return "", fmt.Errorf("ping over S1-U after attach: %w", err)
	}

	return res.UEIPv4, nil
}

func moveTo5GS(ctx context.Context, env scenarios.Env) (string, error) {
	gNodeB, err := startGNB(env)
	if err != nil {
		return "", err
	}
	defer gNodeB.Close()

	newUE, err := newInterworkingUE(gNodeB)
	if err != nil {
		return "", err
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)
	gNodeB.AddUE(ranUENGAPID, newUE)

	if err := newUE.SendRegistrationRequest(ranUENGAPID, uint8(fgs.RegistrationTypeInitial)); err != nil {
		return "", fmt.Errorf("registration request over NR: %w", err)
	}

	if _, err := newUE.WaitForNASGMMMessage(uint8(fgs.MsgRegistrationAccept), attachTimeout); err != nil {
		return "", fmt.Errorf("registration accept over NR: %w", err)
	}

	if err := newUE.MovePDUSessionFromEPC(
		gNodeB.GetAMFUENGAPID(ranUENGAPID), ranUENGAPID, movedPDUSessionID,
		scenarios.DefaultDNN,
		models.Snssai{Sst: scenarios.DefaultSST, Sd: scenarios.DefaultSD},
	); err != nil {
		return "", fmt.Errorf("request the existing PDU session over NR: %w", err)
	}

	if _, err := newUE.WaitForNASGSMMessage(uint8(fgs.MsgPDUSessionEstablishmentAccept), attachTimeout); err != nil {
		return "", fmt.Errorf("the anchor refused to move the session onto 5GS: %w", err)
	}

	return probeOver5GS(ctx, env, gNodeB, newUE, ranUENGAPID)
}

func probeOver5GS(ctx context.Context, env scenarios.Env, gNodeB *gnb.GnodeB, u *ue.UE, ranUENGAPID int64) (string, error) {
	uePDUSession, err := u.WaitForPDUSession(movedPDUSessionID, attachTimeout)
	if err != nil {
		return "", fmt.Errorf("waiting for the PDU session: %w", err)
	}

	session := u.GetPDUSession(movedPDUSessionID)
	if session.UEIP == "" {
		return "", fmt.Errorf("the PDU session carries no UE address")
	}

	gnbSession, err := gNodeB.WaitForPDUSession(ranUENGAPID, int64(movedPDUSessionID), attachTimeout)
	if err != nil {
		return "", fmt.Errorf("waiting for the gNB's PDU session resources: %w", err)
	}

	if _, err := gNodeB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             session.UEIP + "/16",
		UpfIP:            gnbSession.UpfAddress,
		TunInterfaceName: gnbTunIface,
		ULteid:           gnbSession.ULTeid,
		DLteid:           gnbSession.DLTeid,
		MTU:              uePDUSession.MTU,
		QFI:              session.QFI,
	}); err != nil {
		return "", fmt.Errorf("add N3 tunnel: %w", err)
	}

	time.Sleep(datapathSettle)

	if err := probe.Run(ctx, probe.ICMP, gnbTunIface, env.PingDestination(), scenarios.DefaultProbePort, false); err != nil {
		return "", fmt.Errorf("ping over N3: %w", err)
	}

	return session.UEIP, nil
}

func newInterworkingUE(gNodeB *gnb.GnodeB) (*ue.UE, error) {
	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   movedPDUSessionID,
		PDUSessionType: fgs.PDUSessionTypeIPv4,
		Msin:           interworkingIMSI[5:],
		K:              scenarios.DefaultKey,
		OpC:            scenarios.DefaultOPC,
		Amf:            scenarios.DefaultAMF,
		Sqn:            scenarios.DefaultSequenceNumber,
		Mcc:            scenarios.DefaultMCC,
		Mnc:            scenarios.DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.NullScheme,
			PublicKeyID:      "0",
		},
		RoutingIndicator: scenarios.DefaultRoutingIndicator,
		DNN:              scenarios.DefaultDNN,
		Sst:              scenarios.DefaultSST,
		Sd:               scenarios.DefaultSD,
		IMEISV:           scenarios.DefaultIMEISV,
		UeSecurityCapability: testutil.GetUESecurityCapability(&testutil.UeSecurityCapability{
			Integrity: testutil.IntegrityAlgorithms{Nia2: true},
			Ciphering: testutil.CipheringAlgorithms{Nea0: true, Nea2: true},
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("create UE: %w", err)
	}

	return newUE, nil
}
