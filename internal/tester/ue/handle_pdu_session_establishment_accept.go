package ue

import (
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

func handlePDUSessionEstablishmentAccept(ue *UE, msg *nasMessage.PDUSessionEstablishmentAccept) error {
	pduAddrInfo := msg.GetPDUAddressInformation()
	pduSessionType := msg.SelectedSSCModeAndSelectedPDUSessionType.Octet & 0x07

	ueIP, err := testutil.UEIPFromNAS(pduAddrInfo)
	if err != nil {
		return fmt.Errorf("could not get UE IP from NAS PDU Address Information: %v", err)
	}

	var ueIPV6 string

	if pduSessionType == nasMessage.PDUSessionTypeIPv6 || pduSessionType == nasMessage.PDUSessionTypeIPv4IPv6 {
		var ifaceId [8]uint8
		copy(ifaceId[:], pduAddrInfo[0:8])
		ueIPV6 = interfaceIdToLinkLocal(ifaceId)
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
		zap.String("UE IPv6", ueIPV6),
		zap.Uint16("MTU", mtu),
		zap.Uint8("QFI", qfi),
	)

	ue.SetPDUSession(PDUSessionInfo{
		PDUSessionID: msg.GetPDUSessionID(),
		UEIP:         ueIP.String(),
		UEIPV6:       ueIPV6,
		MTU:          mtu,
		QFI:          qfi,
	})

	return nil
}

func interfaceIdToLinkLocal(interfaceId [8]uint8) string {
	linkLocalPrefix := [8]uint8{0xfe, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	var addr [16]byte
	copy(addr[0:8], linkLocalPrefix[:])
	copy(addr[8:16], interfaceId[:])

	return netip.AddrFrom16(addr).String()
}
