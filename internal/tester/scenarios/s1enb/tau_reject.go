// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const tauRejectIMSI = "001017271246625"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/tau_reject/unknown_guti",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBTAUReject,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(tauRejectIMSI, "")},
			}
		},
	})
}

// A UE arriving with a GUTI this MME never issued cannot be identified, so the
// network rejects the update with EMM cause #9 rather than letting the UE wait
// out T3430, then releases the bare S1 connection that carried it
// (TS 24.301 §5.5.3.2.5).
func runS1ENBTAUReject(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(tauRejectIMSI, k, opc)

	res, err := e.TrackingAreaUpdateRejected(ue, s1enb.UnknownGUTI(), attachTimeout)
	if err != nil {
		return fmt.Errorf("tracking area update with an unknown GUTI: %w", err)
	}

	if res.EMMCause != eps.EMMCauseUEIdentityCannotBeDerived {
		return fmt.Errorf("tracking Area Update Reject cause = %s, want UE identity cannot be derived (#%d)",
			res.EMMCause, eps.EMMCauseUEIdentityCannotBeDerived)
	}

	if res.ReleaseCause.Group != s1ap.CauseGroupNAS || res.ReleaseCause.Value != s1ap.CauseNASUnspecified {
		return fmt.Errorf("UE Context Release Command cause = group %d value %d, want NAS unspecified",
			res.ReleaseCause.Group, res.ReleaseCause.Value)
	}

	return nil
}
