// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/s1ap"
)

// ENBID is the decoded eNB identity (TS 36.413 §9.2.1.37): the choice kind and
// its numeric value.
type ENBID struct {
	Kind  utils.EnumField `json:"kind"`
	Value uint32          `json:"value"`
}

type GlobalENBID struct {
	PLMNID PLMNID `json:"plmn_id"`
	ENBID  ENBID  `json:"enb_id"`
}

type SupportedTA struct {
	TAC            uint16   `json:"tac"`
	BroadcastPLMNs []PLMNID `json:"broadcast_plmns,omitempty"`
}

type ServedGUMMEI struct {
	ServedPLMNs    []PLMNID `json:"served_plmns,omitempty"`
	ServedGroupIDs []uint16 `json:"served_group_ids,omitempty"`
	ServedMMECodes []uint16 `json:"served_mme_codes,omitempty"` // uint16 so JSON renders a number array, not base64
}

// Cause is the decoded S1AP cause (TS 36.413 §9.2.1.3): the CHOICE group and the
// named value within that group.
type Cause struct {
	Group utils.EnumField `json:"group"`
	Value utils.EnumField `json:"value"`
}

func enbIDKind(kind s1ap.ENBIDKind) utils.EnumField {
	return utils.NamedEnum(uint8(kind), kind.Name())
}

func globalENBID(g s1ap.GlobalENBID) GlobalENBID {
	return GlobalENBID{
		PLMNID: plmnToID(g.PLMNIdentity),
		ENBID:  ENBID{Kind: enbIDKind(g.ENBID.Kind), Value: g.ENBID.Value},
	}
}

func supportedTAs(tas s1ap.SupportedTAs) []SupportedTA {
	out := make([]SupportedTA, 0, len(tas))

	for _, ta := range tas {
		plmns := make([]PLMNID, 0, len(ta.BroadcastPLMNs))
		for _, p := range ta.BroadcastPLMNs {
			plmns = append(plmns, plmnToID(p))
		}

		out = append(out, SupportedTA{TAC: uint16(ta.TAC), BroadcastPLMNs: plmns})
	}

	return out
}

func pagingDRXToEnum(d s1ap.PagingDRX) utils.EnumField {
	return utils.NamedEnum(uint8(d), d.Name())
}

func timeToWaitToEnum(t s1ap.TimeToWait) utils.EnumField {
	return utils.NamedEnum(uint8(t), t.Name())
}

func causeGroupToEnum(g s1ap.CauseGroup) utils.EnumField {
	return utils.NamedEnum(uint8(g), g.Name())
}

func cause(c s1ap.Cause) Cause {
	name, index := c.ValueName()

	return Cause{
		Group: causeGroupToEnum(c.Group),
		Value: utils.MakeEnum(int64(index), name, name == "unknown"),
	}
}

func buildS1SetupRequest(value []byte) (S1APMessageValue, string) {
	req, err := s1ap.ParseS1SetupRequest(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse S1 Setup Request: %v", err)}, ""
	}

	ies := []IE{{
		ID:          ieEnum(s1ap.IDGlobalENBID),
		Criticality: criticalityToEnum(s1ap.CriticalityReject),
		Value:       globalENBID(req.GlobalENBID),
	}}

	if req.ENBName != nil {
		ies = append(ies, IE{
			ID:          ieEnum(s1ap.IDENBname),
			Criticality: criticalityToEnum(s1ap.CriticalityIgnore),
			Value:       *req.ENBName,
		})
	}

	ies = append(ies, IE{
		ID:          ieEnum(s1ap.IDSupportedTAs),
		Criticality: criticalityToEnum(s1ap.CriticalityReject),
		Value:       supportedTAs(req.SupportedTAs),
	})

	if req.DefaultPagingDRX != nil {
		ies = append(ies, IE{
			ID:          ieEnum(s1ap.IDDefaultPagingDRX),
			Criticality: criticalityToEnum(s1ap.CriticalityIgnore),
			Value:       pagingDRXToEnum(*req.DefaultPagingDRX),
		})
	}

	if req.UERetentionInformation != nil {
		r := *req.UERetentionInformation
		ies = append(ies, ie(s1ap.IDUERetentionInformation, s1ap.CriticalityIgnore, utils.NamedEnum(uint8(r), r.Name())))
	}

	summary := "S1 Setup Request"
	if req.ENBName != nil {
		summary = fmt.Sprintf("S1 Setup Request (%s)", *req.ENBName)
	}

	ies = appendUnknownIEs(ies, req.UnknownIEs())

	return S1APMessageValue{IEs: ies}, summary
}

func buildS1SetupResponse(value []byte) (S1APMessageValue, string) {
	resp, err := s1ap.ParseS1SetupResponse(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse S1 Setup Response: %v", err)}, ""
	}

	var ies []IE

	if resp.MMEName != nil {
		ies = append(ies, IE{
			ID:          ieEnum(s1ap.IDMMEname),
			Criticality: criticalityToEnum(s1ap.CriticalityIgnore),
			Value:       *resp.MMEName,
		})
	}

	ies = append(ies, IE{
		ID:          ieEnum(s1ap.IDServedGUMMEIs),
		Criticality: criticalityToEnum(s1ap.CriticalityReject),
		Value:       servedGUMMEIs(resp.ServedGUMMEIs),
	})

	if resp.RelativeMMECapacity != nil {
		ies = append(ies, IE{
			ID:          ieEnum(s1ap.IDRelativeMMECapacity),
			Criticality: criticalityToEnum(s1ap.CriticalityIgnore),
			Value:       *resp.RelativeMMECapacity,
		})
	}

	if resp.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*resp.CriticalityDiagnostics)))
	}

	if resp.UERetentionInformation != nil {
		r := *resp.UERetentionInformation
		ies = append(ies, ie(s1ap.IDUERetentionInformation, s1ap.CriticalityIgnore, utils.NamedEnum(uint8(r), r.Name())))
	}

	ies = appendUnknownIEs(ies, resp.UnknownIEs())

	return S1APMessageValue{IEs: ies}, "S1 Setup Response"
}

func buildS1SetupFailure(value []byte) (S1APMessageValue, string) {
	fail, err := s1ap.ParseS1SetupFailure(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse S1 Setup Failure: %v", err)}, ""
	}

	var ies []IE

	if fail.Cause != nil {
		ies = append(ies, IE{
			ID:          ieEnum(s1ap.IDCause),
			Criticality: criticalityToEnum(s1ap.CriticalityIgnore),
			Value:       cause(*fail.Cause),
		})
	}

	if fail.TimeToWait != nil {
		ies = append(ies, IE{
			ID:          ieEnum(s1ap.IDTimeToWait),
			Criticality: criticalityToEnum(s1ap.CriticalityIgnore),
			Value:       timeToWaitToEnum(*fail.TimeToWait),
		})
	}

	if fail.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*fail.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, fail.UnknownIEs())

	return S1APMessageValue{IEs: ies}, "S1 Setup Failure"
}

func servedGUMMEIs(gummeis s1ap.ServedGUMMEIs) []ServedGUMMEI {
	out := make([]ServedGUMMEI, 0, len(gummeis))

	for _, g := range gummeis {
		plmns := make([]PLMNID, 0, len(g.ServedPLMNs))
		for _, p := range g.ServedPLMNs {
			plmns = append(plmns, plmnToID(p))
		}

		groupIDs := make([]uint16, 0, len(g.ServedGroupIDs))
		for _, id := range g.ServedGroupIDs {
			groupIDs = append(groupIDs, uint16(id[0])<<8|uint16(id[1]))
		}

		codes := make([]uint16, 0, len(g.ServedMMECs))
		for _, c := range g.ServedMMECs {
			codes = append(codes, uint16(c))
		}

		out = append(out, ServedGUMMEI{ServedPLMNs: plmns, ServedGroupIDs: groupIDs, ServedMMECodes: codes})
	}

	return out
}
