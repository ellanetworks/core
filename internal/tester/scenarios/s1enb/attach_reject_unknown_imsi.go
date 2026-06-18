// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

// unknownIMSI is not provisioned (no subscriber fixture), so the MME cannot
// obtain an authentication vector for it.
const unknownIMSI = "001017271246601"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/attach_reject/unknown_imsi",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBAttachRejectUnknownIMSI,
	})
}

// runS1ENBAttachRejectUnknownIMSI attaches with an IMSI the core does not know
// and verifies the MME answers with ATTACH REJECT cause "IMSI unknown in HSS"
// (TS 24.301 §5.5.1.2.5) — the 4G counterpart of ue/registration_reject/unknown_ue.
func runS1ENBAttachRejectUnknownIMSI(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(unknownIMSI, k, opc)

	cause, err := e.AttachExpectReject(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	// EMM cause #2, IMSI unknown in HSS (TS 24.301 §9.9.3.9).
	if cause != 2 {
		return fmt.Errorf("expected Attach Reject cause #2 (IMSI unknown in HSS), got #%d", cause)
	}

	return nil
}
