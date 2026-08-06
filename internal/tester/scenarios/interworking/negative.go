// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

// mismatchDNN is a second data network the subscriber is entitled to, used to
// ask for a transfer onto a data network the session is not on.
const mismatchDNN = "enterprise"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:    "interworking/transfer_unknown_session",
		Run:     func(ctx context.Context, env scenarios.Env, _ any) error { return runTransferUnknownSession(ctx, env) },
		Fixture: fixture,
	})
	scenarios.Register(scenarios.Scenario{
		Name: "interworking/transfer_without_pdu_session_id",
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return runTransferWithoutPDUSessionID(ctx, env)
		},
		Fixture: fixture,
	})
	scenarios.Register(scenarios.Scenario{
		Name:    "interworking/transfer_data_network_mismatch",
		Run:     func(ctx context.Context, env scenarios.Env, _ any) error { return runTransferDNNMismatch(ctx, env) },
		Fixture: mismatchFixture,
	})
}

// mismatchFixture adds a second data network the subscriber may use, so a
// transfer can name one the session is not established on.
func mismatchFixture(env scenarios.Env) scenarios.FixtureSpec {
	spec := fixture(env)

	spec.DataNetworks = []scenarios.DataNetworkSpec{
		{Name: mismatchDNN, IPv4Pool: "10.46.0.0/16", DNS: scenarios.DefaultDNS, MTU: scenarios.DefaultMTU},
	}
	spec.Policies = []scenarios.PolicySpec{
		{
			Name:                mismatchDNN,
			ProfileName:         scenarios.DefaultProfileName,
			SliceName:           scenarios.DefaultSliceName,
			DataNetworkName:     mismatchDNN,
			SessionAmbrUplink:   "30 Mbps",
			SessionAmbrDownlink: "60 Mbps",
			Var5qi:              9,
			Arp:                 1,
		},
	}

	return spec
}

// expectESMReject attaches over EPS expecting the network to refuse the ESM
// procedure, and checks the cause pair. TS 24.301 §5.5.1.2.5 pairs EMM cause #19
// "ESM failure" with the ESM cause in the message container.
func expectESMReject(env scenarios.Env, apn string, pduSessionID uint8, want eps.ESMCause) error {
	e, err := startENB(env)
	if err != nil {
		return err
	}

	defer func() { _ = e.Close() }()

	epsUE, err := newEPSUE(e)
	if err != nil {
		return err
	}

	epsUE.RequestAPN(apn)
	epsUE.TransferPDUSession(pduSessionID)

	emmCause, esmCause, err := e.AttachExpectESMReject(epsUE, 15*time.Second)
	if err != nil {
		return fmt.Errorf("await the refusal: %w", err)
	}

	if emmCause != eps.EMMCauseESMFailure {
		return fmt.Errorf("EMM cause = %s, want #19 ESM failure", emmCause)
	}

	if esmCause != want {
		return fmt.Errorf("ESM cause = %s, want %s", esmCause, want)
	}

	return nil
}

// A transfer naming a PDU session the network does not hold is refused #54
// (TS 24.301 §6.5.1.6 b).
func runTransferUnknownSession(_ context.Context, env scenarios.Env) error {
	return expectESMReject(env, scenarios.DefaultDNN, transferPDUSessionID, eps.ESMCausePDNConnectionDoesNotExist)
}

// A transfer that names no PDU session identity cannot be correlated with a PDU
// session, so it is refused #54 as one that does not exist.
func runTransferWithoutPDUSessionID(_ context.Context, env scenarios.Env) error {
	return expectESMReject(env, scenarios.DefaultDNN, 0, eps.ESMCausePDNConnectionDoesNotExist)
}

// A transfer onto a data network the session is not established on is refused
// #27: the session exists, so #54 would invite the UE to discard state for a
// PDU session the network still holds (TS 24.501 annex B).
func runTransferDNNMismatch(_ context.Context, env scenarios.Env) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	fiveGUE, err := newFiveGUE(gNodeB, fgs.RequestTypeInitialRequest)
	if err != nil {
		return err
	}

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: transferPDUSessionID,
		UE:           fiveGUE,
	}); err != nil {
		return fmt.Errorf("establish over 5GS: %w", err)
	}

	return expectESMReject(env, mismatchDNN, transferPDUSessionID, eps.ESMCauseMissingOrUnknownAPN)
}
