package ue

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ue/context/release",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runUEContextRelease(ctx, env, params)
		},
	})
}

func runUEContextRelease(_ context.Context, env scenarios.Env, _ any) error {
	g := env.FirstGNB()

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:         scenarios.DefaultGNBID,
		MCC:           scenarios.DefaultMCC,
		MNC:           scenarios.DefaultMNC,
		SST:           scenarios.DefaultSST,
		SD:            scenarios.DefaultSD,
		DNN:           scenarios.DefaultDNN,
		TAC:           scenarios.DefaultTAC,
		Name:          "Ella-Core-Tester",
		CoreN2Address: env.FirstCore(),
		GnbN2Address:  g.N2Address,
		GnbN3Address:  g.N3Address,
	})
	if err != nil {
		return fmt.Errorf("error starting gNB: %v", err)
	}

	defer gNodeB.Close()

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	newUE, err := newDefaultUE(gNodeB, scenarios.DefaultIMSI[5:], scenarios.DefaultKey, scenarios.DefaultOPC, scenarios.DefaultSequenceNumber)
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(int64(scenarios.DefaultRANUENGAPID), newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  int64(scenarios.DefaultRANUENGAPID),
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	})
	if err != nil {
		return fmt.Errorf("InitialRegistrationProcedure failed: %v", err)
	}

	pduSessionStatus := [16]bool{}
	pduSessionStatus[scenarios.DefaultPDUSessionID] = true

	err = gNodeB.SendUEContextReleaseRequest(&gnb.UEContextReleaseRequestOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(int64(scenarios.DefaultRANUENGAPID)),
		RANUENGAPID:   int64(scenarios.DefaultRANUENGAPID),
		PDUSessionIDs: pduSessionStatus,
		Cause:         ngapType.CauseRadioNetworkPresentReleaseDueToNgranGeneratedReason,
	})
	if err != nil {
		return fmt.Errorf("could not send UEContextReleaseComplete: %v", err)
	}

	logger.Logger.Debug(
		"Sent UE Context Release Request",
		zap.Int64("AMF UE NGAP ID", gNodeB.GetAMFUENGAPID(int64(scenarios.DefaultRANUENGAPID))),
		zap.Int64("RAN UE NGAP ID", int64(scenarios.DefaultRANUENGAPID)),
		zap.String("Cause", "ReleaseDueToNgranGeneratedReason"),
	)

	fr, err := gNodeB.WaitForMessage(ngapType.NGAPPDUPresentInitiatingMessage, ngapType.InitiatingMessagePresentUEContextReleaseCommand, 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	err = validateUEContextReleaseCommand(fr, &ngapType.Cause{
		Present: ngapType.CausePresentRadioNetwork,
		RadioNetwork: &ngapType.CauseRadioNetwork{
			Value: ngapType.CauseRadioNetworkPresentReleaseDueToNgranGeneratedReason,
		},
	},
	)
	if err != nil {
		return fmt.Errorf("UEContextRelease validation failed: %v", err)
	}

	return nil
}

func validateUEContextReleaseCommand(fr gnb.SCTPFrame, ca *ngapType.Cause) error {
	err := testutil.ValidateSCTP(fr.Info, 60, 1)
	if err != nil {
		return fmt.Errorf("SCTP validation failed: %v", err)
	}

	pdu, err := ngap.Decoder(fr.Data)
	if err != nil {
		return fmt.Errorf("could not decode NGAP: %v", err)
	}

	if pdu.InitiatingMessage == nil {
		return fmt.Errorf("NGAP PDU is not a InitiatingMessage")
	}

	if pdu.InitiatingMessage.ProcedureCode.Value != ngapType.ProcedureCodeUEContextRelease {
		return fmt.Errorf("NGAP ProcedureCode is not UEContextRelease (%d), received %d", ngapType.ProcedureCodeUEContextRelease, pdu.InitiatingMessage.ProcedureCode.Value)
	}

	ueContextReleaseCommand := pdu.InitiatingMessage.Value.UEContextReleaseCommand
	if ueContextReleaseCommand == nil {
		return fmt.Errorf("UE Context Release Command is nil")
	}

	var (
		ueNGAPIDs *ngapType.UENGAPIDs
		cause     *ngapType.Cause
	)

	for _, ie := range ueContextReleaseCommand.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDUENGAPIDs:
			ueNGAPIDs = ie.Value.UENGAPIDs
		case ngapType.ProtocolIEIDCause:
			cause = ie.Value.Cause
		default:
			return fmt.Errorf("UEContextReleaseCommand IE ID (%d) not supported", ie.Id.Value)
		}
	}

	if cause.Present != ca.Present {
		return fmt.Errorf("unexpected Cause Present: got %d, want %d", cause.Present, ca.Present)
	}

	switch cause.Present {
	case ngapType.CausePresentRadioNetwork:
		if cause.RadioNetwork.Value != ca.RadioNetwork.Value {
			return fmt.Errorf("unexpected RadioNetwork Cause value: got %d, want %d", cause.RadioNetwork.Value, ca.RadioNetwork.Value)
		}
	case ngapType.CausePresentNas:
		if cause.Nas.Value != ca.Nas.Value {
			return fmt.Errorf("unexpected NAS Cause value: got %d, want %d", cause.Nas.Value, ca.Nas.Value)
		}
	default:
		return fmt.Errorf("unexpected Cause Present type: %d", cause.Present)
	}

	if ueNGAPIDs == nil {
		return fmt.Errorf("UENGAPIDs is nil")
	}

	return nil
}
