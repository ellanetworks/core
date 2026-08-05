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
		ie(idRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
		ie(idNASPDU, ngap.CriticalityReject, libNASPDU(m.NASPDU)),
		ie(idUserLocationInformation, ngap.CriticalityReject,
			userLocationInformation(m.UserLocationInformation)),
	}

	if m.RRCEstablishmentCause != nil {
		ies = append(ies, ie(idRRCEstablishmentCause, ngap.CriticalityIgnore,
			libRRCEstablishmentCause(*m.RRCEstablishmentCause)))
	}

	if m.FiveGSTMSI != nil {
		ies = append(ies, ie(idFiveGSTMSI, ngap.CriticalityReject, buildFiveGSTMSI(*m.FiveGSTMSI)))
	}

	if m.AMFSetID != nil {
		ies = append(ies, ie(idAMFSetID, ngap.CriticalityIgnore, bitsHex(uint64(*m.AMFSetID), 10)))
	}

	if m.UEContextRequest != nil {
		ies = append(ies, ie(idUEContextRequest, ngap.CriticalityIgnore,
			libUEContextRequest(*m.UEContextRequest)))
	}

	if m.AllowedNSSAI != nil {
		slices := make([]SNSSAI, 0, len(m.AllowedNSSAI))
		for _, item := range m.AllowedNSSAI {
			slices = append(slices, buildSNSSAIValue(item.SNSSAI))
		}

		ies = append(ies, ie(idAllowedNSSAI, ngap.CriticalityReject, slices))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

// TS 38.413 §9.3.1.111. TS 36.413 §9.2.1.3a shares only the first five members;
// the rest have no S1AP counterpart.
func libRRCEstablishmentCause(c ngap.RRCEstablishmentCause) utils.EnumField {
	switch c {
	case ngap.RRCCauseEmergency:
		return utils.MakeEnum(uint64(c), "Emergency", false)
	case ngap.RRCCauseHighPriorityAccess:
		return utils.MakeEnum(uint64(c), "HighPriorityAccess", false)
	case ngap.RRCCauseMTAccess:
		return utils.MakeEnum(uint64(c), "MtAccess", false)
	case ngap.RRCCauseMOSignalling:
		return utils.MakeEnum(uint64(c), "MoSignalling", false)
	case ngap.RRCCauseMOData:
		return utils.MakeEnum(uint64(c), "MoData", false)
	case ngap.RRCCauseMOVoiceCall:
		return utils.MakeEnum(uint64(c), "MoVoiceCall", false)
	case ngap.RRCCauseMOVideoCall:
		return utils.MakeEnum(uint64(c), "MoVideoCall", false)
	case ngap.RRCCauseMOSMS:
		return utils.MakeEnum(uint64(c), "MoSMS", false)
	case ngap.RRCCauseMPSPriorityAccess:
		return utils.MakeEnum(uint64(c), "MpsPriorityAccess", false)
	case ngap.RRCCauseMCSPriorityAccess:
		return utils.MakeEnum(uint64(c), "McsPriorityAccess", false)
	default:
		return utils.MakeEnum(uint64(c), "", true)
	}
}

func libUEContextRequest(r ngap.UEContextRequest) utils.EnumField {
	if r == ngap.UEContextRequested {
		return utils.MakeEnum(uint64(r), "Requested", false)
	}

	return utils.MakeEnum(uint64(r), "", true)
}
