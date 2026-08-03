// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/ngap"
)

type GlobalRANNodeIDIE struct {
	PLMNIdentity  PLMNID `json:"plmn_identity"`
	GlobalGNBID   string `json:"global_gnb_id,omitempty"`
	GlobalNgENBID string `json:"global_ng_enb_id,omitempty"`
	GlobalN3IWFID string `json:"global_n3iwf_id,omitempty"`
}

type SupportedTA struct {
	TAC               string `json:"tac"`
	BroadcastPLMNList []PLMN `json:"broadcast_plmn_list,omitempty"`
}

// ranNodeIDHex renders a RAN node identifier as the hex digits its bit length
// covers, matching how the AMF stores it.
func ranNodeIDHex(id ngap.GlobalRANNodeID) string {
	b := make([]byte, (id.Bits+7)/8)

	for i := range id.Bits {
		if id.Value&(1<<uint(id.Bits-1-i)) != 0 {
			b[i/8] |= 1 << uint(7-i%8)
		}
	}

	return hex.EncodeToString(b)[:(id.Bits+3)/4]
}

func buildGlobalRANNodeID(id ngap.GlobalRANNodeID) GlobalRANNodeIDIE {
	ie := GlobalRANNodeIDIE{PLMNIdentity: plmnIDToDecoder(id.PLMNIdentity)}

	switch id.Kind {
	case ngap.RANNodeIDGNB:
		ie.GlobalGNBID = ranNodeIDHex(id)
	case ngap.RANNodeIDMacroNgENB, ngap.RANNodeIDShortMacroNgENB, ngap.RANNodeIDLongMacroNgENB:
		ie.GlobalNgENBID = ranNodeIDHex(id)
	case ngap.RANNodeIDN3IWF:
		ie.GlobalN3IWFID = ranNodeIDHex(id)
	}

	return ie
}

func buildSupportedTAList(list ngap.SupportedTAList) []SupportedTA {
	if len(list) == 0 {
		return nil
	}

	out := make([]SupportedTA, len(list))
	for i, ta := range list {
		out[i] = SupportedTA{
			TAC:               fmt.Sprintf("%06x", uint32(ta.TAC)),
			BroadcastPLMNList: buildBroadcastPLMNList(ta.BroadcastPLMNList),
		}
	}

	return out
}

func buildBroadcastPLMNList(list ngap.BroadcastPLMNList) []PLMN {
	out := make([]PLMN, len(list))
	for i, bp := range list {
		out[i] = PLMN{
			PLMNID:           plmnIDToDecoder(bp.PLMNIdentity),
			SliceSupportList: buildSliceSupportList(bp.TAISliceSupportList),
		}
	}

	return out
}

func buildSliceSupportList(list ngap.SliceSupportList) []SNSSAI {
	out := make([]SNSSAI, len(list))
	for i, item := range list {
		out[i] = buildSNSSAIValue(item.SNSSAI)
	}

	return out
}

func buildSNSSAIValue(s ngap.SNSSAI) SNSSAI {
	out := SNSSAI{SST: int32(s.SST)}
	if s.SD != nil {
		sd := hex.EncodeToString(s.SD[:])
		out.SD = &sd
	}

	return out
}

func buildPagingDRX(drx ngap.PagingDRX) utils.EnumField {
	switch drx {
	case ngap.PagingDRXv32:
		return utils.MakeEnum(uint64(drx), "v32", false)
	case ngap.PagingDRXv64:
		return utils.MakeEnum(uint64(drx), "v64", false)
	case ngap.PagingDRXv128:
		return utils.MakeEnum(uint64(drx), "v128", false)
	case ngap.PagingDRXv256:
		return utils.MakeEnum(uint64(drx), "v256", false)
	default:
		return utils.MakeEnum(uint64(drx), "", true)
	}
}

func buildUERetention(uri ngap.UERetentionInformation) utils.EnumField {
	if uri == ngap.UERetentionUesRetained {
		return utils.MakeEnum(uint64(uri), "UesRetained", false)
	}

	return utils.MakeEnum(uint64(uri), "", true)
}

// buildNGSetupRequest renders an NG SETUP REQUEST. Absent IEs are omitted
// rather than rendered as a zero value, and IEs this version does not model
// are reported with their id, criticality and octets rather than dropped.

// buildNGSetupRequest renders an NG SETUP REQUEST. Absent IEs are omitted
// rather than rendered as a zero value, and IEs this version does not model
// are reported with their id, criticality and octets rather than dropped.
func buildNGSetupRequest(value []byte) NGAPMessageValue {
	req, err := ngap.ParseNGSetupRequest(value)
	if err != nil {
		return NGAPMessageValue{Error: err.Error()}
	}

	ies := make([]IE, 0, 5)

	ies = append(ies, libIE(idGlobalRANNodeID, ngap.CriticalityReject, buildGlobalRANNodeID(req.GlobalRANNodeID)))

	if req.RANNodeName != nil {
		ies = append(ies, libIE(idRANNodeName, ngap.CriticalityIgnore, *req.RANNodeName))
	}

	ies = append(ies, libIE(idSupportedTAList, ngap.CriticalityReject, buildSupportedTAList(req.SupportedTAList)))

	if req.DefaultPagingDRX != nil {
		ies = append(ies, libIE(idDefaultPagingDRX, ngap.CriticalityIgnore, buildPagingDRX(*req.DefaultPagingDRX)))
	}

	if req.UERetentionInformation != nil {
		ies = append(ies, libIE(idUERetentionInformation, ngap.CriticalityIgnore,
			buildUERetention(*req.UERetentionInformation)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(req.UnknownIEs())...)}
}

func buildLibGUAMI(g ngap.GUAMI) Guami {
	return Guami{
		PLMNID:      plmnIDToDecoder(g.PLMNIdentity),
		AMFRegionID: bitsHex(uint64(g.AMFRegionID), 8),
		AMFSetID:    bitsHex(uint64(g.AMFSetID), 10),
		AMFPointer:  bitsHex(uint64(g.AMFPointer), 6),
	}
}

func buildServedGUAMIList(list ngap.ServedGUAMIList) []Guami {
	out := make([]Guami, len(list))
	for i, item := range list {
		out[i] = buildLibGUAMI(item.GUAMI)
	}

	return out
}

func buildPLMNSupportList(list ngap.PLMNSupportList) []PLMN {
	if len(list) == 0 {
		return nil
	}

	out := make([]PLMN, len(list))
	for i, item := range list {
		out[i] = PLMN{
			PLMNID:           plmnIDToDecoder(item.PLMNIdentity),
			SliceSupportList: buildSliceSupportList(item.SliceSupportList),
		}
	}

	return out
}

// buildNGSetupResponse renders an NG SETUP RESPONSE. Absent optional IEs are
// omitted, and IEs this version does not model keep their id and octets.

// buildNGSetupResponse renders an NG SETUP RESPONSE. Absent optional IEs are
// omitted, and IEs this version does not model keep their id and octets.
func buildNGSetupResponse(value []byte) NGAPMessageValue {
	resp, err := ngap.ParseNGSetupResponse(value)
	if err != nil {
		return NGAPMessageValue{Error: err.Error()}
	}

	ies := []IE{
		libIE(idAMFName, ngap.CriticalityReject, resp.AMFName),
		libIE(idServedGUAMIList, ngap.CriticalityReject, buildServedGUAMIList(resp.ServedGUAMIList)),
	}

	if resp.RelativeAMFCapacity != nil {
		ies = append(ies, libIE(idRelativeAMFCapacity, ngap.CriticalityIgnore, int64(*resp.RelativeAMFCapacity)))
	}

	ies = append(ies, libIE(idPLMNSupportList, ngap.CriticalityReject, buildPLMNSupportList(resp.PLMNSupportList)))

	if resp.CriticalityDiagnostics != nil {
		ies = append(ies, libIE(idCriticalityDiagnostics, ngap.CriticalityIgnore,
			buildLibCriticalityDiagnostics(*resp.CriticalityDiagnostics)))
	}

	if resp.UERetentionInformation != nil {
		ies = append(ies, libIE(idUERetentionInformation, ngap.CriticalityIgnore,
			buildUERetention(*resp.UERetentionInformation)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(resp.UnknownIEs())...)}
}

// buildNGSetupFailure renders an NG SETUP FAILURE. Cause is mandatory but
// ignore criticality (TS 38.413 §9.2.6.3), so a failure without one still
// decodes; it is then omitted rather than shown as a zero cause.
func buildNGSetupFailure(value []byte) NGAPMessageValue {
	fail, err := ngap.ParseNGSetupFailure(value)
	if err != nil {
		return NGAPMessageValue{Error: err.Error()}
	}

	ies := make([]IE, 0, 3)

	if fail.Cause != nil {
		ies = append(ies, libIE(idCause, ngap.CriticalityIgnore, libCause(*fail.Cause)))
	}

	if fail.TimeToWait != nil {
		ies = append(ies, libIE(idTimeToWait, ngap.CriticalityIgnore, buildTimeToWait(*fail.TimeToWait)))
	}

	if fail.CriticalityDiagnostics != nil {
		ies = append(ies, libIE(idCriticalityDiagnostics, ngap.CriticalityIgnore,
			buildLibCriticalityDiagnostics(*fail.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(fail.UnknownIEs())...)}
}

func buildTimeToWait(t ngap.TimeToWait) utils.EnumField {
	switch t {
	case ngap.TimeToWaitV1s:
		return utils.MakeEnum(uint64(t), "V1s", false)
	case ngap.TimeToWaitV2s:
		return utils.MakeEnum(uint64(t), "V2s", false)
	case ngap.TimeToWaitV5s:
		return utils.MakeEnum(uint64(t), "V5s", false)
	case ngap.TimeToWaitV10s:
		return utils.MakeEnum(uint64(t), "V10s", false)
	case ngap.TimeToWaitV20s:
		return utils.MakeEnum(uint64(t), "V20s", false)
	case ngap.TimeToWaitV60s:
		return utils.MakeEnum(uint64(t), "V60s", false)
	default:
		return utils.MakeEnum(uint64(t), "", true)
	}
}
