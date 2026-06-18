// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/internal/s1apcause"
	"github.com/ellanetworks/core/s1ap"
)

// ENBID is the decoded eNB identity (TS 36.413 §9.2.1.37): the choice kind and
// its numeric value.
type ENBID struct {
	Kind  utils.EnumField[uint64] `json:"kind"`
	Value uint32                  `json:"value"`
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
	Group utils.EnumField[uint64] `json:"group"`
	Value utils.EnumField[int64]  `json:"value"`
}

func enbIDKind(kind s1ap.ENBIDKind) utils.EnumField[uint64] {
	switch kind {
	case s1ap.ENBIDMacro:
		return utils.MakeEnum(uint64(kind), "macro", false)
	case s1ap.ENBIDHome:
		return utils.MakeEnum(uint64(kind), "home", false)
	case s1ap.ENBIDShortMacro:
		return utils.MakeEnum(uint64(kind), "short-macro", false)
	case s1ap.ENBIDLongMacro:
		return utils.MakeEnum(uint64(kind), "long-macro", false)
	default:
		return utils.MakeEnum(uint64(kind), "", true)
	}
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

func pagingDRXToEnum(d s1ap.PagingDRX) utils.EnumField[uint64] {
	switch d {
	case s1ap.PagingDRXv32:
		return utils.MakeEnum(uint64(d), "v32", false)
	case s1ap.PagingDRXv64:
		return utils.MakeEnum(uint64(d), "v64", false)
	case s1ap.PagingDRXv128:
		return utils.MakeEnum(uint64(d), "v128", false)
	case s1ap.PagingDRXv256:
		return utils.MakeEnum(uint64(d), "v256", false)
	default:
		return utils.MakeEnum(uint64(d), "", true)
	}
}

func timeToWaitToEnum(t s1ap.TimeToWait) utils.EnumField[uint64] {
	names := map[s1ap.TimeToWait]string{
		s1ap.TimeToWaitV1s:  "v1s",
		s1ap.TimeToWaitV2s:  "v2s",
		s1ap.TimeToWaitV5s:  "v5s",
		s1ap.TimeToWaitV10s: "v10s",
		s1ap.TimeToWaitV20s: "v20s",
		s1ap.TimeToWaitV60s: "v60s",
	}

	name, ok := names[t]

	return utils.MakeEnum(uint64(t), name, !ok)
}

func causeGroupToEnum(g s1ap.CauseGroup) utils.EnumField[uint64] {
	switch g {
	case s1ap.CauseGroupRadioNetwork:
		return utils.MakeEnum(uint64(g), "radioNetwork", false)
	case s1ap.CauseGroupTransport:
		return utils.MakeEnum(uint64(g), "transport", false)
	case s1ap.CauseGroupNAS:
		return utils.MakeEnum(uint64(g), "nas", false)
	case s1ap.CauseGroupProtocol:
		return utils.MakeEnum(uint64(g), "protocol", false)
	case s1ap.CauseGroupMisc:
		return utils.MakeEnum(uint64(g), "misc", false)
	default:
		return utils.MakeEnum(uint64(g), "", true)
	}
}

func cause(c s1ap.Cause) Cause {
	name, index := s1apcause.ValueName(c.Group, c.Value, c.Extended)

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
		ID:          ieEnum(idGlobalENBID),
		Criticality: criticalityToEnum(s1ap.CriticalityReject),
		Value:       globalENBID(req.GlobalENBID),
	}}

	if req.ENBName != "" {
		ies = append(ies, IE{
			ID:          ieEnum(idENBname),
			Criticality: criticalityToEnum(s1ap.CriticalityIgnore),
			Value:       req.ENBName,
		})
	}

	ies = append(ies,
		IE{
			ID:          ieEnum(idSupportedTAs),
			Criticality: criticalityToEnum(s1ap.CriticalityReject),
			Value:       supportedTAs(req.SupportedTAs),
		},
		IE{
			ID:          ieEnum(idDefaultPagingDRX),
			Criticality: criticalityToEnum(s1ap.CriticalityIgnore),
			Value:       pagingDRXToEnum(req.DefaultPagingDRX),
		},
	)

	summary := "S1 Setup Request"
	if req.ENBName != "" {
		summary = fmt.Sprintf("S1 Setup Request (%s)", req.ENBName)
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

	if resp.MMEName != "" {
		ies = append(ies, IE{
			ID:          ieEnum(idMMEname),
			Criticality: criticalityToEnum(s1ap.CriticalityIgnore),
			Value:       resp.MMEName,
		})
	}

	ies = append(ies,
		IE{
			ID:          ieEnum(idServedGUMMEIs),
			Criticality: criticalityToEnum(s1ap.CriticalityReject),
			Value:       servedGUMMEIs(resp.ServedGUMMEIs),
		},
		IE{
			ID:          ieEnum(idRelativeMMECapacity),
			Criticality: criticalityToEnum(s1ap.CriticalityIgnore),
			Value:       resp.RelativeMMECapacity,
		},
	)

	if resp.CriticalityDiagnostics != nil {
		ies = append(ies, ie(idCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*resp.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, resp.UnknownIEs())

	return S1APMessageValue{IEs: ies}, "S1 Setup Response"
}

func buildS1SetupFailure(value []byte) (S1APMessageValue, string) {
	fail, err := s1ap.ParseS1SetupFailure(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse S1 Setup Failure: %v", err)}, ""
	}

	ies := []IE{{
		ID:          ieEnum(idCause),
		Criticality: criticalityToEnum(s1ap.CriticalityIgnore),
		Value:       cause(fail.Cause),
	}}

	if fail.TimeToWait != nil {
		ies = append(ies, IE{
			ID:          ieEnum(idTimeToWait),
			Criticality: criticalityToEnum(s1ap.CriticalityIgnore),
			Value:       timeToWaitToEnum(*fail.TimeToWait),
		})
	}

	if fail.CriticalityDiagnostics != nil {
		ies = append(ies, ie(idCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*fail.CriticalityDiagnostics)))
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
