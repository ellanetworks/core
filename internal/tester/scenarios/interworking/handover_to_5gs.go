// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"bytes"
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
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const (
	targetRANUENGAPID = int64(80)
	targetGNBIDBits   = 24
)

var handoverRequiredCause = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkTimeCriticalHandover}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "interworking/handover_eps_to_5gs",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run:       runHandoverEPSTo5GS,
		Fixture:   fixture,
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "interworking/handover_eps_to_5gs_target_refuses",
		BindFlags: func(_ *pflag.FlagSet) any { return struct{}{} },
		Run:       runHandoverEPSTo5GSTargetRefuses,
		Fixture:   fixture,
	})
}

func runHandoverEPSTo5GS(ctx context.Context, env scenarios.Env, _ any) error {
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

	epsUE, attached, before, err := attachAndProbeOverEPS(ctx, env, e)
	if err != nil {
		return err
	}

	session, container, err := handoverToFiveGS(gNodeB, e, attached)
	if err != nil {
		return err
	}

	newUE, err := adoptMappedContext(gNodeB, epsUE, attached, container)
	if err != nil {
		return err
	}

	if err := mobilityRegistrationUpdate(gNodeB, newUE); err != nil {
		return err
	}

	if _, err := e.WaitForUEContextReleaseCommand(attached.ENBUES1APID, handoverTimeout); err != nil {
		return fmt.Errorf("the source eNB was not told to release the UE after the handover completed: %w", err)
	}

	if err := e.SendUEContextReleaseComplete(attached.MMEUES1APID, attached.ENBUES1APID); err != nil {
		return fmt.Errorf("send UE Context Release Complete at the source eNB: %w", err)
	}

	after, err := probeAfterHandoverTo5GS(ctx, env, gNodeB, session, before.addrs)
	if err != nil {
		return err
	}

	return assertContinuity(before, after)
}

func runHandoverEPSTo5GSTargetRefuses(ctx context.Context, env scenarios.Env, _ any) error {
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

	_, attached, before, err := attachAndProbeOverEPS(ctx, env, e)
	if err != nil {
		return err
	}

	if err := refuseHandoverToFiveGS(gNodeB, e, attached); err != nil {
		return err
	}

	after, err := sessionFactsFor(ctx, env, before.addrs, enbTunIface, before.anchorAddr, before.anchorTEID,
		"over S1-U after the refused handover")
	if err != nil {
		return err
	}

	return assertContinuity(before, after)
}

func attachAndProbeOverEPS(ctx context.Context, env scenarios.Env, e *s1enb.ENB) (*s1enb.UE, *s1enb.AttachResult, sessionFacts, error) {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return nil, nil, sessionFacts{}, err
	}

	epsUE := e.NewUE(interworkingIMSI, k, opc)
	epsUE.RequestPDNType(uint8(eps.PDNTypeIPv4v6))
	epsUE.AnnounceN1Mode(movedPDUSessionID)

	res, err := e.Attach(epsUE, attachTimeout)
	if err != nil {
		return nil, nil, sessionFacts{}, fmt.Errorf("attach over E-UTRAN: %w", err)
	}

	facts, err := probeOverEPS(ctx, env, e, res, "before the handover")
	if err != nil {
		return nil, nil, sessionFacts{}, err
	}

	return epsUE, res, facts, nil
}

func handoverRequiredToFiveGS(e *s1enb.ENB, attached *s1enb.AttachResult) error {
	gnbID, err := strconv.ParseUint(scenarios.DefaultGNBID, 16, 32)
	if err != nil {
		return fmt.Errorf("parse gNB ID %q: %w", scenarios.DefaultGNBID, err)
	}

	tac, err := strconv.ParseUint(scenarios.DefaultTAC, 16, 32)
	if err != nil {
		return fmt.Errorf("parse tracking area code %q: %w", scenarios.DefaultTAC, err)
	}

	if err := e.SendHandoverRequiredToFiveGS(&s1enb.HandoverToFiveGSOpts{
		ENBUEID:         attached.ENBUES1APID,
		MMEUEID:         attached.MMEUES1APID,
		TargetGNBID:     uint32(gnbID),
		TargetGNBIDBits: targetGNBIDBits,
		TargetTAC:       uint32(tac),
		Cause:           &handoverRequiredCause,
	}); err != nil {
		return fmt.Errorf("send Handover Required for a handover to 5GS: %w", err)
	}

	return nil
}

func handoverToFiveGS(gNodeB *gnb.GnodeB, e *s1enb.ENB, attached *s1enb.AttachResult) (
	*gnb.PDUSessionResult, fgs.S1ModeToN1ModeNASTransparentContainer, error,
) {
	var none fgs.S1ModeToN1ModeNASTransparentContainer

	if err := handoverRequiredToFiveGS(e, attached); err != nil {
		return nil, none, err
	}

	req, err := gNodeB.WaitForHandoverRequest(handoverTimeout)
	if err != nil {
		return nil, none, fmt.Errorf("the target gNB got no Handover Request: %w", err)
	}

	container, err := checkArrivingHandoverRequest(req)
	if err != nil {
		return nil, none, err
	}

	nasc, err := container.MarshalBinary()
	if err != nil {
		return nil, none, fmt.Errorf("re-encode the NAS container for the target to source container: %w", err)
	}

	admitted, err := gNodeB.AdmitHandover(&gnb.HandoverAdmissionOpts{
		Request:        req,
		RANUENGAPID:    targetRANUENGAPID,
		TargetToSource: nasc,
	})
	if err != nil {
		return nil, none, fmt.Errorf("admit the handover at the target gNB: %w", err)
	}

	if len(admitted) != 1 {
		return nil, none, fmt.Errorf("the target gNB admitted %d PDU sessions, want the one handed over", len(admitted))
	}

	cmd, err := e.WaitForHandoverCommand(attached.ENBUES1APID, handoverTimeout)
	if err != nil {
		return nil, none, fmt.Errorf("the source eNB got no Handover Command: %w", err)
	}

	if cmd.HandoverType != s1ap.HandoverTypeEPSToFiveGS {
		return nil, none, fmt.Errorf("the Handover Command handover type = %d, want eps-to-5gs", cmd.HandoverType)
	}

	if len(cmd.ERABToRelease) != 0 {
		return nil, none, fmt.Errorf("the Handover Command released E-RABs %+v, want the bearer handed over", cmd.ERABToRelease)
	}

	if !bytes.Equal(cmd.TargetToSource, nasc) {
		return nil, none, fmt.Errorf("the Handover Command relayed % x as the target to source container, want the % x the AMF built",
			cmd.TargetToSource, nasc)
	}

	relayed, err := fgs.ParseS1ModeToN1ModeNASTransparentContainer(cmd.TargetToSource)
	if err != nil {
		return nil, none, fmt.Errorf("parse the NAS container the Handover Command relayed: %w", err)
	}

	if err := gNodeB.SendHandoverNotify(&gnb.HandoverNotifyOpts{
		AMFUENGAPID: int64(req.AMFUENGAPID),
		RANUENGAPID: targetRANUENGAPID,
	}); err != nil {
		return nil, none, fmt.Errorf("send Handover Notify: %w", err)
	}

	return &admitted[0], relayed, nil
}

func checkArrivingHandoverRequest(req *ngap.HandoverRequest) (fgs.S1ModeToN1ModeNASTransparentContainer, error) {
	var none fgs.S1ModeToN1ModeNASTransparentContainer

	if req.HandoverType != ngap.HandoverTypeEPSToFiveGS {
		return none, fmt.Errorf("the Handover Request handover type = %d, want eps-to-5gs", req.HandoverType)
	}

	want := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkTimeCriticalHandover}
	if req.Cause == nil || *req.Cause != want {
		return none, fmt.Errorf("the Handover Request cause = %+v, want the time-critical handover the eNB asked for", req.Cause)
	}

	if req.SecurityContext.NextHopChainingCount != 0 {
		return none, fmt.Errorf("the Handover Request NCC = %d, want the 0 that names the temporary K_gNB",
			req.SecurityContext.NextHopChainingCount)
	}

	if req.NewSecurityContextInd == nil {
		return none, errors.New("the Handover Request carries no New Security Context Indicator, so the target would not tell the UE to take the mapped context into use")
	}

	if len(req.NASC) == 0 {
		return none, errors.New("the Handover Request carries no NAS container, so the UE cannot build the mapped 5G security context")
	}

	if len(req.PDUSessionResourceSetupListHOReq) != 1 {
		return none, fmt.Errorf("the Handover Request names %d PDU sessions, want the one handed over",
			len(req.PDUSessionResourceSetupListHOReq))
	}

	container, err := fgs.ParseS1ModeToN1ModeNASTransparentContainer(req.NASC)
	if err != nil {
		return none, fmt.Errorf("parse the NAS container the AMF built: %w", err)
	}

	if !container.NgKSI.Mapped {
		return none, fmt.Errorf("the NAS container's ngKSI %+v is not a mapped one", container.NgKSI)
	}

	return container, nil
}

func adoptMappedContext(gNodeB *gnb.GnodeB, epsUE *s1enb.UE, attached *s1enb.AttachResult,
	container fgs.S1ModeToN1ModeNASTransparentContainer,
) (*ue.UE, error) {
	material, err := epsUE.SecurityContextForHandoverToFiveGS(container.NCC)
	if err != nil {
		return nil, fmt.Errorf("rebuild the EPS key material the mapped context hangs off: %w", err)
	}

	newUE, err := newInterworkingUE(gNodeB, false)
	if err != nil {
		return nil, err
	}

	if err := newUE.InstallMappedSecurityContextFromEPS(ue.MappedFromEPS{
		KASME:     material.KASME,
		NH:        material.NH,
		Container: container,
	}); err != nil {
		return nil, fmt.Errorf("derive the mapped 5G security context: %w", err)
	}

	if attached.GUTI == nil || attached.GUTI.GUTI == nil {
		return nil, errors.New("the UE holds no GUTI to map into a 5G-GUTI")
	}

	mapped := fgs.GUTIIdentity(etsi.MapGUTIEPSTo5G(*attached.GUTI.GUTI))
	newUE.Set5gGuti(&mapped)

	gNodeB.AddUE(targetRANUENGAPID, newUE)

	return newUE, nil
}

func mobilityRegistrationUpdate(gNodeB *gnb.GnodeB, newUE *ue.UE) error {
	amfUENGAPID := gNodeB.GetAMFUENGAPID(targetRANUENGAPID)
	if amfUENGAPID == 0 {
		return errors.New("the target gNB holds no AMF UE NGAP ID for the UE it took over")
	}

	if err := newUE.SendMobilityRegistrationUpdate(amfUENGAPID, targetRANUENGAPID); err != nil {
		return fmt.Errorf("mobility registration update over NR: %w", err)
	}

	if _, err := newUE.WaitForNASGMMMessage(uint8(fgs.MsgRegistrationAccept), attachTimeout); err != nil {
		return fmt.Errorf("registration accept for the mobility registration update: %w", err)
	}

	return nil
}

func refuseHandoverToFiveGS(gNodeB *gnb.GnodeB, e *s1enb.ENB, attached *s1enb.AttachResult) error {
	if err := handoverRequiredToFiveGS(e, attached); err != nil {
		return err
	}

	req, err := gNodeB.WaitForHandoverRequest(handoverTimeout)
	if err != nil {
		return fmt.Errorf("the target gNB got no Handover Request: %w", err)
	}

	refusal := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkNoRadioResourcesInTargetCell}
	if err := gNodeB.SendHandoverFailure(int64(req.AMFUENGAPID), refusal); err != nil {
		return fmt.Errorf("refuse the handover at the target gNB: %w", err)
	}

	fail, err := e.WaitForHandoverPreparationFailure(attached.ENBUES1APID, handoverTimeout)
	if err != nil {
		return fmt.Errorf("the source eNB was not told the preparation failed: %w", err)
	}

	if fail.Cause == nil {
		return errors.New("the Handover Preparation Failure carries no cause")
	}

	if fail.Cause.Group != s1ap.CauseGroupRadioNetwork || fail.Cause.Value != s1ap.CauseRadioNetworkNoRadioResourcesInTargetCell {
		return fmt.Errorf("the Handover Preparation Failure cause = %+v, want no radio resources available in the target cell", *fail.Cause)
	}

	return nil
}

func probeAfterHandoverTo5GS(ctx context.Context, env scenarios.Env, gNodeB *gnb.GnodeB,
	session *gnb.PDUSessionResult, addrs ueAddresses,
) (sessionFacts, error) {
	tunnel := &gnb.TunnelOpts{
		UpfAddress:       session.UpfAddress,
		TunInterfaceName: gnbTunIface,
		ULTEID:           session.ULTEID,
		DLTEID:           session.DLTEID,
		QFI:              session.QFI,
	}

	if addrs.v4 != "" {
		tunnel.UEIPv4 = addrs.v4 + ipv4TunPrefix
	}

	if env.HasIPv6() {
		tunnel.UEIPv6 = addrs.v6 + ipv6TunPrefix
	}

	if err := gNodeB.AddTunnel(tunnel); err != nil {
		return sessionFacts{}, fmt.Errorf("add the N3 tunnel the handover established: %w", err)
	}

	if env.HasIPv6() {
		if err := gnb.WaitForULAAddr(gnbTunIface, scenarios.DefaultUEIPv6Pool, slaacTimeout); err != nil {
			return sessionFacts{}, fmt.Errorf("await the SLAAC global address on N3: %w", err)
		}
	}

	time.Sleep(datapathSettle)

	if err := probe.Run(ctx, probe.ICMP, gnbTunIface, env.PingDestination(), scenarios.DefaultProbePort, wantsIPv6Probe(env)); err != nil {
		return sessionFacts{}, fmt.Errorf("ping over N3 after the handover: %w", err)
	}

	return sessionFactsFor(ctx, env, addrs, gnbTunIface, session.UpfAddress, session.ULTEID, "N3 after the handover")
}
