package gnb

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/core-tester/internal/gnb"
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/ellanetworks/core/core-tester/internal/tests/engine"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/core"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/procedure"
	"github.com/ellanetworks/core/core-tester/internal/ue"
	"github.com/ellanetworks/core/core-tester/internal/ue/sidf"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

const (
	xnHandoverRANUENGAPID       = int64(1)
	xnHandoverTargetRANUENGAPID = int64(2)
	xnHandoverPDUSessionID      = uint8(1)
	xnHandoverSourceGnbID       = "000001"
	xnHandoverTargetGnbID       = "000002"
)

type XnHandover struct{}

func (XnHandover) Meta() engine.Meta {
	return engine.Meta{
		ID:      "gnb/ngap/xn_handover",
		Summary: "Xn Handover test validating the path switch procedure between two gNBs",
		Timeout: 10 * time.Second,
	}
}

func (t XnHandover) Run(ctx context.Context, env engine.Env) error {
	ellaCoreEnv := core.NewEllaCoreEnv(env.EllaCoreClient, getDefaultXnHandoverEllaCoreConfig())

	err := ellaCoreEnv.Create(ctx)
	if err != nil {
		return fmt.Errorf("could not create EllaCore environment: %v", err)
	}

	defer func() {
		if delErr := ellaCoreEnv.Delete(ctx); delErr != nil {
			logger.Logger.Error("could not delete EllaCore environment", zap.Error(delErr))
		}
	}()

	logger.Logger.Debug("Created EllaCore environment")

	// Start source gNB
	sourceGnb, err := gnb.Start(
		xnHandoverSourceGnbID,
		DefaultMCC,
		DefaultMNC,
		DefaultSST,
		DefaultSD,
		DefaultDNN,
		DefaultTAC,
		"Source-gNB",
		env.Config.EllaCore.N2Address,
		env.Config.Gnb.N2Address,
		env.Config.Gnb.N3Address,
	)
	if err != nil {
		return fmt.Errorf("error starting source gNB: %v", err)
	}

	defer sourceGnb.Close()

	_, err = sourceGnb.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 1*time.Second)
	if err != nil {
		return fmt.Errorf("source gNB did not receive NG Setup Response: %v", err)
	}

	logger.Logger.Debug("Source gNB: NG Setup complete")

	// Start target gNB with its own N3 address
	targetGnb, err := gnb.Start(
		xnHandoverTargetGnbID,
		DefaultMCC,
		DefaultMNC,
		DefaultSST,
		DefaultSD,
		DefaultDNN,
		DefaultTAC,
		"Target-gNB",
		env.Config.EllaCore.N2Address,
		env.Config.Gnb.N2Address,
		env.Config.Gnb.N3AddressSecondary,
	)
	if err != nil {
		return fmt.Errorf("error starting target gNB: %v", err)
	}

	defer targetGnb.Close()

	_, err = targetGnb.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 1*time.Second)
	if err != nil {
		return fmt.Errorf("target gNB did not receive NG Setup Response: %v", err)
	}

	logger.Logger.Debug("Target gNB: NG Setup complete")

	// Create UE and register on source gNB
	newUE, err := ue.NewUE(&ue.UEOpts{
		PDUSessionID:   xnHandoverPDUSessionID,
		PDUSessionType: 1, // IPv4
		GnodeB:         sourceGnb,
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

	sourceGnb.AddUE(xnHandoverRANUENGAPID, newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID: xnHandoverRANUENGAPID,
		UE:          newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	logger.Logger.Debug("UE registered on source gNB")

	// Get the AMF UE NGAP ID and PDU session from the source gNB
	sourceAmfUeNgapID := sourceGnb.GetAMFUENGAPID(xnHandoverRANUENGAPID)
	sourcePduSession := sourceGnb.GetPDUSession(xnHandoverRANUENGAPID)

	if sourcePduSession == nil {
		return fmt.Errorf("source gNB has no PDU session for UE")
	}

	logger.Logger.Debug("Source gNB PDU session info",
		zap.Int64("AMF UE NGAP ID", sourceAmfUeNgapID),
		zap.Int64("PDU Session ID", sourcePduSession.PDUSessionID),
		zap.Uint32("UL TEID", sourcePduSession.ULTeid),
	)

	// Target gNB sends PathSwitchRequest to AMF
	// In Xn handover, the target gNB assigns a new RAN UE NGAP ID and new DL TEID,
	// and tells the AMF about the PDU sessions being switched over.
	targetDLTeid := uint32(100)

	targetN3Addr, err := netip.ParseAddr(env.Config.Gnb.N3AddressSecondary)
	if err != nil {
		return fmt.Errorf("could not parse target N3 address: %v", err)
	}

	// Build UE security capabilities for the path switch request
	ueSecurityCapabilities := &ngapType.UESecurityCapabilities{
		NRencryptionAlgorithms: ngapType.NRencryptionAlgorithms{
			Value: getBitStringFromUint16(0xC000), // NEA0 + NEA2
		},
		NRintegrityProtectionAlgorithms: ngapType.NRintegrityProtectionAlgorithms{
			Value: getBitStringFromUint16(0x4000), // NIA2
		},
		EUTRAencryptionAlgorithms: ngapType.EUTRAencryptionAlgorithms{
			Value: getBitStringFromUint16(0x0000),
		},
		EUTRAintegrityProtectionAlgorithms: ngapType.EUTRAintegrityProtectionAlgorithms{
			Value: getBitStringFromUint16(0x0000),
		},
	}

	err = targetGnb.SendPathSwitchRequest(&gnb.PathSwitchRequestOpts{
		RANUENGAPID:            xnHandoverTargetRANUENGAPID,
		SourceAMFUENGAPID:      sourceAmfUeNgapID,
		N3GnbIp:                targetN3Addr,
		UESecurityCapabilities: ueSecurityCapabilities,
		PDUSessions: [16]*gnb.PDUSessionInformation{
			{
				PDUSessionID: sourcePduSession.PDUSessionID,
				DLTeid:       targetDLTeid,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("could not send PathSwitchRequest: %v", err)
	}

	logger.Logger.Debug("Target gNB: sent PathSwitchRequest")

	// Wait for PathSwitchRequestAcknowledge from AMF
	ackFrame, err := targetGnb.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentPathSwitchRequestAcknowledge,
		2*time.Second,
	)
	if err != nil {
		return fmt.Errorf("target gNB did not receive PathSwitchRequestAcknowledge: %v", err)
	}

	logger.Logger.Debug("Target gNB: received PathSwitchRequestAcknowledge")

	// Validate the PathSwitchRequestAcknowledge
	err = validatePathSwitchRequestAcknowledge(ackFrame, xnHandoverTargetRANUENGAPID)
	if err != nil {
		return fmt.Errorf("PathSwitchRequestAcknowledge validation failed: %v", err)
	}

	return nil
}

func validatePathSwitchRequestAcknowledge(frame gnb.SCTPFrame, expectedRANUENGAPID int64) error {
	pdu, err := ngap.Decoder(frame.Data)
	if err != nil {
		return fmt.Errorf("could not decode NGAP: %v", err)
	}

	if pdu.SuccessfulOutcome == nil {
		return fmt.Errorf("NGAP PDU is not a SuccessfulOutcome")
	}

	if pdu.SuccessfulOutcome.ProcedureCode.Value != ngapType.ProcedureCodePathSwitchRequest {
		return fmt.Errorf("procedure code is not PathSwitchRequest (%d), got %d",
			ngapType.ProcedureCodePathSwitchRequest,
			pdu.SuccessfulOutcome.ProcedureCode.Value)
	}

	ack := pdu.SuccessfulOutcome.Value.PathSwitchRequestAcknowledge
	if ack == nil {
		return fmt.Errorf("PathSwitchRequestAcknowledge is nil")
	}

	var amfUeNgapID *ngapType.AMFUENGAPID

	var ranUeNgapID *ngapType.RANUENGAPID

	for _, ie := range ack.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			amfUeNgapID = ie.Value.AMFUENGAPID
		case ngapType.ProtocolIEIDRANUENGAPID:
			ranUeNgapID = ie.Value.RANUENGAPID
		}
	}

	if amfUeNgapID == nil {
		return fmt.Errorf("AMF UE NGAP ID is missing in PathSwitchRequestAcknowledge")
	}

	if ranUeNgapID == nil {
		return fmt.Errorf("RAN UE NGAP ID is missing in PathSwitchRequestAcknowledge")
	}

	if ranUeNgapID.Value != expectedRANUENGAPID {
		return fmt.Errorf("RAN UE NGAP ID mismatch: expected %d, got %d", expectedRANUENGAPID, ranUeNgapID.Value)
	}

	return nil
}

func getDefaultXnHandoverEllaCoreConfig() core.EllaCoreConfig {
	return core.EllaCoreConfig{
		Operator: core.OperatorConfig{
			ID: core.OperatorID{
				MCC: DefaultMCC,
				MNC: DefaultMNC,
			},
			Slice: core.OperatorSlice{
				SST: DefaultSST,
				SD:  DefaultSD,
			},
			Tracking: core.OperatorTracking{
				SupportedTACs: []string{DefaultTAC},
			},
		},
		DataNetworks: []core.DataNetworkConfig{
			{
				Name:   DefaultDNN,
				IPPool: "10.45.0.0/16",
				DNS:    "8.8.8.8",
				Mtu:    1500,
			},
		},
		Policies: []core.PolicyConfig{
			{
				Name:            "default",
				BitrateUplink:   "100 Mbps",
				BitrateDownlink: "100 Mbps",
				Var5qi:          9,
				Arp:             15,
				DataNetworkName: DefaultDNN,
			},
		},
		Subscribers: []core.SubscriberConfig{
			{
				Imsi:           DefaultIMSI,
				Key:            DefaultKey,
				SequenceNumber: DefaultSequenceNumber,
				OPc:            DefaultOPC,
				PolicyName:     "default",
			},
		},
	}
}

func getBitStringFromUint16(val uint16) aper.BitString {
	return aper.BitString{
		Bytes:     []byte{byte(val >> 8), byte(val)},
		BitLength: 16,
	}
}
