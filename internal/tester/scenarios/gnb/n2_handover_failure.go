// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	ngaplib "github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
)

const (
	n2RefuseIMSI      = "001017271246594"
	n2CancelIMSI      = "001017271246595"
	n2FailureTargetID = "000002"
	n2FailureTimeout  = 5 * time.Second
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/n2_handover_target_refuses",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runN2HandoverTargetRefuses,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{
					scenarios.DefaultSubscriberWith(n2RefuseIMSI, ""),
				},
			}
		},
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/n2_handover_cancel",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runN2HandoverCancel,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{
					scenarios.DefaultSubscriberWith(n2CancelIMSI, ""),
				},
			}
		},
	})
}

type n2HandoverPair struct {
	Source      *gnb.GnodeB
	Target      *gnb.GnodeB
	RANUENGAPID int64
	AMFUENGAPID int64
}

func startN2HandoverPair(env scenarios.Env, imsi string) (*n2HandoverPair, func(), error) {
	sourceGNB, err := startGNB(env)
	if err != nil {
		return nil, nil, err
	}

	targetGNB, err := startXnTargetGNB(env, n2FailureTargetID, env.FirstGNB().N2Address, "")
	if err != nil {
		sourceGNB.Close()

		return nil, nil, err
	}

	closeBoth := func() {
		targetGNB.Close()
		sourceGNB.Close()
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	newUE, err := newDefaultUE(sourceGNB, imsi[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, scenarios.DefaultPDUSessionTypeIPv4)
	if err != nil {
		closeBoth()

		return nil, nil, fmt.Errorf("create UE: %w", err)
	}

	sourceGNB.AddUE(ranUENGAPID, newUE)

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	}); err != nil {
		closeBoth()

		return nil, nil, fmt.Errorf("initial registration: %w", err)
	}

	if _, err := sourceGNB.WaitForPDUSession(ranUENGAPID, int64(scenarios.DefaultPDUSessionID), n2FailureTimeout); err != nil {
		closeBoth()

		return nil, nil, fmt.Errorf("source gNB: wait PDU session: %w", err)
	}

	return &n2HandoverPair{
		Source:      sourceGNB,
		Target:      targetGNB,
		RANUENGAPID: ranUENGAPID,
		AMFUENGAPID: sourceGNB.GetAMFUENGAPID(ranUENGAPID),
	}, closeBoth, nil
}

func (p *n2HandoverPair) requireHandover() (*ngaplib.HandoverRequest, error) {
	if err := p.Source.SendHandoverRequired(&gnb.HandoverRequiredOpts{
		AMFUENGAPID:  p.AMFUENGAPID,
		RANUENGAPID:  p.RANUENGAPID,
		HandoverType: ngaplib.HandoverTypeIntra5GS,
		TargetGnbID:  n2FailureTargetID,
		PDUSessions: []gnb.HandoverRequiredPDUSession{
			{PDUSessionID: int64(scenarios.DefaultPDUSessionID)},
		},
	}); err != nil {
		return nil, fmt.Errorf("send HandoverRequired: %w", err)
	}

	req, err := p.Target.WaitForHandoverRequest(n2FailureTimeout)
	if err != nil {
		return nil, fmt.Errorf("the target gNB got no HandoverRequest: %w", err)
	}

	return req, nil
}

func runN2HandoverTargetRefuses(_ context.Context, env scenarios.Env, _ any) error {
	pair, closePair, err := startN2HandoverPair(env, n2RefuseIMSI)
	if err != nil {
		return err
	}

	defer closePair()

	req, err := pair.requireHandover()
	if err != nil {
		return err
	}

	refusal := ngaplib.Cause{Group: ngaplib.CauseGroupRadioNetwork, Value: ngaplib.CauseRadioNetworkHOFailureInTarget}
	if err := pair.Target.SendHandoverFailure(int64(req.AMFUENGAPID), refusal); err != nil {
		return fmt.Errorf("refuse the handover at the target gNB: %w", err)
	}

	fail, err := pair.Source.WaitForHandoverPreparationFailure(n2FailureTimeout)
	if err != nil {
		return fmt.Errorf("the source gNB was not told the preparation failed: %w", err)
	}

	if fail.AMFUENGAPID == nil || int64(*fail.AMFUENGAPID) != pair.AMFUENGAPID {
		return fmt.Errorf("the Handover Preparation Failure AMF-UE-NGAP-ID = %v, want %d", fail.AMFUENGAPID, pair.AMFUENGAPID)
	}

	if fail.RANUENGAPID == nil || int64(*fail.RANUENGAPID) != pair.RANUENGAPID {
		return fmt.Errorf("the Handover Preparation Failure RAN-UE-NGAP-ID = %v, want the source's %d", fail.RANUENGAPID, pair.RANUENGAPID)
	}

	if fail.Cause == nil {
		return errors.New("the Handover Preparation Failure carries no cause")
	}

	if fail.Cause.Group != ngaplib.CauseGroupRadioNetwork || fail.Cause.Value != ngaplib.CauseRadioNetworkHOFailureInTarget {
		return fmt.Errorf("the Handover Preparation Failure cause = %+v, want a radio-network failure in the target", *fail.Cause)
	}

	return survivesOnSource(pair)
}

func runN2HandoverCancel(_ context.Context, env scenarios.Env, _ any) error {
	pair, closePair, err := startN2HandoverPair(env, n2CancelIMSI)
	if err != nil {
		return err
	}

	defer closePair()

	req, err := pair.requireHandover()
	if err != nil {
		return err
	}

	targetRanUENGAPID := int64(300)

	if err := pair.Target.SendHandoverRequestAcknowledge(&gnb.HandoverRequestAcknowledgeOpts{
		AMFUENGAPID: int64(req.AMFUENGAPID),
		RANUENGAPID: targetRanUENGAPID,
		PDUSessions: []gnb.HandoverAdmittedPDUSession{
			{
				PDUSessionID: int64(scenarios.DefaultPDUSessionID),
				DLTeid:       uint32(9300),
				DLIP:         netip.MustParseAddr(env.FirstGNB().N3Address),
			},
		},
		TargetToSourceTransparentContainer: n2HandoverRRCContainer,
	}); err != nil {
		return fmt.Errorf("admit the handover at the target gNB: %w", err)
	}

	hoCmdFrame, err := pair.Source.WaitForMessage(gnb.Successful, ngaplib.ProcHandoverPreparation, n2FailureTimeout)
	if err != nil {
		return fmt.Errorf("source gNB: wait HandoverCommand: %w", err)
	}

	if err := assertN2HandoverCommand(hoCmdFrame, pair.AMFUENGAPID, pair.RANUENGAPID); err != nil {
		return err
	}

	cancelCause := ngaplib.Cause{Group: ngaplib.CauseGroupRadioNetwork, Value: ngaplib.CauseRadioNetworkHandoverCancelled}
	if err := pair.Source.SendHandoverCancel(pair.AMFUENGAPID, pair.RANUENGAPID, cancelCause); err != nil {
		return fmt.Errorf("cancel the handover at the source gNB: %w", err)
	}

	ack, err := pair.Source.WaitForHandoverCancelAcknowledge(n2FailureTimeout)
	if err != nil {
		return fmt.Errorf("the source gNB was not acknowledged its Handover Cancel: %w", err)
	}

	if ack.AMFUENGAPID == nil || int64(*ack.AMFUENGAPID) != pair.AMFUENGAPID {
		return fmt.Errorf("the Handover Cancel Acknowledge AMF-UE-NGAP-ID = %v, want %d", ack.AMFUENGAPID, pair.AMFUENGAPID)
	}

	if ack.RANUENGAPID == nil || int64(*ack.RANUENGAPID) != pair.RANUENGAPID {
		return fmt.Errorf("the Handover Cancel Acknowledge RAN-UE-NGAP-ID = %v, want the source's %d", ack.RANUENGAPID, pair.RANUENGAPID)
	}

	release, err := pair.Target.WaitForMessage(gnb.Initiating, ngaplib.ProcUEContextRelease, n2FailureTimeout)
	if err != nil {
		return fmt.Errorf("the prepared target gNB was not told to release the UE context: %w", err)
	}

	cmd, err := ngaplib.ParseUEContextReleaseCommand(release.Value)
	if err != nil {
		return fmt.Errorf("parse the target's UEContextReleaseCommand: %w", err)
	}

	if !cmd.UENGAPIDs.Pair || int64(cmd.UENGAPIDs.RANUENGAPID) != targetRanUENGAPID {
		return fmt.Errorf("the UEContextReleaseCommand named RAN-UE-NGAP-ID %v, want the target's %d", cmd.UENGAPIDs, targetRanUENGAPID)
	}

	return survivesOnSource(pair)
}

func survivesOnSource(pair *n2HandoverPair) error {
	if _, err := pair.Source.WaitForMessage(gnb.Initiating, ngaplib.ProcUEContextRelease, 500*time.Millisecond); err == nil {
		return errors.New("the source gNB was told to release the UE context, but the UE never left it")
	}

	return nil
}
