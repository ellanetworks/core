// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const (
	handoverTimeout        = 15 * time.Second
	movedEPSBearerIdentity = s1ap.ERABID(5)
	targetENBUEID          = int64(70)
	mobilityRANUENGAPID    = int64(scenarios.DefaultRANUENGAPID + 1)
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "interworking/handover_5gs_to_eps",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run:       runHandover5GSToEPS,
		Fixture:   fixture,
	})
}

// TS 23.502 §4.11.1.2.1
func runHandover5GSToEPS(ctx context.Context, env scenarios.Env, _ any) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}
	defer gNodeB.Close()

	e, err := startENBOnSecondaryN3(env)
	if err != nil {
		return err
	}

	defer func() { _ = e.Close() }()

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	newUE, err := newInterworkingUE(gNodeB, true)
	if err != nil {
		return err
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: movedPDUSessionID,
		UE:           newUE,
	}); err != nil {
		return fmt.Errorf("initial registration over NR: %w", err)
	}

	if err := provisionEPSNASAlgorithms(gNodeB, newUE, ranUENGAPID); err != nil {
		return err
	}

	before, err := probeOver5GS(ctx, env, gNodeB, newUE, mobilityRANUENGAPID, "over N3 before the handover")
	if err != nil {
		return err
	}

	bearer, err := handoverToEPS(gNodeB, e, newUE, mobilityRANUENGAPID)
	if err != nil {
		return err
	}

	// TS 24.301 §5.5.3.2.2 case zd
	if err := trackingAreaUpdate(e, newUE, bearer); err != nil {
		return err
	}

	after, err := probeAfterHandover(ctx, env, e, bearer, before.addrs)
	if err != nil {
		return err
	}

	return assertContinuity(before, after)
}

// TS 23.003 §2.10.2.1.2
func trackingAreaUpdate(e *s1enb.ENB, u *ue.UE, bearer handoverBearer) error {
	if u.UeSecurity.Guti == nil || u.UeSecurity.Guti.GUTI == nil {
		return errors.New("the UE holds no 5G-GUTI to map into a tracking area update")
	}

	if err := e.TrackingAreaUpdateAfterHandover(bearer.epsUE, bearer.mmeUEID, targetENBUEID,
		etsi.MapGUTI5GToEPS(*u.UeSecurity.Guti.GUTI), handoverTimeout); err != nil {
		return fmt.Errorf("tracking area update after the handover: %w", err)
	}

	return nil
}

type handoverBearer struct {
	upfAddress string
	ulTEID     uint32
	dlTEID     uint32
	mmeUEID    int64
	epsUE      *s1enb.UE
}

func handoverToEPS(gNodeB *gnb.GnodeB, e *s1enb.ENB, u *ue.UE, ranUENGAPID int64) (handoverBearer, error) {
	enbID, err := strconv.ParseUint(scenarios.DefaultGNBID, 16, 32)
	if err != nil {
		return handoverBearer{}, fmt.Errorf("parse eNB ID %q: %w", scenarios.DefaultGNBID, err)
	}

	if err := gNodeB.SendHandoverRequiredToEPS(&gnb.HandoverToEPSOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID:   ranUENGAPID,
		TargetMcc:     scenarios.DefaultMCC,
		TargetMnc:     scenarios.DefaultMNC,
		TargetTac:     scenarios.DefaultTAC,
		TargetENBID:   uint32(enbID),
		PDUSessionIDs: []int64{movedPDUSessionID},
	}); err != nil {
		return handoverBearer{}, fmt.Errorf("send Handover Required for a handover to EPS: %w", err)
	}

	req, err := e.WaitForHandoverRequest(handoverTimeout)
	if err != nil {
		return handoverBearer{}, fmt.Errorf("the target eNB got no Handover Request: %w", err)
	}

	if req.HandoverType != s1ap.HandoverTypeFiveGSToEPS {
		return handoverBearer{}, fmt.Errorf("the Handover Request handover type = %d, want fivegs-to-eps", req.HandoverType)
	}

	// TS 33.501 §8.3.2 step 4
	if req.SecurityContext.NextHopChainingCount != 2 {
		return handoverBearer{}, fmt.Errorf("the Handover Request NCC = %d, want the 2 the mapped context carries",
			req.SecurityContext.NextHopChainingCount)
	}

	if len(req.ERABToBeSetup) != 1 || req.ERABToBeSetup[0].ERABID != movedEPSBearerIdentity {
		return handoverBearer{}, fmt.Errorf("the Handover Request E-RAB list = %+v, want one bearer on E-RAB %d",
			req.ERABToBeSetup, movedEPSBearerIdentity)
	}

	mmeUEID := int64(req.MMEUES1APID)

	dlTEID, err := e.SendHandoverRequestAcknowledge(targetENBUEID, mmeUEID, movedEPSBearerIdentity)
	if err != nil {
		return handoverBearer{}, fmt.Errorf("admit the handover at the target eNB: %w", err)
	}

	cmd, err := gNodeB.WaitForHandoverToEPSCommand(handoverTimeout)
	if err != nil {
		return handoverBearer{}, fmt.Errorf("the source gNB got no Handover Command: %w", err)
	}

	if len(cmd.ReleasedPDUSessions) != 0 {
		return handoverBearer{}, fmt.Errorf("the Handover Command released PDU sessions %v, want the session handed over", cmd.ReleasedPDUSessions)
	}

	mapped, err := installMappedContext(u, cmd)
	if err != nil {
		return handoverBearer{}, err
	}

	if err := e.SendHandoverNotify(targetENBUEID, mmeUEID); err != nil {
		return handoverBearer{}, fmt.Errorf("send Handover Notify: %w", err)
	}

	anchor, err := e.AnchorAddress(req.ERABToBeSetup[0].TransportLayerAddress)
	if err != nil {
		return handoverBearer{}, fmt.Errorf("read the anchor S1-U address from the Handover Request: %w", err)
	}

	return handoverBearer{
		upfAddress: anchor,
		ulTEID:     uint32(req.ERABToBeSetup[0].GTPTEID),
		dlTEID:     dlTEID,
		mmeUEID:    mmeUEID,
		epsUE:      mapped,
	}, nil
}

// TS 33.501 §8.3.2 steps 8-9
func installMappedContext(u *ue.UE, cmd *gnb.HandoverToEPSCommand) (*s1enb.UE, error) {
	dl, err := s1enb.EstimateDownlinkNASCount(u.UeSecurity.DLCount, cmd.DownlinkNASCountSequenceNumber)
	if err != nil {
		return nil, fmt.Errorf("rebuild the downlink NAS COUNT: %w", err)
	}

	mapped := s1enb.NewUnboundUE()

	if err := mapped.InstallMappedSecurityContext(s1enb.MappedFrom5GS{
		KAMF:             u.UeSecurity.Kamf,
		DownlinkNASCount: dl,
		UplinkNASCount:   u.UeSecurity.ULCount,
		Ciphering:        uint8(u.UeSecurity.EPSNASAlgorithms.Ciphering),
		Integrity:        uint8(u.UeSecurity.EPSNASAlgorithms.Integrity),
		EKSI:             uint8(u.UeSecurity.NgKsi.Ksi),
	}); err != nil {
		return nil, fmt.Errorf("derive the mapped EPS security context: %w", err)
	}

	return mapped, nil
}

func probeAfterHandover(ctx context.Context, env scenarios.Env, e *s1enb.ENB, bearer handoverBearer, addrs ueAddresses) (sessionFacts, error) {
	opts := &s1enb.TunnelOpts{
		UpfAddress:       bearer.upfAddress,
		ULTEID:           bearer.ulTEID,
		DLTEID:           bearer.dlTEID,
		TunInterfaceName: enbTunIface,
	}

	if addrs.v4 != "" {
		opts.UEIPv4 = addrs.v4 + ipv4TunPrefix
	}

	if env.HasIPv6() {
		opts.UEIPv6 = addrs.v6 + ipv6TunPrefix
	}

	if err := e.AddTunnel(opts); err != nil {
		return sessionFacts{}, fmt.Errorf("add the S1-U tunnel the handover established: %w", err)
	}

	if env.HasIPv6() {
		if err := s1enb.WaitForULAAddr(enbTunIface, scenarios.DefaultUEIPv6Pool, slaacTimeout); err != nil {
			return sessionFacts{}, fmt.Errorf("await the SLAAC global address on S1-U: %w", err)
		}
	}

	time.Sleep(datapathSettle)

	if err := probe.Run(ctx, probe.ICMP, enbTunIface, env.PingDestination(), scenarios.DefaultProbePort, wantsIPv6Probe(env)); err != nil {
		return sessionFacts{}, fmt.Errorf("ping over S1-U after the handover: %w", err)
	}

	return sessionFactsFor(ctx, env, addrs, enbTunIface, bearer.upfAddress, bearer.ulTEID, "S1-U after the handover")
}

func provisionEPSNASAlgorithms(gNodeB *gnb.GnodeB, u *ue.UE, ranUENGAPID int64) error {
	var sessions [16]bool

	sessions[movedPDUSessionID] = true

	if err := procedure.UEContextRelease(&procedure.UEContextReleaseOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID:   ranUENGAPID,
		GnodeB:        gNodeB,
		UE:            u,
		PDUSessionIDs: sessions,
	}); err != nil {
		return fmt.Errorf("release the connection before the mobility registration update: %w", err)
	}

	gNodeB.AddUE(mobilityRANUENGAPID, u)

	if err := u.SendRegistrationRequest(mobilityRANUENGAPID, uint8(fgs.RegistrationTypeMobilityUpdating)); err != nil {
		return fmt.Errorf("mobility registration update over NR: %w", err)
	}

	if _, err := u.WaitForNASGMMMessage(uint8(fgs.MsgRegistrationAccept), attachTimeout); err != nil {
		return fmt.Errorf("registration accept for the mobility registration update: %w", err)
	}

	if u.UeSecurity.EPSNASAlgorithms == nil {
		return fmt.Errorf("the AMF provisioned no EPS NAS algorithms, so the UE cannot be handed over to EPS")
	}

	return nil
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "interworking/handover_5gs_to_eps_target_refuses",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run:       runHandover5GSToEPSTargetRefuses,
		Fixture:   fixture,
	})
}

func runHandover5GSToEPSTargetRefuses(ctx context.Context, env scenarios.Env, _ any) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}
	defer gNodeB.Close()

	e, err := startENBOnSecondaryN3(env)
	if err != nil {
		return err
	}

	defer func() { _ = e.Close() }()

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	newUE, err := newInterworkingUE(gNodeB, true)
	if err != nil {
		return err
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: movedPDUSessionID,
		UE:           newUE,
	}); err != nil {
		return fmt.Errorf("initial registration over NR: %w", err)
	}

	if err := provisionEPSNASAlgorithms(gNodeB, newUE, ranUENGAPID); err != nil {
		return err
	}

	before, err := probeOver5GS(ctx, env, gNodeB, newUE, mobilityRANUENGAPID, "over N3 before the refused handover")
	if err != nil {
		return err
	}

	if err := refuseHandoverToEPS(gNodeB, e, mobilityRANUENGAPID); err != nil {
		return err
	}

	after, err := sessionFactsFor(ctx, env, before.addrs, gnbTunIface, before.anchorAddr, before.anchorTEID,
		"over N3 after the refused handover")
	if err != nil {
		return err
	}

	return assertContinuity(before, after)
}

func refuseHandoverToEPS(gNodeB *gnb.GnodeB, e *s1enb.ENB, ranUENGAPID int64) error {
	enbID, err := strconv.ParseUint(scenarios.DefaultGNBID, 16, 32)
	if err != nil {
		return fmt.Errorf("parse eNB ID %q: %w", scenarios.DefaultGNBID, err)
	}

	if err := gNodeB.SendHandoverRequiredToEPS(&gnb.HandoverToEPSOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID:   ranUENGAPID,
		TargetMcc:     scenarios.DefaultMCC,
		TargetMnc:     scenarios.DefaultMNC,
		TargetTac:     scenarios.DefaultTAC,
		TargetENBID:   uint32(enbID),
		PDUSessionIDs: []int64{movedPDUSessionID},
	}); err != nil {
		return fmt.Errorf("send Handover Required for a handover to EPS: %w", err)
	}

	req, err := e.WaitForHandoverRequest(handoverTimeout)
	if err != nil {
		return fmt.Errorf("the target eNB got no Handover Request: %w", err)
	}

	refusal := s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkHOFailureInTarget}
	if err := e.SendHandoverFailure(int64(req.MMEUES1APID), refusal); err != nil {
		return fmt.Errorf("refuse the handover at the target eNB: %w", err)
	}

	fail, err := gNodeB.WaitForHandoverPreparationFailure(handoverTimeout)
	if err != nil {
		return fmt.Errorf("the source gNB was not told the preparation failed: %w", err)
	}

	if fail.Cause == nil {
		return errors.New("the Handover Preparation Failure carries no cause")
	}

	// TS 38.413 §9.3.1.2
	if fail.Cause.Group != ngap.CauseGroupRadioNetwork || fail.Cause.Value != ngap.CauseRadioNetworkHOFailureInTarget {
		return fmt.Errorf("the Handover Preparation Failure cause = %+v, want a radio-network failure in the target", *fail.Cause)
	}

	return nil
}
