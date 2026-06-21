// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const x2IMSI = "001017271246601"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/x2_path_switch",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBX2PathSwitch,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(x2IMSI, "")},
			}
		},
	})
}

// runS1ENBX2PathSwitch attaches a UE on a source eNB, then has a target eNB issue
// a PATH SWITCH REQUEST for it (the MME-side completion of an X2 handover,
// TS 36.413 §8.4.4) and verifies the MME replies with a PATH SWITCH REQUEST
// ACKNOWLEDGE carrying a fresh security context. Both eNBs run from this one
// container as distinct S1 associations.
func runS1ENBX2PathSwitch(_ context.Context, env scenarios.Env, _ any) error {
	s1mme, err := s1mmeAddress(env.FirstCore())
	if err != nil {
		return err
	}

	g := env.FirstGNB()

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	enbID, err := strconv.ParseUint(scenarios.DefaultGNBID, 16, 32)
	if err != nil {
		return fmt.Errorf("parse eNB ID %q: %w", scenarios.DefaultGNBID, err)
	}

	source, err := s1enb.Start(&s1enb.StartOpts{
		ENBID: uint32(enbID), MCC: scenarios.DefaultMCC, MNC: scenarios.DefaultMNC, TAC: scenarios.DefaultTAC,
		Name: "Source-S1eNB", CoreS1MMEAddress: s1mme, ENBAddress: g.N2Address, ENBN3Address: g.N3Address,
	})
	if err != nil {
		return fmt.Errorf("start source eNB: %w", err)
	}

	defer func() { _ = source.Close() }()

	target, err := s1enb.Start(&s1enb.StartOpts{
		ENBID: uint32(enbID) + 1, MCC: scenarios.DefaultMCC, MNC: scenarios.DefaultMNC, TAC: scenarios.DefaultTAC,
		Name: "Target-S1eNB", CoreS1MMEAddress: s1mme, ENBAddress: g.N2Address, ENBN3Address: g.N3Address,
	})
	if err != nil {
		return fmt.Errorf("start target eNB: %w", err)
	}

	defer func() { _ = target.Close() }()

	ue := source.NewUE(x2IMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	res, err := source.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach on source eNB: %w", err)
	}

	targetENBUEID := target.AllocateENBUEID()
	if err := target.SendPathSwitchRequest(targetENBUEID, res.MMEUES1APID, res.ERABID, ue.S1APSecurityCapabilities()); err != nil {
		return fmt.Errorf("send Path Switch Request: %w", err)
	}

	frame, err := target.WaitForMessage(targetENBUEID, s1enb.Successful, s1ap.ProcPathSwitchRequest, 10*time.Second)
	if err != nil {
		return fmt.Errorf("await Path Switch Request Acknowledge: %w", err)
	}

	ack, err := s1ap.ParsePathSwitchRequestAcknowledge(frame.Value)
	if err != nil {
		return fmt.Errorf("parse Path Switch Request Acknowledge: %w", err)
	}

	if int64(ack.MMEUES1APID) != res.MMEUES1APID {
		return fmt.Errorf("acknowledge MME-UE-S1AP-ID = %d, want %d", ack.MMEUES1APID, res.MMEUES1APID)
	}

	if ack.SecurityContext.NextHopParameter == (s1ap.SecurityKey{}) {
		return fmt.Errorf("acknowledge carried an all-zero Next Hop")
	}

	return nil
}
