// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"strings"

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

func namedEnum[T ~int | ~int32 | ~int64 | ~uint8 | ~uint16 | ~uint32 | ~uint64](v T, name string) utils.EnumField {
	if strings.HasPrefix(name, "unknown(") {
		name = ""
	}

	return utils.NamedEnum(v, name)
}

func derefSlice[S ~[]E, E any](p *S) S {
	if p == nil {
		return nil
	}

	return *p
}

func criticalityToEnum(c s1ap.Criticality) utils.EnumField {
	return namedEnum(uint8(c), c.Name())
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
