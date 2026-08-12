// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas"
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
	plmn, err := nas.ParsePLMN([3]byte(p))
	if err != nil {
		return PLMNID{}
	}

	return PLMNID{Mcc: plmn.MCC, Mnc: plmn.MNC}
}
