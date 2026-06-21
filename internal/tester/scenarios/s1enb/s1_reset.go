// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const s1ResetIMSI = "001017271246614"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/s1_reset",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1Reset,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(s1ResetIMSI, "")},
			}
		},
	})
}

// runS1Reset attaches a UE, then has the eNB reset the whole S1 interface
// (TS 36.413 §8.7.1). The MME must answer with a RESET ACKNOWLEDGE carrying no
// connection list and drop the UE's S1 context. The association stays up — no
// fresh S1 Setup — so a subsequent attach on the same eNB proves the interface
// recovered.
func runS1Reset(_ context.Context, env scenarios.Env, _ any) error {
	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	ue := e.NewUE(s1ResetIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	if _, err := e.Attach(ue, 15*time.Second); err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	// Let the MME settle the UE into EMM-REGISTERED / ECM-CONNECTED before the
	// reset, so it resets an established context rather than racing the attach.
	time.Sleep(2 * time.Second)

	cause := s1ap.Cause{Group: s1ap.CauseGroupMisc, Value: 0}
	if err := e.SendReset(cause, true, nil); err != nil {
		return fmt.Errorf("send Reset: %w", err)
	}

	ack, err := e.WaitForResetAcknowledge(10 * time.Second)
	if err != nil {
		return fmt.Errorf("await Reset Acknowledge: %w", err)
	}

	if len(ack.ConnectionList) != 0 {
		return fmt.Errorf("whole-interface Reset Acknowledge carried a connection list: %+v", ack.ConnectionList)
	}

	logger.GnbLogger.Info("S1 interface reset acknowledged; re-attaching to prove recovery")

	reUE := e.NewUE(s1ResetIMSI, k, opc)
	reUE.RequestPDNType(env.PDUSessionType())

	res, err := e.Attach(reUE, 15*time.Second)
	if err != nil {
		return fmt.Errorf("re-attach after S1 reset: %w", err)
	}

	if err := e.Detach(reUE, res.MMEUES1APID, res.ENBUES1APID, 10*time.Second); err != nil {
		return fmt.Errorf("detach after re-attach: %w", err)
	}

	logger.GnbLogger.Info("S1 reset scenario completed")

	return nil
}
