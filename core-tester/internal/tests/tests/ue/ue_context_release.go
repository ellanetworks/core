package ue

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/core-tester/internal/gnb"
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/ellanetworks/core/core-tester/internal/tests/engine"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/core"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/procedure"
	"github.com/ellanetworks/core/core-tester/internal/ue"
	"github.com/ellanetworks/core/core-tester/internal/ue/sidf"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

type UEContextRelease struct{}

func (UEContextRelease) Meta() engine.Meta {
	return engine.Meta{
		ID:      "ue/context/release",
		Summary: "UE context release test validating the Context Release Request and Response procedures",
		Timeout: 5 * time.Second,
	}
}

func (t UEContextRelease) Run(ctx context.Context, env engine.Env) error {
	ellaCoreEnv := core.NewEllaCoreEnv(env.EllaCoreClient, getDefaultEllaCoreConfig())

	err := ellaCoreEnv.Create(ctx)
	if err != nil {
		return fmt.Errorf("could not create EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Created EllaCore environment")

	gNodeB, err := gnb.Start(
		GNBID,
		DefaultMCC,
		DefaultMNC,
		DefaultSST,
		DefaultSD,
		DefaultDNN,
		DefaultTAC,
		"Ella-Core-Tester",
		env.Config.EllaCore.N2Address,
		env.Config.Gnb.N2Address,
		env.Config.Gnb.N3Address,
	)
	if err != nil {
		return fmt.Errorf("error starting gNB: %v", err)
	}

	defer gNodeB.Close()

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   PDUSessionID,
		PDUSessionType: PDUSessionType,
		Msin:           DefaultIMSI[5:],
		K:              DefaultKey,
		OpC:            DefaultOPC,
		Amf:            "80000000000000000000000000000000",
		Sqn:            DefaultSequenceNumber,
		Mcc:            DefaultMCC,
		Mnc:            DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.NullScheme,
			PublicKeyID:      "0",
		},
		RoutingIndicator: "0000",
		DNN:              DefaultDNN,
		Sst:              DefaultSST,
		Sd:               DefaultSD,
		IMEISV:           "3569380356438091",
		UeSecurityCapability: utils.GetUESecurityCapability(&utils.UeSecurityCapability{
			Integrity: utils.IntegrityAlgorithms{
				Nia2: true,
			},
			Ciphering: utils.CipheringAlgorithms{
				Nea0: true,
				Nea2: true,
			},
		}),
	})
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(RANUENGAPID, newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID: RANUENGAPID,
		UE:          newUE,
	})
	if err != nil {
		return fmt.Errorf("InitialRegistrationProcedure failed: %v", err)
	}

	pduSessionStatus := [16]bool{}
	pduSessionStatus[PDUSessionID] = true

	err = gNodeB.SendUEContextReleaseRequest(&gnb.UEContextReleaseRequestOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(RANUENGAPID),
		RANUENGAPID:   RANUENGAPID,
		PDUSessionIDs: pduSessionStatus,
		Cause:         ngapType.CauseRadioNetworkPresentReleaseDueToNgranGeneratedReason,
	})
	if err != nil {
		return fmt.Errorf("could not send UEContextReleaseComplete: %v", err)
	}

	logger.Logger.Debug(
		"Sent UE Context Release Request",
		zap.Int64("AMF UE NGAP ID", gNodeB.GetAMFUENGAPID(RANUENGAPID)),
		zap.Int64("RAN UE NGAP ID", RANUENGAPID),
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

	// Cleanup
	err = ellaCoreEnv.Delete(ctx)
	if err != nil {
		return fmt.Errorf("could not delete EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Deleted EllaCore environment")

	return nil
}

func validateUEContextReleaseCommand(fr gnb.SCTPFrame, ca *ngapType.Cause) error {
	err := utils.ValidateSCTP(fr.Info, 60, 1)
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
