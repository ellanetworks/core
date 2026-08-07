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

// runTransfer5GSToEPS establishes a PDU session over NR, then re-attaches over
// E-UTRAN with request type "handover", naming the same PDU session identity in
// the PCO. The anchor moves the session rather than establishing a new one, so
// the UE keeps its address and the data keeps flowing — over S1-U now
// (TS 23.502 §4.11.2.2).
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

// runTransferEPSTo5GS is the mirror: attach over E-UTRAN announcing N1 mode and
// a PDU session identity, then register over NR and ask for the session with
// request type "existing PDU session" (TS 23.502 §4.11.2.3).
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

// establishOn5GS registers over NR, brings up the session's user plane, and
// returns the UE address. The gNB is closed before returning: a
// single-registration UE leaves NR entirely, and the two RAN simulators share
// one S1-U/N3 address in this topology.
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

// moveToEPS attaches over E-UTRAN with request type "handover", which asks the
// anchor to move the session rather than establish one, and returns the address
// the network assigned.
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

// establishOnEPS attaches over E-UTRAN announcing N1 mode and a PDU session
// identity, so the connection the anchor establishes can later be moved to 5GS.
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

// moveTo5GS registers over NR and asks for the session with request type
// "existing PDU session", naming the S-NSSAI the UE learned in the PCO while it
// was on EPS.
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

	// The session is not established: it is the one the UE holds in EPC, named by
	// its identity and the slice the network gave it there.
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

// probeOver5GS brings up the N3 tunnel for the UE's session and checks traffic
// flows, returning the UE address.
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
