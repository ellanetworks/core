// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/ngap"
)

type EUTRACGI struct {
	PLMNID            PLMNID `json:"plmn_id"`
	EUTRACellIdentity string `json:"eutra_cell_identity"`
}

type TAI struct {
	PLMNID PLMNID `json:"plmn_id"`
	TAC    string `json:"tac"`
}

type UserLocationInformationEUTRA struct {
	EUTRACGI  EUTRACGI `json:"eutra_cgi"`
	TAI       TAI      `json:"tai"`
	TimeStamp *string  `json:"timestamp,omitempty"`

	Error string `json:"error,omitempty"` // Reserved field for decoding errors
}

type NRCGI struct {
	PLMNID         PLMNID `json:"plmn_id"`
	NRCellIdentity string `json:"nr_cell_identity"`
}

type UserLocationInformationNR struct {
	NRCGI     NRCGI   `json:"nr_cgi"`
	TAI       TAI     `json:"tai"`
	TimeStamp *string `json:"timestamp,omitempty"`

	Error string `json:"error,omitempty"` // Reserved field for decoding errors
}

type UserLocationInformationN3IWF struct {
	IPAddress  string `json:"ip_address"`
	PortNumber int32  `json:"port_number"`
}

type UserLocationInformation struct {
	EUTRA *UserLocationInformationEUTRA `json:"eutra,omitempty"`
	NR    *UserLocationInformationNR    `json:"nr,omitempty"`
	N3IWF *UserLocationInformationN3IWF `json:"n3iwf,omitempty"`

	Error string `json:"error,omitempty"` // Reserved field for decoding errors
}

type FiveGSTMSI struct {
	AMFSetID   string `json:"amf_set_id"`
	AMFPointer string `json:"amf_pointer"`
	FiveGTMSI  string `json:"fiveg_tmsi"`
}

// Initial UE Message carries the UE's first NAS message on a new RAN UE
// association (TS 38.413 §9.2.5.2).
func buildInitialUEMessage(value []byte) NGAPMessageValue {
	m, err := ngap.ParseInitialUEMessage(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Initial UE Message: %v", err)}
	}

	ies := []IE{
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
		ie(ngap.IDNASPDU, ngap.CriticalityReject, libNASPDU(m.NASPDU)),
		ie(ngap.IDUserLocationInformation, ngap.CriticalityReject,
			userLocationInformation(m.UserLocationInformation)),
	}

	if m.RRCEstablishmentCause != nil {
		ies = append(ies, ie(ngap.IDRRCEstablishmentCause, ngap.CriticalityIgnore,
			libRRCEstablishmentCause(*m.RRCEstablishmentCause)))
	}

	if m.FiveGSTMSI != nil {
		ies = append(ies, ie(ngap.IDFiveGSTMSI, ngap.CriticalityReject, buildFiveGSTMSI(*m.FiveGSTMSI)))
	}

	if m.AMFSetID != nil {
		ies = append(ies, ie(ngap.IDAMFSetID, ngap.CriticalityIgnore, bitsHex(uint64(*m.AMFSetID), 10)))
	}

	if m.UEContextRequest != nil {
		ies = append(ies, ie(ngap.IDUEContextRequest, ngap.CriticalityIgnore,
			libUEContextRequest(*m.UEContextRequest)))
	}

	if m.AllowedNSSAI != nil {
		slices := make([]SNSSAI, 0, len(m.AllowedNSSAI))
		for _, item := range m.AllowedNSSAI {
			slices = append(slices, buildSNSSAIValue(item.SNSSAI))
		}

		ies = append(ies, ie(ngap.IDAllowedNSSAI, ngap.CriticalityReject, slices))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func libRRCEstablishmentCause(c ngap.RRCEstablishmentCause) utils.EnumField {
	return utils.NamedEnum(uint8(c), c.Name())
}

func libUEContextRequest(r ngap.UEContextRequest) utils.EnumField {
	return utils.NamedEnum(uint8(r), r.Name())
}
