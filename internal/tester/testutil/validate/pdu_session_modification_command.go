// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package validate

import (
	"fmt"

	"github.com/ellanetworks/core/nas/fgs"
)

type ExpectedPDUSessionModificationCommand struct {
	// 0 skips Session-AMBR uplink validation.
	AmbrUplinkKbps uint64
	// 0 skips Session-AMBR downlink validation.
	AmbrDownlinkKbps uint64
}

func PDUSessionModificationCommand(plain []byte, expected *ExpectedPDUSessionModificationCommand) error {
	if len(plain) < 4 {
		return fmt.Errorf("NAS message is too short")
	}

	if fgs.GSMMessageType(plain[3]) != fgs.MsgPDUSessionModificationCommand {
		return fmt.Errorf("expected PDU Session Modification Command (0x%02x), got 0x%02x", uint8(fgs.MsgPDUSessionModificationCommand), plain[3])
	}

	modCmd, err := fgs.ParsePDUSessionModificationCommand(plain)
	if err != nil {
		return fmt.Errorf("could not parse PDU Session Modification Command: %v", err)
	}

	if expected.AmbrUplinkKbps > 0 || expected.AmbrDownlinkKbps > 0 {
		if modCmd.SessionAMBR == nil {
			return fmt.Errorf("expected Session-AMBR in Modification Command but it was absent")
		}

		ambr := *modCmd.SessionAMBR

		ulKbps := ambrToKbps(uint8(ambr.UplinkUnit), ambr.Uplink)
		dlKbps := ambrToKbps(uint8(ambr.DownlinkUnit), ambr.Downlink)

		if expected.AmbrUplinkKbps > 0 && ulKbps != expected.AmbrUplinkKbps {
			return fmt.Errorf("Session-AMBR uplink mismatch: got %d Kbps, expected %d Kbps", ulKbps, expected.AmbrUplinkKbps)
		}

		if expected.AmbrDownlinkKbps > 0 && dlKbps != expected.AmbrDownlinkKbps {
			return fmt.Errorf("Session-AMBR downlink mismatch: got %d Kbps, expected %d Kbps", dlKbps, expected.AmbrDownlinkKbps)
		}
	}

	return nil
}

// ambrToKbps converts a Session-AMBR value and unit to Kbps (TS 24.501 §9.11.4.14).
func ambrToKbps(unit uint8, raw uint16) uint64 {
	v := uint64(raw)

	switch unit {
	case 0x01: // 1 Kbps
		return v
	case 0x02: // 4 Kbps
		return v * 4
	case 0x03: // 16 Kbps
		return v * 16
	case 0x04: // 64 Kbps
		return v * 64
	case 0x05: // 256 Kbps
		return v * 256
	case 0x06: // 1 Mbps
		return v * 1000
	case 0x07: // 4 Mbps
		return v * 4000
	case 0x08: // 16 Mbps
		return v * 16000
	case 0x09: // 64 Mbps
		return v * 64000
	case 0x0A: // 256 Mbps
		return v * 256000
	case 0x0B: // 1 Gbps
		return v * 1000000
	case 0x0C: // 4 Gbps
		return v * 4000000
	case 0x0D: // 16 Gbps
		return v * 16000000
	case 0x0E: // 64 Gbps
		return v * 64000000
	default:
		return 0
	}
}
