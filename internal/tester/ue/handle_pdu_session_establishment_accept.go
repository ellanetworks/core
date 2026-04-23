package ue

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

func handlePDUSessionEstablishmentAccept(ue *UE, msg *nasMessage.PDUSessionEstablishmentAccept) error {
	ueIP, err := testutil.UEIPFromNAS(msg.GetPDUAddressInformation())
	if err != nil {
		return fmt.Errorf("could not get UE IP from NAS PDU Address Information: %v", err)
	}

	mtu, err := testutil.MTUFromExtendProtocolConfigurationOptionsContents(
		msg.GetExtendedProtocolConfigurationOptionsContents(),
	)
	if err != nil {
		return fmt.Errorf("could not get MTU from Extended Protocol Configuration Options: %v", err)
	}

	qosFlowDescs, err := testutil.ParseAuthorizedQosFlowDescriptions(
		msg.GetQoSFlowDescriptions(),
	)
	if err != nil {
		return fmt.Errorf("could not parse AuthorizedQosFlowDescriptions: %v", err)
	}

	if len(qosFlowDescs) < 1 {
		return fmt.Errorf("not enough AuthorizedQosFlowDescriptions: %v", err)
	}

	qfi := qosFlowDescs[0].Qfi

	logger.UeLogger.Debug(
		"Received PDU Session Establishment Accept NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
		zap.Uint8("PDU Session ID", msg.GetPDUSessionID()),
		zap.String("UE IP", ueIP.String()),
		zap.Uint16("MTU", mtu),
		zap.Uint8("QFI", qfi),
	)

	ue.SetPDUSession(PDUSessionInfo{
		PDUSessionID: msg.GetPDUSessionID(),
		UEIP:         ueIP.String(),
		MTU:          mtu,
		QFI:          qfi,
	})

	return nil
}
