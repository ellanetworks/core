// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const (
	handoverTimeout = 15 * time.Second

	// movedEPSBearerIdentity is the identity the AMF allocates to the first PDU
	// session of an S1-capable UE (the range starts at 5, TS 24.301 §6.5.0).
	movedEPSBearerIdentity = s1ap.ERABID(5)

	// targetENBUEID is the eNB-UE-S1AP-ID the target eNB allocates for the
	// arriving UE.
	targetENBUEID = int64(70)
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "interworking/handover_5gs_to_eps",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run:       runHandover5GSToEPS,
		Fixture:   fixture,
	})
}

// runHandover5GSToEPS drives an N26 handover: the UE registers over NR with a
// PDU session, the source gNB asks for a handover to the eNB, and the session
// continues over S1-U with the same address and anchor tunnel
// (TS 23.502 §4.11.1.2.1).
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

	// The EPS NAS algorithms belong in the security mode command (TS 33.501
	// §6.7.2), but the S1 UE network capability they are selected from is not a
	// cleartext IE (TS 24.501 §4.4.6): it reaches the AMF in the security mode
	// complete, after that command was built. The AMF settles the debt with the
	// security mode procedure of TS 24.501 §5.4.2.2 at the next registration, so
	// the UE registers again before it can be handed over.
	if err := newUE.SendRegistrationRequest(ranUENGAPID, uint8(fgs.RegistrationTypeMobilityUpdating)); err != nil {
		return fmt.Errorf("mobility registration update over NR: %w", err)
	}

	if _, err := newUE.WaitForNASGMMMessage(uint8(fgs.MsgRegistrationAccept), attachTimeout); err != nil {
		return fmt.Errorf("registration accept for the mobility registration update: %w", err)
	}

	if newUE.UeSecurity.EPSNASAlgorithms == nil {
		return fmt.Errorf("the AMF provisioned no EPS NAS algorithms, so the UE cannot be handed over to EPS")
	}

	before, err := probeOver5GS(ctx, env, gNodeB, newUE, ranUENGAPID, "over N3 before the handover")
	if err != nil {
		return err
	}

	bearer, err := handoverToEPS(gNodeB, e, newUE, ranUENGAPID)
	if err != nil {
		return err
	}

	after, err := probeAfterHandover(ctx, env, e, bearer, before.addrs)
	if err != nil {
		return err
	}

	return assertContinuity(before, after)
}

// handoverBearer is the S1-U endpoint pair the handover established: the
// anchor's uplink F-TEID, which the MME took from the session it moved, and the
// downlink TEID the target eNB allocated.
type handoverBearer struct {
	upfAddress string
	ulTEID     uint32
	dlTEID     uint32
}

// handoverToEPS runs the RAN half of the handover on both sides, and has the UE
// derive the EPS security context it arrives with.
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

	// TS 33.501 §8.3.2 step 4: the MME passes on the {NH, NCC=2} pair the AMF
	// derived, so the target eNB and the UE compute the same K_eNB.
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

	if err := installMappedContext(u, cmd); err != nil {
		return handoverBearer{}, err
	}

	// The UE has reached the target; the eNB reports it, which is what commits the
	// user plane and lets the AMF release the 5GS side.
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
	}, nil
}

// installMappedContext has the UE derive the EPS security context it will use in
// EPS, from the K_AMF it holds and the downlink NAS COUNT the Handover Command
// named (TS 33.501 §8.3.2 steps 8-9).
func installMappedContext(u *ue.UE, cmd *gnb.HandoverToEPSCommand) error {
	dl, err := s1enb.EstimateDownlinkNASCount(u.UeSecurity.DLCount, cmd.DownlinkNASCountSequenceNumber)
	if err != nil {
		return fmt.Errorf("rebuild the downlink NAS COUNT: %w", err)
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
		return fmt.Errorf("derive the mapped EPS security context: %w", err)
	}

	return nil
}

// probeAfterHandover builds the S1-U tunnel the handover established and checks
// the session still carries traffic, with the address the UE held in 5GS.
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
