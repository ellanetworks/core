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
	"github.com/ellanetworks/core/internal/tester/scenarios/common"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
)

const (
	n2HandoverIMSI = "001017271246590"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/ngap/n2_handover",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runN2Handover,
		Fixture:   fixtureN2Handover,
	})
}

func fixtureN2Handover(_ scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{
			scenarios.DefaultSubscriberWith(n2HandoverIMSI, ""),
		},
	}
}

func runN2Handover(_ context.Context, env scenarios.Env, _ any) error {
	if len(env.GNBs) < 2 {
		return fmt.Errorf("n2_handover requires at least 2 gNBs, got %d", len(env.GNBs))
	}

	sourceGNBSpec := env.GNBs[0]
	targetGNBSpec := env.GNBs[1]

	sourceGNB, err := gnb.Start(&gnb.StartOpts{
		GnbID:           "000001",
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             scenarios.DefaultDNN,
		TAC:             scenarios.DefaultTAC,
		Name:            "Source-gNB",
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    sourceGNBSpec.N2Address,
		GnbN3Address:    sourceGNBSpec.N3Address,
	})
	if err != nil {
		return fmt.Errorf("start source gNB: %w", err)
	}

	defer sourceGNB.Close()

	if _, err := sourceGNB.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentNGSetupResponse,
		2*time.Second,
	); err != nil {
		return fmt.Errorf("source gNB: wait NGSetupResponse: %w", err)
	}

	targetGNB, err := gnb.Start(&gnb.StartOpts{
		GnbID:           "000002",
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             scenarios.DefaultDNN,
		TAC:             scenarios.DefaultTAC,
		Name:            "Target-gNB",
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    targetGNBSpec.N2Address,
		GnbN3Address:    targetGNBSpec.N3Address,
	})
	if err != nil {
		return fmt.Errorf("start target gNB: %w", err)
	}

	defer targetGNB.Close()

	if _, err := targetGNB.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentNGSetupResponse,
		2*time.Second,
	); err != nil {
		return fmt.Errorf("target gNB: wait NGSetupResponse: %w", err)
	}

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	ue, err := newDefaultUEForHandover(sourceGNB, n2HandoverIMSI)
	if err != nil {
		return fmt.Errorf("create UE: %w", err)
	}

	sourceGNB.AddUE(ranUENGAPID, ue)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           ue,
	})
	if err != nil {
		return fmt.Errorf("initial registration: %w", err)
	}

	amfUENGAPID := sourceGNB.GetAMFUENGAPID(ranUENGAPID)

	err = sourceGNB.SendHandoverRequired(&gnb.HandoverRequiredOpts{
		AMFUENGAPID:  amfUENGAPID,
		RANUENGAPID:  ranUENGAPID,
		HandoverType: int64(ngapType.HandoverTypePresentIntra5gs),
		TargetGnbID:  "000002",
		PDUSessions: []gnb.HandoverRequiredPDUSession{
			{PDUSessionID: int64(scenarios.DefaultPDUSessionID)},
		},
	})
	if err != nil {
		return fmt.Errorf("send HandoverRequired: %w", err)
	}

	hoReqFrame, err := targetGNB.WaitForMessage(
		ngapType.NGAPPDUPresentInitiatingMessage,
		ngapType.InitiatingMessagePresentHandoverRequest,
		5*time.Second,
	)
	if err != nil {
		return fmt.Errorf("target gNB: wait HandoverRequest: %w", err)
	}

	targetAmfUENGAPID, err := common.ExtractAmfUeNgapIDFromHandoverRequest(hoReqFrame.Data)
	if err != nil {
		return fmt.Errorf("extract AMF UE NGAP ID from HandoverRequest: %w", err)
	}

	targetRanUENGAPID := int64(100)
	targetN3IP := netip.MustParseAddr(targetGNBSpec.N3Address)
	targetDLTEID := uint32(9000)

	err = targetGNB.SendHandoverRequestAcknowledge(&gnb.HandoverRequestAcknowledgeOpts{
		AMFUENGAPID: targetAmfUENGAPID,
		RANUENGAPID: targetRanUENGAPID,
		PDUSessions: []gnb.HandoverAdmittedPDUSession{
			{
				PDUSessionID: int64(scenarios.DefaultPDUSessionID),
				DLTeid:       targetDLTEID,
				DLIP:         targetN3IP,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("send HandoverRequestAcknowledge: %w", err)
	}

	_, err = sourceGNB.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentHandoverCommand,
		5*time.Second,
	)
	if err != nil {
		return fmt.Errorf("source gNB: wait HandoverCommand: %w", err)
	}

	err = targetGNB.SendHandoverNotify(&gnb.HandoverNotifyOpts{
		AMFUENGAPID: targetAmfUENGAPID,
		RANUENGAPID: targetRanUENGAPID,
	})
	if err != nil {
		return fmt.Errorf("send HandoverNotify: %w", err)
	}

	_, err = sourceGNB.WaitForMessage(
		ngapType.NGAPPDUPresentInitiatingMessage,
		ngapType.InitiatingMessagePresentUEContextReleaseCommand,
		5*time.Second,
	)
	if err != nil {
		return fmt.Errorf("source gNB: wait UEContextReleaseCommand: %w", err)
	}

	return nil
}

func newDefaultUEForHandover(gNodeB *gnb.GnodeB, imsi string) (*ue.UE, error) {
	msin := imsi[5:]

	return ue.NewUE(&ue.UEOpts{
		PDUSessionID:   scenarios.DefaultPDUSessionID,
		PDUSessionType: scenarios.DefaultPDUSessionTypeIPv4,
		GnodeB:         gNodeB,
		Msin:           msin,
		K:              scenarios.DefaultKey,
		OpC:            scenarios.DefaultOPC,
		Amf:            scenarios.DefaultAMF,
		Sqn:            scenarios.DefaultSequenceNumber,
		Mcc:            scenarios.DefaultMCC,
		Mnc:            scenarios.DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.NullScheme,
			PublicKeyID:      "0",
		},
		RoutingIndicator: scenarios.DefaultRoutingIndicator,
		DNN:              scenarios.DefaultDNN,
		Sst:              scenarios.DefaultSST,
		Sd:               scenarios.DefaultSD,
		IMEISV:           scenarios.DefaultIMEISV,
		UeSecurityCapability: testutil.GetUESecurityCapability(&testutil.UeSecurityCapability{
			Integrity: testutil.IntegrityAlgorithms{
				Nia2: true,
			},
			Ciphering: testutil.CipheringAlgorithms{
				Nea0: true,
				Nea2: true,
			},
		}),
	})
}
