// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const (
	x2FailureIMSI     = "001017271246684"
	x2UnknownMMEUEGap = int64(10000)
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/x2_path_switch_failure",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runX2PathSwitchFailure,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(x2FailureIMSI, "")},
			}
		},
	})
}

func runX2PathSwitchFailure(_ context.Context, env scenarios.Env, _ any) error {
	pair, closePair, err := startS1HandoverPair(env, x2FailureIMSI)
	if err != nil {
		return err
	}

	defer closePair()

	targetENBUEID := pair.Target.AllocateENBUEID()
	unknownMMEUEID := pair.Attached.MMEUES1APID + x2UnknownMMEUEGap

	if _, err := pair.Target.SendPathSwitchRequest(targetENBUEID, unknownMMEUEID, pair.Attached.ERABID, pair.UE.S1APSecurityCapabilities()); err != nil {
		return fmt.Errorf("send Path Switch Request: %w", err)
	}

	fail, err := pair.Target.WaitForPathSwitchRequestFailure(targetENBUEID, s1FailureTimeout)
	if err != nil {
		return fmt.Errorf("the target eNB was not told the path switch failed: %w", err)
	}

	if fail.MMEUES1APID == nil || int64(*fail.MMEUES1APID) != unknownMMEUEID {
		return fmt.Errorf("the failure MME-UE-S1AP-ID = %v, want the requested %d", fail.MMEUES1APID, unknownMMEUEID)
	}

	if fail.ENBUES1APID == nil || int64(*fail.ENBUES1APID) != targetENBUEID {
		return fmt.Errorf("the failure eNB-UE-S1AP-ID = %v, want the target's %d", fail.ENBUES1APID, targetENBUEID)
	}

	if fail.Cause == nil {
		return errors.New("the Path Switch Request Failure carries no cause")
	}

	if fail.Cause.Group != s1ap.CauseGroupRadioNetwork || fail.Cause.Value != s1ap.CauseRadioNetworkUnknownMMEUES1APID {
		return fmt.Errorf("the failure cause = %+v, want a radio-network unknown-MME-UE-S1AP-ID", *fail.Cause)
	}

	if _, err := pair.Source.WaitForUEContextReleaseCommand(pair.Attached.ENBUES1APID, 500*time.Millisecond); err == nil {
		return errors.New("a path switch for an unknown UE released the real UE on the source eNB")
	}

	return nil
}
