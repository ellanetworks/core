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
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runIdleEPSTo5GS(ctx, env, arriveAndResumeUserPlane)
		},
		Fixture: fixture,
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "interworking/idle_eps_to_5gs_returning_to_idle",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runIdleEPSTo5GS(ctx, env, arriveAndStayIdle)
		},
		Fixture: fixture,
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

	scenarios.Register(scenarios.Scenario{
		Name:      "interworking/idle_round_trip_through_eps",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run:       runIdleRoundTripThroughEPS,
		Fixture:   fixture,
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "interworking/idle_round_trip_through_5gs",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run:       runIdleRoundTripThrough5GS,
		Fixture:   fixture,
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

	_, err = gNodeB.Register(u, ranUENGAPID, movedPDUSessionID, registrationTimeout)
	if err != nil {
		return fmt.Errorf("initial registration over NR: %w", err)
	}

	moved, err := provisionEPSNASAlgorithms(gNodeB, u)
	if err != nil {
		return err
	}

	before, err := probeOver5GS(ctx, env, gNodeB, moved, "over N3 before the idle move")
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
	return goIdleOnNRConnection(gNodeB, u, mobilityRANUENGAPID)
}

func goIdleOnNRConnection(gNodeB *gnb.GnodeB, u *ue.UE, ranUENGAPID int64) error {
	sessions := []uint8{movedPDUSessionID}

	if err := gNodeB.ReleaseContext(u, ranUENGAPID, sessions, releaseTimeout); err != nil {
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
		UplinkNASCount:   u.TakeUplinkNASCountForInterSystemChange(),
		DownlinkNASCount: u.UeSecurity.DLCount,
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

func runIdleEPSTo5GS(ctx context.Context, env scenarios.Env, resume resumeUserPlane) error {
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

	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	u, err := newInterworkingUE(gNodeB, false)
	if err != nil {
		return err
	}

	if err := arriveOn5GSFromEPS(gNodeB, epsUE, u, *res.GUTI.GUTI, int64(scenarios.DefaultRANUENGAPID), resume); err != nil {
		return err
	}

	return assertSessionOn(ctx, env, "5G", before.addrs)
}

type resumeUserPlane bool

const (
	arriveAndResumeUserPlane resumeUserPlane = true
	arriveAndStayIdle        resumeUserPlane = false
)

func arriveOn5GSFromEPS(gNodeB *gnb.GnodeB, epsUE *s1enb.UE, u *ue.UE, epsGUTI eps.GUTI, ranUENGAPID int64, resume resumeUserPlane) error {
	var bearerStatus nas.EPSBearerContextStatus

	bearerStatus.Active[movedEPSBearerIdentity] = true

	container, err := epsUE.BuildTrackingAreaUpdateForContainer(epsGUTI, &bearerStatus)
	if err != nil {
		return err
	}

	mapped := epsUE.MappedContextForIdleMobility()

	gNodeB.AddUE(ranUENGAPID, u)

	var sessions [16]bool

	sessions[movedPDUSessionID] = true

	if err := u.SendIdleMobilityRegistration(ue.IdleRegistrationOpts{
		RANUENGAPID:            ranUENGAPID,
		MappedGUTI:             fgs.GUTIIdentity(etsi.MapGUTIEPSTo5G(epsGUTI)),
		EPSNASMessageContainer: container,
		PDUSessionStatus:       &sessions,
		UplinkDataStatus:       uplinkDataStatusFor(resume, sessions),
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

	return assertReactivation(accept, resume)
}

func assertReactivation(plain []byte, resume resumeUserPlane) error {
	accept, err := fgs.ParseRegistrationAccept(plain)
	if err != nil {
		return fmt.Errorf("parse the registration accept: %w", err)
	}

	switch {
	case bool(resume) && accept.PDUSessionReactivationResult == nil:
		return errors.New("the registration accept reports no PDU session reactivation result, " +
			"so the AMF did not act on the uplink data status the UE arrived with")
	case !bool(resume) && accept.PDUSessionReactivationResult != nil:
		return fmt.Errorf("the registration accept reports the reactivation result %+v though the UE asked for no user plane, "+
			"so the AMF re-established one the UE is not ready to use", accept.PDUSessionReactivationResult)
	}

	return nil
}

func uplinkDataStatusFor(resume resumeUserPlane, sessions [16]bool) *[16]bool {
	if !resume {
		return nil
	}

	return &sessions
}

const returnRANUENGAPID = int64(scenarios.DefaultRANUENGAPID + 2)

func runIdleRoundTripThroughEPS(ctx context.Context, env scenarios.Env, _ any) error {
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

	if _, err := gNodeB.Register(u, ranUENGAPID, movedPDUSessionID, registrationTimeout); err != nil {
		return fmt.Errorf("initial registration over NR: %w", err)
	}

	moved, err := provisionEPSNASAlgorithms(gNodeB, u)
	if err != nil {
		return err
	}

	before, err := probeOver5GS(ctx, env, gNodeB, moved, "over N3 before the round trip")
	if err != nil {
		return err
	}

	if err := assertSessionOn(ctx, env, "5G", before.addrs); err != nil {
		return err
	}

	e, epsUE, tau, err := roundTripOutboundLeg(ctx, env, gNodeB, u, before)
	if err != nil {
		return err
	}

	defer func() { _ = e.Close() }()

	return roundTripReturnLeg(ctx, env, e, gNodeB, u, epsUE, tau, before)
}

func roundTripOutboundLeg(ctx context.Context, env scenarios.Env, gNodeB *gnb.GnodeB, u *ue.UE, before sessionFacts) (*s1enb.ENB, *s1enb.UE, *s1enb.AttachResult, error) {
	if err := goIdleOnNR(gNodeB, u); err != nil {
		return nil, nil, nil, err
	}

	security, guti, err := idleMobilityMaterial(u)
	if err != nil {
		return nil, nil, nil, err
	}

	e, err := startENBOnSecondaryN3(env)
	if err != nil {
		return nil, nil, nil, err
	}

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return nil, nil, nil, err
	}

	epsUE := e.NewUE(interworkingIMSI, k, opc)

	var bearerStatus nas.EPSBearerContextStatus

	bearerStatus.Active[movedEPSBearerIdentity] = true

	tau, err := e.TrackingAreaUpdateFrom5GS(epsUE, s1enb.IdleTrackingAreaUpdateOpts{
		GUTI:         guti,
		ActiveFlag:   false,
		BearerStatus: &bearerStatus,
		Security:     security,
	}, attachTimeout)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("tracking area update after the idle move to EPS: %w", err)
	}

	if err := assertAdoptedBearer(tau); err != nil {
		return nil, nil, nil, err
	}

	if err := assertSessionOn(ctx, env, "4G", before.addrs); err != nil {
		return nil, nil, nil, err
	}

	return e, epsUE, tau, nil
}

func roundTripReturnLeg(ctx context.Context, env scenarios.Env, e *s1enb.ENB, gNodeB *gnb.GnodeB,
	u *ue.UE, epsUE *s1enb.UE, tau *s1enb.AttachResult, before sessionFacts,
) error {
	if tau.GUTI == nil || tau.GUTI.GUTI == nil {
		return errors.New("the tracking area update accept assigned no GUTI to enclose in the return to 5GS")
	}

	var bearerStatus nas.EPSBearerContextStatus

	bearerStatus.Active[movedEPSBearerIdentity] = true

	container, err := epsUE.BuildTrackingAreaUpdateForContainer(*tau.GUTI.GUTI, &bearerStatus)
	if err != nil {
		return err
	}

	var sessions [16]bool

	sessions[movedPDUSessionID] = true

	gNodeB.AddUE(returnRANUENGAPID, u)

	authenticated := u.ReceivedNASGMMCount(uint8(fgs.MsgAuthenticationRequest))

	if err := u.SendIdleMobilityRegistration(ue.IdleRegistrationOpts{
		RANUENGAPID:            returnRANUENGAPID,
		MappedGUTI:             fgs.GUTIIdentity(etsi.MapGUTIEPSTo5G(*tau.GUTI.GUTI)),
		EPSNASMessageContainer: container,
		PDUSessionStatus:       &sessions,
		UplinkDataStatus:       &sessions,
	}); err != nil {
		return fmt.Errorf("mobility registration update back over NR: %w", err)
	}

	accept, err := u.WaitForNASGMMMessage(uint8(fgs.MsgRegistrationAccept), attachTimeout)
	if err != nil {
		return fmt.Errorf("registration accept for the return to 5GS on the UE's native security context: %w", err)
	}

	if err := assertAdoptedSession(accept); err != nil {
		return err
	}

	if err := assertReactivation(accept, arriveAndResumeUserPlane); err != nil {
		return err
	}

	if got := u.ReceivedNASGMMCount(uint8(fgs.MsgAuthenticationRequest)) - authenticated; got != 0 {
		return fmt.Errorf("the AMF ran %d authentication request(s) on the return to 5GS, want none: "+
			"the UE arrived on a native 5G NAS security context the AMF still holds", got)
	}

	if err := assertSessionOn(ctx, env, "5G", before.addrs); err != nil {
		return err
	}

	return roundTripLeaveAgain(ctx, env, e, gNodeB, u, epsUE, before)
}

func roundTripLeaveAgain(ctx context.Context, env scenarios.Env, e *s1enb.ENB, gNodeB *gnb.GnodeB,
	u *ue.UE, epsUE *s1enb.UE, before sessionFacts,
) error {
	if err := goIdleOnNRConnection(gNodeB, u, returnRANUENGAPID); err != nil {
		return err
	}

	security, guti, err := idleMobilityMaterial(u)
	if err != nil {
		return err
	}

	var bearerStatus nas.EPSBearerContextStatus

	bearerStatus.Active[movedEPSBearerIdentity] = true

	tau, err := e.TrackingAreaUpdateFrom5GS(epsUE, s1enb.IdleTrackingAreaUpdateOpts{
		GUTI:         guti,
		ActiveFlag:   false,
		BearerStatus: &bearerStatus,
		Security:     security,
	}, attachTimeout)
	if err != nil {
		return fmt.Errorf("tracking area update leaving 5GS again after the resumed arrival: %w", err)
	}

	if err := assertAdoptedBearer(tau); err != nil {
		return err
	}

	return assertSessionOn(ctx, env, "4G", before.addrs)
}

func runIdleRoundTripThrough5GS(ctx context.Context, env scenarios.Env, _ any) error {
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

	before, err := probeOverEPS(ctx, env, e, res, "over S1-U before the round trip")
	if err != nil {
		return err
	}

	if err := assertSessionOn(ctx, env, "4G", before.addrs); err != nil {
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

	u, err := newInterworkingUE(gNodeB, false)
	if err != nil {
		return err
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	if err := arriveOn5GSFromEPS(gNodeB, epsUE, u, *res.GUTI.GUTI, ranUENGAPID, arriveAndResumeUserPlane); err != nil {
		return err
	}

	if err := assertSessionOn(ctx, env, "5G", before.addrs); err != nil {
		return err
	}

	return roundTripReturnToEPS(ctx, env, e, gNodeB, u, epsUE, ranUENGAPID, before)
}

func roundTripReturnToEPS(ctx context.Context, env scenarios.Env, e *s1enb.ENB, gNodeB *gnb.GnodeB,
	u *ue.UE, epsUE *s1enb.UE, ranUENGAPID int64, before sessionFacts,
) error {
	if err := goIdleOnNRConnection(gNodeB, u, ranUENGAPID); err != nil {
		return err
	}

	security, guti, err := idleMobilityMaterial(u)
	if err != nil {
		return err
	}

	var bearerStatus nas.EPSBearerContextStatus

	bearerStatus.Active[movedEPSBearerIdentity] = true

	tau, err := e.TrackingAreaUpdateFrom5GS(epsUE, s1enb.IdleTrackingAreaUpdateOpts{
		GUTI:         guti,
		ActiveFlag:   false,
		BearerStatus: &bearerStatus,
		Security:     security,
	}, attachTimeout)
	if err != nil {
		return fmt.Errorf("tracking area update on the return to EPS: %w", err)
	}

	if err := assertAdoptedBearer(tau); err != nil {
		return err
	}

	return assertSessionOn(ctx, env, "4G", before.addrs)
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
