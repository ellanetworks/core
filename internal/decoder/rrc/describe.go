// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rrc

import "encoding/hex"

type PDU struct {
	Protocol string      `json:"protocol"`
	RawHex   string      `json:"raw_hex"`
	Decoded  *Capability `json:"decoded"`
}

func describe(b []byte, parse func([]byte) (*Capability, error)) PDU {
	pdu := PDU{Protocol: "RRC", RawHex: hex.EncodeToString(b)}

	capability, err := parse(b)
	if err != nil {
		pdu.Decoded = &Capability{Error: err.Error()}

		return pdu
	}

	pdu.Decoded = capability

	return pdu
}

func DescribeNGAP(b []byte) PDU {
	return describe(b, ParseNGAPUERadioCapability)
}

func DescribeS1AP(b []byte) PDU {
	return describe(b, ParseS1APUERadioCapability)
}
