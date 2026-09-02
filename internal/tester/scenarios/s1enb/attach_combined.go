// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/spf13/pflag"
)

const combinedAttachIMSI = "001017271246619"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/attach_combined",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBCombinedAttach,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(combinedAttachIMSI, "")},
			}
		},
	})
}

func runS1ENBCombinedAttach(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(combinedAttachIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())
	ue.RequestCombinedAttach()

	res, err := e.Attach(ue, attachTimeout)
	if err != nil {
		return fmt.Errorf("combined attach: %w", err)
	}

	if res.AttachResultValue != eps.AttachResultEPS {
		return fmt.Errorf("EPS attach result = %d, want EPS only (%d)", res.AttachResultValue, eps.AttachResultEPS)
	}

	if res.EMMCause == nil {
		return fmt.Errorf("combined attach accepted without an EMM cause; the UE is not told the CS domain is unavailable")
	}

	if *res.EMMCause != eps.EMMCauseCSDomainNotAvailable {
		return fmt.Errorf("EMM cause = %s, want CS domain not available (#%d)", *res.EMMCause, eps.EMMCauseCSDomainNotAvailable)
	}

	return assertAttach(res, familyExpect(env, scenarios.DefaultDNN, scenarios.DefaultUEIPv4Pool))
}
