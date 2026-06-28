// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package validate

import (
	"encoding/binary"
	"fmt"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

type ExpectedPDUSessionModificationCommand struct {
	// 0 skips Session-AMBR uplink validation.
	AmbrUplinkKbps uint64
	// 0 skips Session-AMBR downlink validation.
	AmbrDownlinkKbps uint64
}

func PDUSessionModificationCommand(msg *nas.Message, expected *ExpectedPDUSessionModificationCommand) error {
	if msg == nil {
		return fmt.Errorf("NAS message is nil")
	}

	msgType := msg.GsmHeader.GetMessageType()
	if msgType != nas.MsgTypePDUSessionModificationCommand {
		return fmt.Errorf("expected PDU Session Modification Command (0x%02x), got 0x%02x", nas.MsgTypePDUSessionModificationCommand, msgType)
	}

	modCmd := msg.PDUSessionModificationCommand
	if modCmd == nil {
		return fmt.Errorf("PDUSessionModificationCommand is nil in NAS message")
	}

	if expected.AmbrUplinkKbps > 0 || expected.AmbrDownlinkKbps > 0 {
		if modCmd.SessionAMBR == nil {
			return fmt.Errorf("expected Session-AMBR in Modification Command but it was absent")
		}

		ulUnit := modCmd.GetUnitForSessionAMBRForUplink()
		ulValue := modCmd.GetSessionAMBRForUplink()
		ulKbps := ambrToKbps(ulUnit, ulValue)

		dlUnit := modCmd.GetUnitForSessionAMBRForDownlink()
		dlValue := modCmd.GetSessionAMBRForDownlink()
		dlKbps := ambrToKbps(dlUnit, dlValue)

		if expected.AmbrUplinkKbps > 0 && ulKbps != expected.AmbrUplinkKbps {
			return fmt.Errorf("Session-AMBR uplink mismatch: got %d Kbps, expected %d Kbps", ulKbps, expected.AmbrUplinkKbps)
		}

		if expected.AmbrDownlinkKbps > 0 && dlKbps != expected.AmbrDownlinkKbps {
			return fmt.Errorf("Session-AMBR downlink mismatch: got %d Kbps, expected %d Kbps", dlKbps, expected.AmbrDownlinkKbps)
		}
	}

	return nil
}

func ambrToKbps(unit uint8, value [2]uint8) uint64 {
	raw := uint64(binary.BigEndian.Uint16(value[:]))

	switch unit {
	case nasMessage.SessionAMBRUnit1Kbps:
		return raw
	case nasMessage.SessionAMBRUnit4Kbps:
		return raw * 4
	case nasMessage.SessionAMBRUnit16Kbps:
		return raw * 16
	case nasMessage.SessionAMBRUnit64Kbps:
		return raw * 64
	case nasMessage.SessionAMBRUnit256Kbps:
		return raw * 256
	case nasMessage.SessionAMBRUnit1Mbps:
		return raw * 1000
	case nasMessage.SessionAMBRUnit4Mbps:
		return raw * 4000
	case nasMessage.SessionAMBRUnit16Mbps:
		return raw * 16000
	case nasMessage.SessionAMBRUnit64Mbps:
		return raw * 64000
	case nasMessage.SessionAMBRUnit256Mbps:
		return raw * 256000
	case nasMessage.SessionAMBRUnit1Gbps:
		return raw * 1000000
	case nasMessage.SessionAMBRUnit4Gbps:
		return raw * 4000000
	case nasMessage.SessionAMBRUnit16Gbps:
		return raw * 16000000
	case nasMessage.SessionAMBRUnit64Gbps:
		return raw * 64000000
	default:
		return 0
	}
}
