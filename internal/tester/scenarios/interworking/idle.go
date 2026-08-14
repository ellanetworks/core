// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/spf13/pflag"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "interworking/idle_eps_to_5gs",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run:       runIdleEPSTo5GS,
		Fixture:   fixture,
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "interworking/idle_5gs_to_eps",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runIdle5GSToEPS(ctx, env, true)
		},
		Fixture: fixture,
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "interworking/idle_5gs_to_eps_returning_to_idle",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runIdle5GSToEPS(ctx, env, false)
		},
		Fixture: fixture,
	})
}

func runIdle5GSToEPS(ctx context.Context, env scenarios.Env, activeFlag bool) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	u, err := newInterworkingUE(gNodeB, true)
	if err != nil {
		return err
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)
	gNodeB.AddUE(ranUENGAPID, u)

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: movedPDUSessionID,
		UE:           u,
	}); err != nil {
		return fmt.Errorf("initial registration over NR: %w", err)
	}

	if err := provisionEPSNASAlgorithms(gNodeB, u, ranUENGAPID); err != nil {
		return err
	}

	before, err := probeOver5GS(ctx, env, gNodeB, u, mobilityRANUENGAPID, "over N3 before the idle move")
	if err != nil {
		return err
	}

	if err := goIdleOnNR(gNodeB, u); err != nil {
		return err
	}

	security, guti, err := idleMobilityMaterial(u)
	if err != nil {
		return err
	}

	e, err := startENBOnSecondaryN3(env)
	if err != nil {
		return err
	}

	defer func() { _ = e.Close() }()

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	epsUE := e.NewUE(interworkingIMSI, k, opc)

	var bearerStatus nas.EPSBearerContextStatus

	bearerStatus.Active[movedEPSBearerIdentity] = true

	res, err := e.TrackingAreaUpdateFrom5GS(epsUE, s1enb.IdleTrackingAreaUpdateOpts{
		GUTI:         guti,
		ActiveFlag:   activeFlag,
		BearerStatus: &bearerStatus,
		Security:     security,
	}, attachTimeout)
	if err != nil {
		return fmt.Errorf("tracking area update after the idle move to EPS: %w", err)
	}

	if err := assertAdoptedBearer(res); err != nil {
		return err
	}

	if !activeFlag {
		return assertSessionOn(ctx, env, "4G", before.addrs)
	}

	after, err := probeAfterHandover(ctx, env, e, handoverBearer{
		upfAddress: res.UpfAddress,
		ulTEID:     res.ULTEID,
		dlTEID:     res.DLTEID,
		mmeUEID:    res.MMEUES1APID,
		epsUE:      epsUE,
	}, before.addrs)
	if err != nil {
		return err
	}

	return assertContinuity(before, after)
}

func goIdleOnNR(gNodeB *gnb.GnodeB, u *ue.UE) error {
	var sessions [16]bool

	sessions[movedPDUSessionID] = true

	if err := procedure.UEContextRelease(&procedure.UEContextReleaseOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(mobilityRANUENGAPID),
		RANUENGAPID:   mobilityRANUENGAPID,
		GnodeB:        gNodeB,
		UE:            u,
		PDUSessionIDs: sessions,
	}); err != nil {
		return fmt.Errorf("release the NR connection before the inter-system change: %w", err)
	}

	return nil
}

func idleMobilityMaterial(u *ue.UE) (s1enb.IdleMobilityFrom5GS, eps.GUTI, error) {
	var none s1enb.IdleMobilityFrom5GS

	if u.UeSecurity.Guti == nil || u.UeSecurity.Guti.GUTI == nil {
		return none, eps.GUTI{}, errors.New("the UE holds no 5G-GUTI to map into a tracking area update")
	}

	if u.UeSecurity.EPSNASAlgorithms == nil {
		return none, eps.GUTI{}, errors.New("the AMF signalled no EPS NAS algorithms, so no mapped context can be derived")
	}

	return s1enb.IdleMobilityFrom5GS{
		KAMF:             u.UeSecurity.Kamf,
		KNASInt:          u.UeSecurity.KnasInt,
		NIA:              u.UeSecurity.IntegrityAlg,
		UplinkNASCount:   u.UeSecurity.ULCount,
		DownlinkNASCount: u.UeSecurity.DLRecv.LastAccepted(),
		EPSCiphering:     uint8(u.UeSecurity.EPSNASAlgorithms.Ciphering),
		EPSIntegrity:     uint8(u.UeSecurity.EPSNASAlgorithms.Integrity),
		EKSI:             uint8(u.UeSecurity.NgKsi.Ksi),
	}, etsi.MapGUTI5GToEPS(*u.UeSecurity.Guti.GUTI), nil
}

func assertAdoptedBearer(res *s1enb.AttachResult) error {
	if res.BearerStatus == nil {
		return errors.New("the tracking area update accept carried no EPS bearer context status, so the UE cannot tell which session survived")
	}

	if !res.BearerStatus.Active[movedEPSBearerIdentity] {
		return fmt.Errorf("EPS bearer context status = %v, want EBI %d active: the PDU session did not become a PDN connection",
			res.BearerStatus.Active, movedEPSBearerIdentity)
	}

	if res.GUTI == nil {
		return errors.New("the tracking area update accept reallocated no GUTI")
	}

	return nil
}

const (
	sessionSettle = 10 * time.Second
	idleCiphering = uint8(nas.CipheringAES)
	idleIntegrity = uint8(nas.IntegrityAES)
)

func runIdleEPSTo5GS(ctx context.Context, env scenarios.Env, _ any) error {
	e, err := startENBOnSecondaryN3(env)
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

	before, err := probeOverEPS(ctx, env, e, res, "before the idle move to 5GS")
	if err != nil {
		return err
	}

	if err := assertSessionOn(ctx, env, "4G", before.addrs); err != nil {
		return err
	}

	if res.GUTI == nil || res.GUTI.GUTI == nil {
		return errors.New("the attach accept assigned no GUTI to map into a registration")
	}

	var bearerStatus nas.EPSBearerContextStatus

	bearerStatus.Active[movedEPSBearerIdentity] = true

	container, err := epsUE.BuildTrackingAreaUpdateForContainer(*res.GUTI.GUTI, &bearerStatus)
	if err != nil {
		return err
	}

	mapped := epsUE.MappedContextForIdleMobility()

	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	u, err := newInterworkingUE(gNodeB, false)
	if err != nil {
		return err
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)
	gNodeB.AddUE(ranUENGAPID, u)

	var sessions [16]bool

	sessions[movedPDUSessionID] = true

	if err := u.SendIdleMobilityRegistration(ue.IdleRegistrationOpts{
		RANUENGAPID:            ranUENGAPID,
		MappedGUTI:             fgs.GUTIIdentity(etsi.MapGUTIEPSTo5G(*res.GUTI.GUTI)),
		EPSNASMessageContainer: container,
		PDUSessionStatus:       &sessions,
		Mapped: ue.MappedFromEPSIdle{
			KASME:          mapped.KASME,
			UplinkNASCount: mapped.UplinkNASCount,
			EKSI:           mapped.EKSI,
			Ciphering:      idleCiphering,
			Integrity:      idleIntegrity,
		},
	}); err != nil {
		return fmt.Errorf("mobility registration update over NR: %w", err)
	}

	accept, err := u.WaitForNASGMMMessage(uint8(fgs.MsgRegistrationAccept), attachTimeout)
	if err != nil {
		return fmt.Errorf("registration accept for the inter-system change: %w", err)
	}

	if err := assertAdoptedSession(accept); err != nil {
		return err
	}

	return assertSessionOn(ctx, env, "5G", before.addrs)
}

func assertAdoptedSession(plain []byte) error {
	accept, err := fgs.ParseRegistrationAccept(plain)
	if err != nil {
		return fmt.Errorf("parse the registration accept: %w", err)
	}

	if accept.PDUSessionStatus == nil || !accept.PDUSessionStatus.PSI[movedPDUSessionID] {
		return fmt.Errorf("PDU session status = %+v, want PDU session %d active: the PDN connection did not become one",
			accept.PDUSessionStatus, movedPDUSessionID)
	}

	if accept.EPSBearerContextStatus == nil || !accept.EPSBearerContextStatus.Active[movedEPSBearerIdentity] {
		return fmt.Errorf("EPS bearer context status = %+v, want EBI %d active",
			accept.EPSBearerContextStatus, movedEPSBearerIdentity)
	}

	return nil
}

func assertSessionOn(ctx context.Context, env scenarios.Env, want string, addrs ueAddresses) error {
	cl, err := client.New(&client.Config{BaseURL: env.APIAddress})
	if err != nil {
		return fmt.Errorf("create core client: %w", err)
	}

	cl.SetToken(env.APIToken)

	deadline := time.Now().Add(sessionSettle)

	var last string

	for {
		sub, err := cl.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: interworkingIMSI})
		if err == nil {
			for _, s := range sub.Sessions {
				last = s.RadioAccessType

				if s.RadioAccessType != want {
					continue
				}

				if addrs.v4 != "" && s.IPv4Address != addrs.v4 {
					return fmt.Errorf("session on %s holds address %s, want the %s it had before the move",
						want, s.IPv4Address, addrs.v4)
				}

				return nil
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("no session on %s after the move (last seen on %q)", want, last)
		}

		time.Sleep(200 * time.Millisecond)
	}
}
