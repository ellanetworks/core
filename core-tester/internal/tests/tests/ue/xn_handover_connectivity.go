package ue

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/netip"
	"os/exec"
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
	xnHandoverSourceGnbID       = "000001"
	xnHandoverTargetGnbID       = "000002"
	xnHandoverSourceRANUENGAPID = int64(1)
	xnHandoverTargetRANUENGAPID = int64(2)
)

type XnHandoverConnectivity struct{}

func (XnHandoverConnectivity) Meta() engine.Meta {
	return engine.Meta{
		ID:      "ue/xn_handover_connectivity",
		Summary: "Xn Handover connectivity test validating ping works before and after path switch between two gNBs",
		Timeout: 30 * time.Second,
	}
}

func (t XnHandoverConnectivity) Run(ctx context.Context, env engine.Env) error {
	ellaCoreEnv := core.NewEllaCoreEnv(env.EllaCoreClient, getDefaultEllaCoreConfig())

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

	// Start source gNB with N3 for GTP-U
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
		PDUSessionID:   PDUSessionID,
		PDUSessionType: PDUSessionType,
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

	sourceGnb.AddUE(xnHandoverSourceRANUENGAPID, newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID: xnHandoverSourceRANUENGAPID,
		UE:          newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	logger.Logger.Debug("UE registered on source gNB",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.Int64("RAN UE NGAP ID", xnHandoverSourceRANUENGAPID),
		zap.Int64("AMF UE NGAP ID", sourceGnb.GetAMFUENGAPID(xnHandoverSourceRANUENGAPID)),
	)

	// Get PDU session info
	uePduSession := newUE.GetPDUSession()
	ueIP := uePduSession.UEIP + "/16"

	sourceGnbPDUSession, err := sourceGnb.WaitForPDUSession(xnHandoverSourceRANUENGAPID, 5*time.Second)
	if err != nil {
		return fmt.Errorf("source gNB has no PDU session: %v", err)
	}

	// Create GTP tunnel on source gNB for pre-handover connectivity
	preTunName := GTPInterfaceNamePrefix + "ho0"

	_, err = sourceGnb.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP,
		UpfIP:            sourceGnbPDUSession.UpfAddress,
		TunInterfaceName: preTunName,
		ULteid:           sourceGnbPDUSession.ULTeid,
		DLteid:           sourceGnbPDUSession.DLTeid,
		MTU:              uePduSession.MTU,
		QFI:              uePduSession.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create pre-handover GTP tunnel: %v", err)
	}

	logger.GnbLogger.Debug("Created pre-handover GTP tunnel",
		zap.String("Interface", preTunName),
		zap.String("UE IP", ueIP),
		zap.String("UPF IP", sourceGnbPDUSession.UpfAddress),
		zap.Uint32("UL TEID", sourceGnbPDUSession.ULTeid),
		zap.Uint32("DL TEID", sourceGnbPDUSession.DLTeid),
	)

	// Ping BEFORE handover
	cmd := exec.CommandContext(ctx, "ping", "-I", preTunName, env.Config.PingDestination, "-c", "3", "-W", "1")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ping before handover failed: %v\noutput:\n%s", err, string(out))
	}

	logger.Logger.Debug("Ping successful BEFORE handover",
		zap.String("interface", preTunName),
		zap.String("destination", env.Config.PingDestination),
	)

	// Close pre-handover tunnel
	err = sourceGnb.CloseTunnel(sourceGnbPDUSession.DLTeid)
	if err != nil {
		return fmt.Errorf("could not close pre-handover GTP tunnel: %v", err)
	}

	// Perform Xn Handover via PathSwitchRequest
	sourceAmfUeNgapID := sourceGnb.GetAMFUENGAPID(xnHandoverSourceRANUENGAPID)
	targetDLTeid := sourceGnb.GenerateTEID()

	targetN3Addr, err := netip.ParseAddr(env.Config.Gnb.N3AddressSecondary)
	if err != nil {
		return fmt.Errorf("could not parse target N3 address: %v", err)
	}

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
				PDUSessionID: sourceGnbPDUSession.PDUSessionID,
				DLTeid:       targetDLTeid,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("could not send PathSwitchRequest: %v", err)
	}

	logger.Logger.Debug("Target gNB: sent PathSwitchRequest")

	// Wait for PathSwitchRequestAcknowledge
	ackFrame, err := targetGnb.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentPathSwitchRequestAcknowledge,
		2*time.Second,
	)
	if err != nil {
		return fmt.Errorf("target gNB did not receive PathSwitchRequestAcknowledge: %v", err)
	}

	logger.Logger.Debug("Target gNB: received PathSwitchRequestAcknowledge")

	// Parse the PathSwitchRequestAcknowledge to get the new UPF UL tunnel info
	newULTeid, newUpfAddress, err := parsePathSwitchRequestAcknowledge(ackFrame, sourceGnbPDUSession.PDUSessionID)
	if err != nil {
		return fmt.Errorf("could not parse PathSwitchRequestAcknowledge: %v", err)
	}

	logger.Logger.Debug("Parsed PathSwitchRequestAcknowledge transfer",
		zap.Uint32("New UL TEID", newULTeid),
		zap.String("New UPF Address", newUpfAddress),
		zap.Uint32("Target DL TEID", targetDLTeid),
	)

	// Create GTP tunnel on the target gNB for post-handover connectivity
	postTunName := GTPInterfaceNamePrefix + "ho1"

	_, err = targetGnb.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP,
		UpfIP:            newUpfAddress,
		TunInterfaceName: postTunName,
		ULteid:           newULTeid,
		DLteid:           targetDLTeid,
		MTU:              uePduSession.MTU,
		QFI:              uePduSession.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create post-handover GTP tunnel: %v", err)
	}

	logger.GnbLogger.Debug("Created post-handover GTP tunnel",
		zap.String("Interface", postTunName),
		zap.String("UE IP", ueIP),
		zap.String("UPF IP", newUpfAddress),
		zap.Uint32("UL TEID", newULTeid),
		zap.Uint32("DL TEID", targetDLTeid),
	)

	// Ping AFTER handover
	cmd = exec.CommandContext(ctx, "ping", "-I", postTunName, env.Config.PingDestination, "-c", "3", "-W", "1")

	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ping after handover failed: %v\noutput:\n%s", err, string(out))
	}

	logger.Logger.Debug("Ping successful AFTER handover",
		zap.String("interface", postTunName),
		zap.String("destination", env.Config.PingDestination),
	)

	// Cleanup
	err = targetGnb.CloseTunnel(targetDLTeid)
	if err != nil {
		return fmt.Errorf("could not close post-handover GTP tunnel: %v", err)
	}

	return nil
}

// parsePathSwitchRequestAcknowledge extracts the UPF UL tunnel information
// (TEID + IP address) from the PathSwitchRequestAcknowledge for the given PDU session.
func parsePathSwitchRequestAcknowledge(frame gnb.SCTPFrame, expectedPDUSessionID int64) (uint32, string, error) {
	pdu, err := ngap.Decoder(frame.Data)
	if err != nil {
		return 0, "", fmt.Errorf("could not decode NGAP: %v", err)
	}

	if pdu.SuccessfulOutcome == nil {
		return 0, "", fmt.Errorf("NGAP PDU is not a SuccessfulOutcome")
	}

	ack := pdu.SuccessfulOutcome.Value.PathSwitchRequestAcknowledge
	if ack == nil {
		return 0, "", fmt.Errorf("PathSwitchRequestAcknowledge is nil")
	}

	for _, ie := range ack.ProtocolIEs.List {
		if ie.Id.Value != ngapType.ProtocolIEIDPDUSessionResourceSwitchedList {
			continue
		}

		switchedList := ie.Value.PDUSessionResourceSwitchedList
		if switchedList == nil {
			return 0, "", fmt.Errorf("PDUSessionResourceSwitchedList is nil")
		}

		for _, item := range switchedList.List {
			if item.PDUSessionID.Value != expectedPDUSessionID {
				continue
			}

			transfer := &ngapType.PathSwitchRequestAcknowledgeTransfer{}

			err := aper.UnmarshalWithParams(item.PathSwitchRequestAcknowledgeTransfer, transfer, "valueExt")
			if err != nil {
				return 0, "", fmt.Errorf("could not unmarshal PathSwitchRequestAcknowledgeTransfer: %v", err)
			}

			if transfer.ULNGUUPTNLInformation == nil {
				return 0, "", fmt.Errorf("UL NG-U UP TNL Information is missing in transfer")
			}

			if transfer.ULNGUUPTNLInformation.GTPTunnel == nil {
				return 0, "", fmt.Errorf("GTP Tunnel is missing in UL NG-U UP TNL Information")
			}

			teid := binary.BigEndian.Uint32(transfer.ULNGUUPTNLInformation.GTPTunnel.GTPTEID.Value)
			ipBytes := transfer.ULNGUUPTNLInformation.GTPTunnel.TransportLayerAddress.Value.Bytes

			upfAddress := fmt.Sprintf("%d.%d.%d.%d", ipBytes[0], ipBytes[1], ipBytes[2], ipBytes[3])

			return teid, upfAddress, nil
		}
	}

	return 0, "", fmt.Errorf("PDU session ID %d not found in PathSwitchRequestAcknowledge switched list", expectedPDUSessionID)
}

func getBitStringFromUint16(val uint16) aper.BitString {
	return aper.BitString{
		Bytes:     []byte{byte(val >> 8), byte(val)},
		BitLength: 16,
	}
}
