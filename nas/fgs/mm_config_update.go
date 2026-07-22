// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// ConfigurationUpdateCommand is the CONFIGURATION UPDATE COMMAND message
// (TS 24.501 §8.2.19): an optional configuration update indication, 5G-GUTI, and
// network names. The network names are supplied as their already-encoded IE
// value (TS 24.008 §10.5.3.5a).
type ConfigurationUpdateCommand struct {
	ConfigurationUpdateIndication *uint8 // optional (IEI 0xD), value bits 1-4
	GUTI                          []byte // optional (IEI 0x77): 5GS mobile identity value
	FullNameForNetwork            []byte // optional (IEI 0x43)
	ShortNameForNetwork           []byte // optional (IEI 0x45)
}

// Marshal encodes the plain CONFIGURATION UPDATE COMMAND message.
func (m *ConfigurationUpdateCommand) Marshal() ([]byte, error) {
	var w common.Writer

	writeMMHeader(&w, MsgConfigurationUpdateCommand)

	if m.ConfigurationUpdateIndication != nil {
		w.U8(ieiConfigUpdateInd | (*m.ConfigurationUpdateIndication & 0x0F))
	}

	if m.GUTI != nil {
		writeTLVE(&w, ieiGUTI5G, m.GUTI)
	}

	if m.FullNameForNetwork != nil {
		writeTLV(&w, ieiFullNameForNet, m.FullNameForNetwork)
	}

	if m.ShortNameForNetwork != nil {
		writeTLV(&w, ieiShortNameForNet, m.ShortNameForNetwork)
	}

	return w.Bytes(), nil
}
