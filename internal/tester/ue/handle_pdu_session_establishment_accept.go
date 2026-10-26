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

	qfi, err := defaultQFI(acc)
	if err != nil {
		return err
	}

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
		UEIPv4:       ueIP,
		UEIPv6:       ueIPV6,
		MTU:          mtu,
		QFI:          qfi,
	})

	return nil
}

func defaultQFI(acc *fgs.PDUSessionEstablishmentAccept) (uint8, error) {
	for _, rule := range acc.QoSRules {
		if rule.DQR == 1 && rule.Parameters != nil {
			return rule.Parameters.QFI, nil
		}
	}

	if len(acc.QoSFlowDescriptions) < 1 {
		return 0, fmt.Errorf("not enough AuthorizedQosFlowDescriptions")
	}

	return acc.QoSFlowDescriptions[0].QFI, nil
}
