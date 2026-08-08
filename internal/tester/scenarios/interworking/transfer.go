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

// ueAddresses is what the session carries on whichever access is serving it. The
// point of the move is that these survive it, so both families are compared in
// dualstack: the IPv4 address and the IPv6 interface identifier are preserved
// independently (TS 23.502 §4.11.2.2/§4.11.2.3).
type ueAddresses struct {
	v4 string
	v6 string
}

func (a ueAddresses) sameAs(b ueAddresses) bool {
	return a.v4 == b.v4 && a.v6 == b.v6
}

func (a ueAddresses) String() string {
	switch {
	case a.v4 != "" && a.v6 != "":
		return a.v4 + "," + a.v6
	case a.v6 != "":
		return a.v6
	default:
		return a.v4
	}
}

func runTransfer5GSToEPS(ctx context.Context, env scenarios.Env, _ any) error {
	fiveGS, err := establishOn5GS(ctx, env)
	if err != nil {
		return err
	}

	eps, err := moveToEPS(ctx, env)
	if err != nil {
		return err
	}

	if !eps.sameAs(fiveGS) {
		return fmt.Errorf("UE address after the move to EPS = %s, want the %s it held on 5GS: the session was re-established, not moved",
			eps, fiveGS)
	}

	return nil
}

func runTransferEPSTo5GS(ctx context.Context, env scenarios.Env, _ any) error {
	eps, err := establishOnEPS(ctx, env)
	if err != nil {
		return err
	}

	fiveGS, err := moveTo5GS(ctx, env)
	if err != nil {
		return err
	}

	if !fiveGS.sameAs(eps) {
		return fmt.Errorf("UE address after the move to 5GS = %s, want the %s it held on EPS: the session was re-established, not moved",
			fiveGS, eps)
	}

	return nil
}

func establishOn5GS(ctx context.Context, env scenarios.Env) (ueAddresses, error) {
	gNodeB, err := startGNB(env)
	if err != nil {
		return ueAddresses{}, err
	}
	defer gNodeB.Close()

	newUE, err := newInterworkingUE(env, gNodeB)
	if err != nil {
		return ueAddresses{}, err
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)
	gNodeB.AddUE(ranUENGAPID, newUE)

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: movedPDUSessionID,
		UE:           newUE,
	}); err != nil {
		return ueAddresses{}, fmt.Errorf("initial registration over NR: %w", err)
	}

	return probeOver5GS(ctx, env, gNodeB, newUE, ranUENGAPID, "over N3")
}

func moveToEPS(ctx context.Context, env scenarios.Env) (ueAddresses, error) {
	e, err := startENB(env)
	if err != nil {
		return ueAddresses{}, err
	}

	defer func() { _ = e.Close() }()

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return ueAddresses{}, err
	}

	epsUE := e.NewUE(interworkingIMSI, k, opc)
	epsUE.RequestPDNType(env.PDUSessionType())
	epsUE.MoveSessionFromNR(movedPDUSessionID)

	res, err := e.Attach(epsUE, attachTimeout)
	if err != nil {
		return ueAddresses{}, fmt.Errorf("attach over E-UTRAN with request type handover: %w", err)
	}

	return probeOverEPS(ctx, env, e, res, "after the move to EPS")
}

func establishOnEPS(ctx context.Context, env scenarios.Env) (ueAddresses, error) {
	e, err := startENB(env)
	if err != nil {
		return ueAddresses{}, err
	}

	defer func() { _ = e.Close() }()

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return ueAddresses{}, err
	}

	epsUE := e.NewUE(interworkingIMSI, k, opc)
	epsUE.RequestPDNType(env.PDUSessionType())
	epsUE.AnnounceN1Mode(movedPDUSessionID)

	res, err := e.Attach(epsUE, attachTimeout)
	if err != nil {
		return ueAddresses{}, fmt.Errorf("attach over E-UTRAN: %w", err)
	}

	return probeOverEPS(ctx, env, e, res, "after attach")
}

func moveTo5GS(ctx context.Context, env scenarios.Env) (ueAddresses, error) {
	gNodeB, err := startGNB(env)
	if err != nil {
		return ueAddresses{}, err
	}
	defer gNodeB.Close()

	newUE, err := newInterworkingUE(env, gNodeB)
	if err != nil {
		return ueAddresses{}, err
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)
	gNodeB.AddUE(ranUENGAPID, newUE)

	if err := newUE.SendRegistrationRequest(ranUENGAPID, uint8(fgs.RegistrationTypeInitial)); err != nil {
		return ueAddresses{}, fmt.Errorf("registration request over NR: %w", err)
	}

	if _, err := newUE.WaitForNASGMMMessage(uint8(fgs.MsgRegistrationAccept), attachTimeout); err != nil {
		return ueAddresses{}, fmt.Errorf("registration accept over NR: %w", err)
	}

	if err := newUE.MovePDUSessionFromEPC(
		gNodeB.GetAMFUENGAPID(ranUENGAPID), ranUENGAPID, movedPDUSessionID,
		scenarios.DefaultDNN,
		models.Snssai{Sst: scenarios.DefaultSST, Sd: scenarios.DefaultSD},
	); err != nil {
		return ueAddresses{}, fmt.Errorf("request the existing PDU session over NR: %w", err)
	}

	if _, err := newUE.WaitForNASGSMMessage(uint8(fgs.MsgPDUSessionEstablishmentAccept), attachTimeout); err != nil {
		return ueAddresses{}, fmt.Errorf("the anchor refused to move the session onto 5GS: %w", err)
	}

	return probeOver5GS(ctx, env, gNodeB, newUE, ranUENGAPID, "over N3 after the move to 5GS")
}

func probeOverEPS(ctx context.Context, env scenarios.Env, e *s1enb.ENB, res *s1enb.AttachResult, stage string) (ueAddresses, error) {
	addrs, err := attachAddresses(env, res)
	if err != nil {
		return ueAddresses{}, err
	}

	opts := &s1enb.TunnelOpts{
		UpfAddress:       res.UpfAddress,
		ULTEID:           res.ULTEID,
		DLTEID:           res.DLTEID,
		TunInterfaceName: enbTunIface,
	}

	if addrs.v4 != "" {
		opts.UEIPv4 = addrs.v4 + ipv4TunPrefix
	}

	if addrs.v6 != "" {
		opts.UEIPv6 = addrs.v6 + ipv6TunPrefix
	}

	if err := e.AddTunnel(opts); err != nil {
		return ueAddresses{}, fmt.Errorf("add S1-U tunnel: %w", err)
	}

	if addrs.v6 != "" {
		// The PDN IID gives a link-local; the UPF's Router Advertisement promotes it
		// to the global address the ping needs.
		if err := s1enb.WaitForULAAddr(enbTunIface, scenarios.DefaultUEIPv6Pool, slaacTimeout); err != nil {
			return ueAddresses{}, fmt.Errorf("await the SLAAC global address on S1-U: %w", err)
		}
	}

	time.Sleep(datapathSettle)

	if err := probe.Run(ctx, probe.ICMP, enbTunIface, env.PingDestination(), scenarios.DefaultProbePort, wantsIPv6Probe(env)); err != nil {
		return ueAddresses{}, fmt.Errorf("ping over S1-U %s: %w", stage, err)
	}

	return addrs, nil
}

func probeOver5GS(ctx context.Context, env scenarios.Env, gNodeB *gnb.GnodeB, u *ue.UE, ranUENGAPID int64, stage string) (ueAddresses, error) {
	uePDUSession, err := u.WaitForPDUSession(movedPDUSessionID, attachTimeout)
	if err != nil {
		return ueAddresses{}, fmt.Errorf("waiting for the PDU session: %w", err)
	}

	session := u.GetPDUSession(movedPDUSessionID)

	addrs, err := sessionAddresses(env, session.UEIP, session.UEIPV6)
	if err != nil {
		return ueAddresses{}, err
	}

	gnbSession, err := gNodeB.WaitForPDUSession(ranUENGAPID, int64(movedPDUSessionID), attachTimeout)
	if err != nil {
		return ueAddresses{}, fmt.Errorf("waiting for the gNB's PDU session resources: %w", err)
	}

	tunnel := &gnb.NewTunnelOpts{
		UpfIP:            gnbSession.UpfAddress,
		TunInterfaceName: gnbTunIface,
		ULteid:           gnbSession.ULTeid,
		DLteid:           gnbSession.DLTeid,
		MTU:              uePDUSession.MTU,
		QFI:              session.QFI,
	}

	if addrs.v4 != "" {
		tunnel.UEIP = addrs.v4 + ipv4TunPrefix
	}

	if addrs.v6 != "" {
		tunnel.UEIPV6 = addrs.v6 + ipv6TunPrefix
	}

	if _, err := gNodeB.AddTunnel(tunnel); err != nil {
		return ueAddresses{}, fmt.Errorf("add N3 tunnel: %w", err)
	}

	if addrs.v6 != "" {
		if err := gnb.WaitForULAAddr(gnbTunIface, scenarios.DefaultUEIPv6Pool, slaacTimeout); err != nil {
			return ueAddresses{}, fmt.Errorf("await the SLAAC global address on N3: %w", err)
		}
	}

	time.Sleep(datapathSettle)

	if err := probe.Run(ctx, probe.ICMP, gnbTunIface, env.PingDestination(), scenarios.DefaultProbePort, wantsIPv6Probe(env)); err != nil {
		return ueAddresses{}, fmt.Errorf("ping %s: %w", stage, err)
	}

	return addrs, nil
}

func attachAddresses(env scenarios.Env, res *s1enb.AttachResult) (ueAddresses, error) {
	return sessionAddresses(env, res.UEIPv4, res.UEIPv6)
}

// sessionAddresses keeps only the families the deployment provisions, so a
// comparison across the move is not defeated by a family neither access carries.
func sessionAddresses(env scenarios.Env, v4, v6 string) (ueAddresses, error) {
	var addrs ueAddresses

	if env.HasIPv4() {
		if v4 == "" {
			return ueAddresses{}, fmt.Errorf("the session carries no IPv4 address")
		}

		addrs.v4 = v4
	}

	if env.HasIPv6() {
		if v6 == "" {
			return ueAddresses{}, fmt.Errorf("the session carries no IPv6 address")
		}

		addrs.v6 = v6
	}

	return addrs, nil
}

// The ping follows env.PingDestination(), which stays on IPv4 in dualstack.
func wantsIPv6Probe(env scenarios.Env) bool {
	return env.IPFamily() == scenarios.IPv6Only
}

func newInterworkingUE(env scenarios.Env, gNodeB *gnb.GnodeB) (*ue.UE, error) {
	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   movedPDUSessionID,
		PDUSessionType: fgs.PDUSessionType(env.PDUSessionType()),
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
