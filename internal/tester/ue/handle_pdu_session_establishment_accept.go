// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func handlePDUSessionEstablishmentAccept(ue *UE, payload []byte) error {
	acc, err := fgs.ParsePDUSessionEstablishmentAccept(payload)
	if err != nil {
		return fmt.Errorf("could not parse PDU Session Establishment Accept: %v", err)
	}

	if acc.PDUAddress == nil {
		return fmt.Errorf("PDU Session Establishment Accept carries no PDU address")
	}

	var ueIP string
	if v4 := acc.PDUAddress.IPv4Addr(); v4.IsValid() {
		ueIP = v4.String()
	}

	var ueIPV6 string
	if v6 := acc.PDUAddress.IPv6LinkLocal(); v6.IsValid() {
		ueIPV6 = v6.String()
	}

	var mtu uint16
	if acc.ExtendedPCO != nil {
		mtu, _ = acc.ExtendedPCO.IPv4LinkMTU()
	}

	qosFlowDescs := acc.QoSFlowDescriptions
	if len(qosFlowDescs) < 1 {
		return fmt.Errorf("not enough AuthorizedQosFlowDescriptions")
	}

	qfi := qosFlowDescs[0].QFI

	logger.UeLogger.Debug(
		"Received PDU Session Establishment Accept NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
		zap.Uint8("PDU Session ID", uint8(acc.PDUSessionID)),
		zap.String("UE IP", ueIP),
		zap.String("UE IPv6", ueIPV6),
		zap.Uint16("MTU", mtu),
		zap.Uint8("QFI", qfi),
	)

	ue.SetPDUSession(PDUSessionInfo{
		PDUSessionID: uint8(acc.PDUSessionID),
		UEIP:         ueIP,
		UEIPV6:       ueIPV6,
		MTU:          mtu,
		QFI:          qfi,
	})

	return nil
}
