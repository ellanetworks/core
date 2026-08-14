// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const (
	s1RefuseIMSI     = "001017271246682"
	s1CancelIMSI     = "001017271246683"
	s1FailureTimeout = 10 * time.Second
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/s1_handover_target_refuses",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1HandoverTargetRefuses,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(s1RefuseIMSI, "")},
			}
		},
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/s1_handover_cancel",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1HandoverCancel,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(s1CancelIMSI, "")},
			}
		},
	})
}

type s1HandoverPair struct {
	Source   *s1enb.ENB
	Target   *s1enb.ENB
	UE       *s1enb.UE
	Attached *s1enb.AttachResult
}

func startS1HandoverPair(env scenarios.Env, imsi string) (*s1HandoverPair, func(), error) {
	s1mme, err := s1mmeAddress(env.FirstCore())
	if err != nil {
		return nil, nil, err
	}

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return nil, nil, err
	}

	enbID, err := strconv.ParseUint(scenarios.DefaultGNBID, 16, 32)
	if err != nil {
		return nil, nil, fmt.Errorf("parse eNB ID %q: %w", scenarios.DefaultGNBID, err)
	}

	g := env.FirstGNB()

	source, err := s1enb.Start(&s1enb.StartOpts{
		ENBID: uint32(enbID), MCC: scenarios.DefaultMCC, MNC: scenarios.DefaultMNC, TAC: scenarios.DefaultTAC,
		Name: "Source-S1eNB", CoreS1MMEAddress: s1mme, ENBAddress: g.N2Address, ENBN3Address: g.N3Address,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("start source eNB: %w", err)
	}

	target, err := s1enb.Start(&s1enb.StartOpts{
		ENBID: uint32(enbID) + 1, MCC: scenarios.DefaultMCC, MNC: scenarios.DefaultMNC, TAC: scenarios.DefaultTAC,
		Name: "Target-S1eNB", CoreS1MMEAddress: s1mme, ENBAddress: g.N2Address, ENBN3Address: g.N3Address,
	})
	if err != nil {
		_ = source.Close()

		return nil, nil, fmt.Errorf("start target eNB: %w", err)
	}

	closeBoth := func() {
		_ = target.Close()
		_ = source.Close()
	}

	ue := source.NewUE(imsi, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	attached, err := source.Attach(ue, s1FailureTimeout)
	if err != nil {
		closeBoth()

		return nil, nil, fmt.Errorf("attach on source eNB: %w", err)
	}

	return &s1HandoverPair{Source: source, Target: target, UE: ue, Attached: attached}, closeBoth, nil
}

func (p *s1HandoverPair) requireHandover() (*s1ap.HandoverRequest, error) {
	if err := p.Source.SendHandoverRequired(p.Attached.ENBUES1APID, p.Attached.MMEUES1APID, p.Target.GlobalENBID()); err != nil {
		return nil, fmt.Errorf("send Handover Required: %w", err)
	}

	req, err := p.Target.WaitForHandoverRequest(s1FailureTimeout)
	if err != nil {
		return nil, fmt.Errorf("the target eNB got no Handover Request: %w", err)
	}

	return req, nil
}

func runS1HandoverTargetRefuses(_ context.Context, env scenarios.Env, _ any) error {
	pair, closePair, err := startS1HandoverPair(env, s1RefuseIMSI)
	if err != nil {
		return err
	}

	defer closePair()

	req, err := pair.requireHandover()
	if err != nil {
		return err
	}

	refusal := s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkHOFailureInTarget}
	if err := pair.Target.SendHandoverFailure(int64(req.MMEUES1APID), refusal); err != nil {
		return fmt.Errorf("refuse the handover at the target eNB: %w", err)
	}

	fail, err := pair.Source.WaitForHandoverPreparationFailure(pair.Attached.ENBUES1APID, s1FailureTimeout)
	if err != nil {
		return fmt.Errorf("the source eNB was not told the preparation failed: %w", err)
	}

	if fail.MMEUES1APID == nil || int64(*fail.MMEUES1APID) != pair.Attached.MMEUES1APID {
		return fmt.Errorf("the Handover Preparation Failure MME-UE-S1AP-ID = %v, want %d", fail.MMEUES1APID, pair.Attached.MMEUES1APID)
	}

	if fail.Cause == nil {
		return errors.New("the Handover Preparation Failure carries no cause")
	}

	if fail.Cause.Group != s1ap.CauseGroupRadioNetwork || fail.Cause.Value != s1ap.CauseRadioNetworkHOFailureInTarget {
		return fmt.Errorf("the Handover Preparation Failure cause = %+v, want a radio-network failure in the target", *fail.Cause)
	}

	return s1SurvivesOnSource(pair)
}

func runS1HandoverCancel(_ context.Context, env scenarios.Env, _ any) error {
	pair, closePair, err := startS1HandoverPair(env, s1CancelIMSI)
	if err != nil {
		return err
	}

	defer closePair()

	req, err := pair.requireHandover()
	if err != nil {
		return err
	}

	targetENBUEID := pair.Target.AllocateENBUEID()

	if _, err := pair.Target.SendHandoverRequestAcknowledge(targetENBUEID, int64(req.MMEUES1APID), pair.Attached.ERABID); err != nil {
		return fmt.Errorf("admit the handover at the target eNB: %w", err)
	}

	if _, err := pair.Source.WaitForHandoverCommand(pair.Attached.ENBUES1APID, s1FailureTimeout); err != nil {
		return fmt.Errorf("await Handover Command: %w", err)
	}

	cancelCause := s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkHandoverCancelled}
	if err := pair.Source.SendHandoverCancel(pair.Attached.MMEUES1APID, pair.Attached.ENBUES1APID, cancelCause); err != nil {
		return fmt.Errorf("cancel the handover at the source eNB: %w", err)
	}

	ack, err := pair.Source.WaitForHandoverCancelAcknowledge(pair.Attached.ENBUES1APID, s1FailureTimeout)
	if err != nil {
		return fmt.Errorf("the source eNB was not acknowledged its Handover Cancel: %w", err)
	}

	if ack.MMEUES1APID == nil || int64(*ack.MMEUES1APID) != pair.Attached.MMEUES1APID {
		return fmt.Errorf("the Handover Cancel Acknowledge MME-UE-S1AP-ID = %v, want %d", ack.MMEUES1APID, pair.Attached.MMEUES1APID)
	}

	if ack.ENBUES1APID == nil || int64(*ack.ENBUES1APID) != pair.Attached.ENBUES1APID {
		return fmt.Errorf("the Handover Cancel Acknowledge eNB-UE-S1AP-ID = %v, want the source's %d", ack.ENBUES1APID, pair.Attached.ENBUES1APID)
	}

	if _, err := pair.Target.WaitForUEContextReleaseCommand(targetENBUEID, s1FailureTimeout); err != nil {
		return fmt.Errorf("the prepared target eNB was not told to release the UE context: %w", err)
	}

	return s1SurvivesOnSource(pair)
}

func s1SurvivesOnSource(pair *s1HandoverPair) error {
	if _, err := pair.Source.WaitForUEContextReleaseCommand(pair.Attached.ENBUES1APID, 500*time.Millisecond); err == nil {
		return errors.New("the source eNB was told to release the UE context, but the UE never left it")
	}

	return nil
}
