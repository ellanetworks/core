// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/ellanetworks/core/nas/eps"
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

type sessionFacts struct {
	addrs          ueAddresses
	anchorAddr     string
	anchorTEID     uint32
	observedSource string
}

const continuitySrcPort = 50111

func assertContinuity(before, after sessionFacts) error {
	if !after.addrs.sameAs(before.addrs) {
		return fmt.Errorf("UE address = %s, want the %s it held before the move", after.addrs, before.addrs)
	}

	if after.anchorTEID != before.anchorTEID || after.anchorAddr != before.anchorAddr {
		return fmt.Errorf("anchor uplink endpoint = %s/%#x, want %s/%#x: the session was re-established, not moved",
			after.anchorAddr, after.anchorTEID, before.anchorAddr, before.anchorTEID)
	}

	if after.observedSource != before.observedSource {
		return fmt.Errorf("the data network saw the same flow arrive from %s, want %s: the user plane was rebuilt, not re-pointed",
			after.observedSource, before.observedSource)
	}

	return nil
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

	if err := assertContinuity(fiveGS, eps); err != nil {
		return err
	}

	return assertRegisteredOn(ctx, env, "4G")
}

func runTransferEPSTo5GS(ctx context.Context, env scenarios.Env, _ any) error {
	e, err := startENB(env)
	if err != nil {
		return err
	}

	defer func() { _ = e.Close() }()

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	epsUE := e.NewUE(interworkingIMSI, k, opc)
	epsUE.RequestPDNType(uint8(eps.PDNTypeIPv4v6))
	epsUE.AnnounceN1Mode(movedPDUSessionID)

	res, err := e.Attach(epsUE, attachTimeout)
	if err != nil {
		return fmt.Errorf("attach over E-UTRAN: %w", err)
	}

	before, err := probeOverEPS(ctx, env, e, res, "after attach")
	if err != nil {
		return err
	}

	if res.GUTI == nil || res.GUTI.GUTI == nil {
		return errors.New("the attach accept assigned no GUTI to map into a registration")
	}

	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	newUE, err := newInterworkingUE(gNodeB, false)
	if err != nil {
		return err
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	if err := arriveOn5GSFromEPS(gNodeB, epsUE, newUE, *res.GUTI.GUTI, ranUENGAPID, arriveAndResumeUserPlane); err != nil {
		return err
	}

	session, ok := gNodeB.PDUSession(ranUENGAPID, movedPDUSessionID)
	if !ok {
		return errors.New("the gNB holds no PDU session after the inter-system change, so the user plane was not re-established")
	}

	after, err := probeOver5GS(ctx, env, gNodeB, session, "over N3 after the move to 5GS")
	if err != nil {
		return err
	}

	if err := assertContinuity(before, after); err != nil {
		return err
	}

	return assertRegisteredOn(ctx, env, "5G")
}

func establishOn5GS(ctx context.Context, env scenarios.Env) (sessionFacts, error) {
	gNodeB, err := startGNB(env)
	if err != nil {
		return sessionFacts{}, err
	}
	defer gNodeB.Close()

	newUE, err := newInterworkingUE(gNodeB, true)
	if err != nil {
		return sessionFacts{}, err
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)
	gNodeB.AddUE(ranUENGAPID, newUE)

	registration, err := gNodeB.Register(newUE, ranUENGAPID, movedPDUSessionID, registrationTimeout)
	if err != nil {
		return sessionFacts{}, fmt.Errorf("initial registration over NR: %w", err)
	}

	return probeOver5GS(ctx, env, gNodeB, registration.Session, "over N3")
}

func moveToEPS(ctx context.Context, env scenarios.Env) (sessionFacts, error) {
	e, err := startENB(env)
	if err != nil {
		return sessionFacts{}, err
	}

	defer func() { _ = e.Close() }()

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return sessionFacts{}, err
	}

	epsUE := e.NewUE(interworkingIMSI, k, opc)
	epsUE.RequestPDNType(uint8(eps.PDNTypeIPv4v6))
	epsUE.MoveSessionFromNR(movedPDUSessionID)

	res, err := e.Attach(epsUE, attachTimeout)
	if err != nil {
		return sessionFacts{}, fmt.Errorf("attach over E-UTRAN with request type handover: %w", err)
	}

	return probeOverEPS(ctx, env, e, res, "after the move to EPS")
}

func probeOverEPS(ctx context.Context, env scenarios.Env, e *s1enb.ENB, res *s1enb.AttachResult, stage string) (sessionFacts, error) {
	addrs, err := attachAddresses(env, res)
	if err != nil {
		return sessionFacts{}, err
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

	if env.HasIPv6() {
		opts.UEIPv6 = addrs.v6 + ipv6TunPrefix
	}

	if err := e.AddTunnel(opts); err != nil {
		return sessionFacts{}, fmt.Errorf("add S1-U tunnel: %w", err)
	}

	if env.HasIPv6() {
		if err := s1enb.WaitForULAAddr(enbTunIface, scenarios.DefaultUEIPv6Pool, slaacTimeout); err != nil {
			return sessionFacts{}, fmt.Errorf("await the SLAAC global address on S1-U: %w", err)
		}
	}

	time.Sleep(datapathSettle)

	if err := probe.Run(ctx, probe.ICMP, enbTunIface, env.PingDestination(), scenarios.DefaultProbePort, wantsIPv6Probe(env)); err != nil {
		return sessionFacts{}, fmt.Errorf("ping over S1-U %s: %w", stage, err)
	}

	return sessionFactsFor(ctx, env, addrs, enbTunIface, res.UpfAddress, res.ULTEID, "S1-U "+stage)
}

func sessionFactsFor(ctx context.Context, env scenarios.Env, addrs ueAddresses, tun, anchorAddr string, anchorTEID uint32, stage string) (sessionFacts, error) {
	if v6 := env.PingDestinationV6(); v6 != "" && v6 != env.PingDestination() {
		if err := probe.Run(ctx, probe.ICMP, tun, v6, scenarios.DefaultProbePort, true); err != nil {
			return sessionFacts{}, fmt.Errorf("ping over IPv6 %s: %w", stage, err)
		}
	}

	src, err := assignedSource(env, tun, addrs)
	if err != nil {
		return sessionFacts{}, fmt.Errorf("determine the source address %s: %w", stage, err)
	}

	observed, err := probe.ObservedSource(ctx, tun, src, env.PingDestination(), scenarios.DefaultProbePort, continuitySrcPort)
	if err != nil {
		return sessionFacts{}, fmt.Errorf("read the observed source %s: %w", stage, err)
	}

	return sessionFacts{
		addrs:          addrs,
		anchorAddr:     anchorAddr,
		anchorTEID:     anchorTEID,
		observedSource: observed,
	}, nil
}

func probeOver5GS(ctx context.Context, env scenarios.Env, gNodeB *gnb.GnodeB, session gnb.PDUSessionResult, stage string) (sessionFacts, error) {
	addrs, err := sessionAddresses(env, session.UEIPv4, session.UEIPv6)
	if err != nil {
		return sessionFacts{}, err
	}

	tunnel := &gnb.TunnelOpts{
		UpfAddress:       session.UpfAddress,
		TunInterfaceName: gnbTunIface,
		ULTEID:           session.ULTEID,
		DLTEID:           session.DLTEID,
		MTU:              session.MTU,
		QFI:              session.QFI,
	}

	if addrs.v4 != "" {
		tunnel.UEIPv4 = addrs.v4 + ipv4TunPrefix
	}

	if env.HasIPv6() {
		tunnel.UEIPv6 = addrs.v6 + ipv6TunPrefix
	}

	if err := gNodeB.AddTunnel(tunnel); err != nil {
		return sessionFacts{}, fmt.Errorf("add N3 tunnel: %w", err)
	}

	if env.HasIPv6() {
		if err := gnb.WaitForULAAddr(gnbTunIface, scenarios.DefaultUEIPv6Pool, slaacTimeout); err != nil {
			return sessionFacts{}, fmt.Errorf("await the SLAAC global address on N3: %w", err)
		}
	}

	time.Sleep(datapathSettle)

	if err := probe.Run(ctx, probe.ICMP, gnbTunIface, env.PingDestination(), scenarios.DefaultProbePort, wantsIPv6Probe(env)); err != nil {
		return sessionFacts{}, fmt.Errorf("ping %s: %w", stage, err)
	}

	return sessionFactsFor(ctx, env, addrs, gnbTunIface, session.UpfAddress, session.ULTEID, "N3 "+stage)
}

func attachAddresses(env scenarios.Env, res *s1enb.AttachResult) (ueAddresses, error) {
	return sessionAddresses(env, res.UEIPv4, res.UEIPv6)
}

func sessionAddresses(env scenarios.Env, v4, v6 string) (ueAddresses, error) {
	var addrs ueAddresses

	if env.HasIPv4() {
		if v4 == "" {
			return ueAddresses{}, fmt.Errorf("the session carries no IPv4 address")
		}

		addrs.v4 = v4
	}

	if v6 == "" {
		return ueAddresses{}, fmt.Errorf("the session carries no IPv6 interface identifier to prove continuity with")
	}

	addrs.v6 = v6

	return addrs, nil
}

func wantsIPv6Probe(env scenarios.Env) bool {
	return env.IPFamily() == scenarios.IPv6Only
}

func newInterworkingUE(gNodeB *gnb.GnodeB, autoSession bool) (*ue.UE, error) {
	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:           gNodeB,
		PDUSessionID:     movedPDUSessionID,
		PDUSessionType:   fgs.PDUSessionTypeIPv4v6,
		NoAutoPDUSession: !autoSession,
		Msin:             interworkingIMSI[5:],
		K:                scenarios.DefaultKey,
		OpC:              scenarios.DefaultOPC,
		Amf:              scenarios.DefaultAMF,
		Sqn:              scenarios.DefaultSequenceNumber,
		Mcc:              scenarios.DefaultMCC,
		Mnc:              scenarios.DefaultMNC,
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
