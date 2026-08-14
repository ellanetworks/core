// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
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
	xnPathSwitchIMSI        = "001017271246592"
	xnPathSwitchTargetRANID = int64(200)
	xnPathSwitchTargetTEID  = uint32(9200)
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/xn_path_switch",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runXnPathSwitch,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{
					scenarios.DefaultSubscriberWith(xnPathSwitchIMSI, ""),
				},
			}
		},
	})
}

func runXnPathSwitch(_ context.Context, env scenarios.Env, _ any) error {
	sourceGNB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer sourceGNB.Close()

	targetGNB, err := startXnTargetGNB(env, "000002", env.FirstGNB().N2Address, "")
	if err != nil {
		return err
	}

	defer targetGNB.Close()

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	newUE, err := newDefaultUE(sourceGNB, xnPathSwitchIMSI[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber, scenarios.DefaultPDUSessionTypeIPv4)
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

	sourceAmfUENGAPID := sourceGNB.GetAMFUENGAPID(ranUENGAPID)

	targetN3IP, err := netip.ParseAddr(env.FirstGNB().N3Address)
	if err != nil {
		return fmt.Errorf("parse target N3 address: %w", err)
	}

	ack, err := xnPathSwitch(targetGNB, &xnPathSwitchOpts{
		SourceAMFUENGAPID: sourceAmfUENGAPID,
		TargetRANUENGAPID: xnPathSwitchTargetRANID,
		TargetN3IP:        targetN3IP,
		TargetDLTEID:      xnPathSwitchTargetTEID,
	})
	if err != nil {
		return err
	}

	if ack.AMFUENGAPID == nil {
		return fmt.Errorf("acknowledge carried no AMF-UE-NGAP-ID, want %d", sourceAmfUENGAPID)
	}

	if int64(*ack.AMFUENGAPID) != sourceAmfUENGAPID {
		return fmt.Errorf("acknowledge AMF-UE-NGAP-ID = %d, want %d", *ack.AMFUENGAPID, sourceAmfUENGAPID)
	}

	if ack.RANUENGAPID == nil {
		return fmt.Errorf("acknowledge carried no RAN-UE-NGAP-ID, want %d", xnPathSwitchTargetRANID)
	}

	if int64(*ack.RANUENGAPID) != xnPathSwitchTargetRANID {
		return fmt.Errorf("acknowledge RAN-UE-NGAP-ID = %d, want the target's %d", *ack.RANUENGAPID, xnPathSwitchTargetRANID)
	}

	if ack.SecurityContext.NextHopNH == (ngaplib.SecurityKey{}) {
		return fmt.Errorf("acknowledge carried an all-zero Next Hop")
	}

	return nil
}

func startXnTargetGNB(env scenarios.Env, gnbID, n2Address, n3Address string) (*gnb.GnodeB, error) {
	targetGNB, err := gnb.Start(&gnb.StartOpts{
		GnbID:           gnbID,
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             scenarios.DefaultDNN,
		TAC:             scenarios.DefaultTAC,
		Name:            "Target-gNB",
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    n2Address,
		GnbN3Address:    n3Address,
	})
	if err != nil {
		return nil, fmt.Errorf("start target gNB: %w", err)
	}

	if _, err := targetGNB.WaitForMessage(gnb.Successful, ngaplib.ProcNGSetup, 2*time.Second); err != nil {
		targetGNB.Close()

		return nil, fmt.Errorf("target gNB: wait NGSetupResponse: %w", err)
	}

	return targetGNB, nil
}

type xnPathSwitchOpts struct {
	SourceAMFUENGAPID int64
	TargetRANUENGAPID int64
	TargetN3IP        netip.Addr
	TargetDLTEID      uint32
}

func xnPathSwitch(targetGNB *gnb.GnodeB, opts *xnPathSwitchOpts) (*ngaplib.PathSwitchRequestAcknowledge, error) {
	var sessions [16]*gnb.PDUSessionInformation

	sessions[scenarios.DefaultPDUSessionID] = &gnb.PDUSessionInformation{
		PDUSessionID: int64(scenarios.DefaultPDUSessionID),
		DLTeid:       opts.TargetDLTEID,
	}

	if err := targetGNB.SendPathSwitchRequest(&gnb.PathSwitchRequestOpts{
		RANUENGAPID:            opts.TargetRANUENGAPID,
		SourceAMFUENGAPID:      opts.SourceAMFUENGAPID,
		PDUSessions:            sessions,
		N3GnbIp:                opts.TargetN3IP,
		UESecurityCapabilities: xnUESecurityCapabilities(),
	}); err != nil {
		return nil, fmt.Errorf("send PathSwitchRequest: %w", err)
	}

	frame, err := targetGNB.WaitForMessage(gnb.Successful, ngaplib.ProcPathSwitchRequest, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("target gNB: wait PathSwitchRequestAcknowledge: %w", err)
	}

	ack, err := ngaplib.ParsePathSwitchRequestAcknowledge(frame.Value)
	if err != nil {
		return nil, fmt.Errorf("parse PathSwitchRequestAcknowledge: %w", err)
	}

	if len(ack.PDUSessionResourceReleased) != 0 {
		return nil, fmt.Errorf("the AMF released %d PDU session(s) instead of switching them", len(ack.PDUSessionResourceReleased))
	}

	if len(ack.PDUSessionResourceSwitchedList) != 1 {
		return nil, fmt.Errorf("acknowledge switched %d PDU sessions, want 1", len(ack.PDUSessionResourceSwitchedList))
	}

	if got := ack.PDUSessionResourceSwitchedList[0].PDUSessionID; int(got) != scenarios.DefaultPDUSessionID {
		return nil, fmt.Errorf("acknowledge switched PDU session %d, want %d", got, scenarios.DefaultPDUSessionID)
	}

	return ack, nil
}

func xnUESecurityCapabilities() ngaplib.UESecurityCapabilities {
	return ngaplib.UESecurityCapabilities{
		NREncryptionAlgorithms:          0x4000,
		NRIntegrityProtectionAlgorithms: 0x4000,
	}
}
