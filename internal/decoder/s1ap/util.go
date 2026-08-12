// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/s1ap"
)

func ieEnum(id s1ap.ProtocolIEID) utils.EnumField {
	name, ok := s1ap.ProtocolIEIDName(id)

	return utils.MakeEnum(uint16(id), name, !ok)
}

func procedureCodeToEnum(code s1ap.ProcedureCode) utils.EnumField {
	name, ok := s1ap.ProcedureCodeName(code)

	return utils.MakeEnum(int64(code), name, !ok)
}

func criticalityToEnum(c s1ap.Criticality) utils.EnumField {
	return utils.NamedEnum(uint8(c), c.Name())
}

// PLMNID is the MCC/MNC view of a 3-octet PLMN identity.
type PLMNID struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

// plmnToID decodes a PLMN identity (TS 24.008 §10.5.1.3 / TS 23.003 BCD nibble
// packing) into its MCC/MNC digits.
func plmnToID(p s1ap.PLMNIdentity) PLMNID {
	mcc := fmt.Sprintf("%d%d%d", p[0]&0x0f, p[0]>>4, p[1]&0x0f)

	var mnc string
	if p[1]>>4 == 0x0f { // 2-digit MNC: the third digit is filler
		mnc = fmt.Sprintf("%d%d", p[2]&0x0f, p[2]>>4)
	} else {
		mnc = fmt.Sprintf("%d%d%d", p[1]>>4, p[2]&0x0f, p[2]>>4)
	}

	return PLMNID{Mcc: mcc, Mnc: mnc}
}
