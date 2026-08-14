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
	xnFailureIMSI     = "001017271246596"
	xnFailureRANID    = int64(202)
	xnFailureTEID     = uint32(9202)
	xnUnknownAMFUEGap = int64(10000)
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/xn_path_switch_failure",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runXnPathSwitchFailure,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{
					scenarios.DefaultSubscriberWith(xnFailureIMSI, ""),
				},
			}
		},
	})
}

func runXnPathSwitchFailure(_ context.Context, env scenarios.Env, _ any) error {
	sourceGNB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer sourceGNB.Close()

	targetGNB, err := startXnTargetGNB(env, env.FirstGNB().N2Address, "")
	if err != nil {
		return err
	}

	defer targetGNB.Close()

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	newUE, err := newDefaultUE(sourceGNB, xnFailureIMSI[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, scenarios.DefaultPDUSessionTypeIPv4)
	if err != nil {
		return fmt.Errorf("create UE: %w", err)
	}

	sourceGNB.AddUE(ranUENGAPID, newUE)

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	}); err != nil {
		return fmt.Errorf("initial registration: %w", err)
	}

	if _, err := sourceGNB.WaitForPDUSession(ranUENGAPID, int64(scenarios.DefaultPDUSessionID), 5*time.Second); err != nil {
		return fmt.Errorf("source gNB: wait PDU session: %w", err)
	}

	unknownAMFUENGAPID := sourceGNB.GetAMFUENGAPID(ranUENGAPID) + xnUnknownAMFUEGap

	targetN3IP, err := netip.ParseAddr(env.FirstGNB().N3Address)
	if err != nil {
		return fmt.Errorf("parse target N3 address: %w", err)
	}

	var sessions [16]*gnb.PDUSessionInformation

	sessions[scenarios.DefaultPDUSessionID] = &gnb.PDUSessionInformation{
		PDUSessionID: int64(scenarios.DefaultPDUSessionID),
		DLTeid:       xnFailureTEID,
	}

	if err := targetGNB.SendPathSwitchRequest(&gnb.PathSwitchRequestOpts{
		RANUENGAPID:            xnFailureRANID,
		SourceAMFUENGAPID:      unknownAMFUENGAPID,
		PDUSessions:            sessions,
		N3GnbIp:                targetN3IP,
		UESecurityCapabilities: xnUESecurityCapabilities(),
	}); err != nil {
		return fmt.Errorf("send PathSwitchRequest: %w", err)
	}

	frame, err := targetGNB.WaitForMessage(gnb.Unsuccessful, ngaplib.ProcPathSwitchRequest, 5*time.Second)
	if err != nil {
		return fmt.Errorf("the target gNB was not told the path switch failed: %w", err)
	}

	fail, err := ngaplib.ParsePathSwitchRequestFailure(frame.Value)
	if err != nil {
		return fmt.Errorf("parse PathSwitchRequestFailure: %w", err)
	}

	if fail.AMFUENGAPID == nil || int64(*fail.AMFUENGAPID) != unknownAMFUENGAPID {
		return fmt.Errorf("the failure AMF-UE-NGAP-ID = %v, want the requested %d", fail.AMFUENGAPID, unknownAMFUENGAPID)
	}

	if fail.RANUENGAPID == nil || int64(*fail.RANUENGAPID) != xnFailureRANID {
		return fmt.Errorf("the failure RAN-UE-NGAP-ID = %v, want the target's %d", fail.RANUENGAPID, xnFailureRANID)
	}

	if len(fail.PDUSessionResourceReleased) != 1 {
		return fmt.Errorf("the failure released %d PDU sessions, want 1", len(fail.PDUSessionResourceReleased))
	}

	released := fail.PDUSessionResourceReleased[0]
	if int(released.PDUSessionID) != scenarios.DefaultPDUSessionID {
		return fmt.Errorf("the failure released PDU session %d, want %d", released.PDUSessionID, scenarios.DefaultPDUSessionID)
	}

	transfer, err := ngaplib.ParsePathSwitchRequestUnsuccessfulTransfer(released.Transfer)
	if err != nil {
		return fmt.Errorf("parse PathSwitchRequestUnsuccessfulTransfer: %w", err)
	}

	if transfer.Cause.Group != ngaplib.CauseGroupRadioNetwork || transfer.Cause.Value != ngaplib.CauseRadioNetworkUnknownLocalUENGAPID {
		return fmt.Errorf("the failure cause = %+v, want a radio-network unknown-local-UE-NGAP-ID", transfer.Cause)
	}

	if _, err := sourceGNB.WaitForMessage(gnb.Initiating, ngaplib.ProcUEContextRelease, 500*time.Millisecond); err == nil {
		return errors.New("a path switch for an unknown UE released the real UE on the source gNB")
	}

	return nil
}
