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

type Cause struct {
	Group utils.EnumField `json:"group"`
	Value utils.EnumField `json:"value"`
}

func causeGroupToEnum(g ngap.CauseGroup) utils.EnumField {
	return utils.NamedEnum(uint8(g), g.Name())
}

// The library owns the cause vocabulary, including which values are extension
// additions and where their numbering resumes, so the index it reports is the
// one to display rather than the raw per-group value.
func cause(c ngap.Cause) Cause {
	name, index := c.ValueName()

	return Cause{
		Group: causeGroupToEnum(c.Group),
		Value: utils.MakeEnum(int64(index), name, name == "unknown"),
	}
}

func buildGlobalRANNodeID(id ngap.GlobalRANNodeID) GlobalRANNodeIDIE {
	ie := GlobalRANNodeIDIE{PLMNIdentity: plmnIDToDecoder(id.PLMNIdentity)}

	switch id.Kind {
	case ngap.RANNodeIDGNB:
		ie.GlobalGNBID = id.Hex()
	case ngap.RANNodeIDMacroNgENB, ngap.RANNodeIDShortMacroNgENB, ngap.RANNodeIDLongMacroNgENB:
		ie.GlobalNgENBID = id.Hex()
	case ngap.RANNodeIDN3IWF:
		ie.GlobalN3IWFID = id.Hex()
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
	return utils.NamedEnum(uint8(drx), drx.Name())
}

func buildUERetention(uri ngap.UERetentionInformation) utils.EnumField {
	return utils.NamedEnum(uint8(uri), uri.Name())
}

// buildNGSetupRequest renders an NG SETUP REQUEST. Absent IEs are omitted
// rather than rendered as a zero value, and IEs this version does not model
// are reported with their id, criticality and octets rather than dropped.
func buildNGSetupRequest(value []byte) NGAPMessageValue {
	req, err := ngap.ParseNGSetupRequest(value)
	if err != nil {
		return NGAPMessageValue{Error: err.Error()}
	}

	ies := make([]IE, 0, 5)

	ies = append(ies, ie(ngap.IDGlobalRANNodeID, ngap.CriticalityReject, buildGlobalRANNodeID(req.GlobalRANNodeID)))

	if req.RANNodeName != nil {
		ies = append(ies, ie(ngap.IDRANNodeName, ngap.CriticalityIgnore, *req.RANNodeName))
	}

	ies = append(ies, ie(ngap.IDSupportedTAList, ngap.CriticalityReject, buildSupportedTAList(req.SupportedTAList)))

	if req.DefaultPagingDRX != nil {
		ies = append(ies, ie(ngap.IDDefaultPagingDRX, ngap.CriticalityIgnore, buildPagingDRX(*req.DefaultPagingDRX)))
	}

	if req.UERetentionInformation != nil {
		ies = append(ies, ie(ngap.IDUERetentionInformation, ngap.CriticalityIgnore,
			buildUERetention(*req.UERetentionInformation)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(req.UnknownIEs())...)}
}

func guami(g ngap.GUAMI) Guami {
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
		out[i] = guami(item.GUAMI)
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
func buildNGSetupResponse(value []byte) NGAPMessageValue {
	resp, err := ngap.ParseNGSetupResponse(value)
	if err != nil {
		return NGAPMessageValue{Error: err.Error()}
	}

	ies := []IE{
		ie(ngap.IDAMFName, ngap.CriticalityReject, resp.AMFName),
		ie(ngap.IDServedGUAMIList, ngap.CriticalityReject, buildServedGUAMIList(resp.ServedGUAMIList)),
	}

	if resp.RelativeAMFCapacity != nil {
		ies = append(ies, ie(ngap.IDRelativeAMFCapacity, ngap.CriticalityIgnore, int64(*resp.RelativeAMFCapacity)))
	}

	ies = append(ies, ie(ngap.IDPLMNSupportList, ngap.CriticalityReject, buildPLMNSupportList(resp.PLMNSupportList)))

	if resp.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore,
			criticalityDiagnostics(*resp.CriticalityDiagnostics)))
	}

	if resp.UERetentionInformation != nil {
		ies = append(ies, ie(ngap.IDUERetentionInformation, ngap.CriticalityIgnore,
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
		ies = append(ies, ie(ngap.IDCause, ngap.CriticalityIgnore, cause(*fail.Cause)))
	}

	if fail.TimeToWait != nil {
		ies = append(ies, ie(ngap.IDTimeToWait, ngap.CriticalityIgnore, buildTimeToWait(*fail.TimeToWait)))
	}

	if fail.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore,
			criticalityDiagnostics(*fail.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(fail.UnknownIEs())...)}
}

func buildTimeToWait(t ngap.TimeToWait) utils.EnumField {
	return utils.NamedEnum(uint8(t), t.Name())
}
