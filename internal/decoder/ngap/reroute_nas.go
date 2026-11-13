// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/ngap"
)

type SourceToTargetAMFInformationReroute struct {
	ConfiguredNSSAI     *utils.RawOctets `json:"configured_nssai,omitempty"`
	RejectedNSSAIinPLMN *utils.RawOctets `json:"rejected_nssai_in_plmn,omitempty"`
	RejectedNSSAIinTA   *utils.RawOctets `json:"rejected_nssai_in_ta,omitempty"`
}

func sourceToTargetAMFInformationReroute(s ngap.SourceToTargetAMFInformationReroute) SourceToTargetAMFInformationReroute {
	var out SourceToTargetAMFInformationReroute

	if s.ConfiguredNSSAI != nil {
		out.ConfiguredNSSAI = utils.NewRawOctets(s.ConfiguredNSSAI[:])
	}

	if s.RejectedNSSAIinPLMN != nil {
		out.RejectedNSSAIinPLMN = utils.NewRawOctets(s.RejectedNSSAIinPLMN[:])
	}

	if s.RejectedNSSAIinTA != nil {
		out.RejectedNSSAIinTA = utils.NewRawOctets(s.RejectedNSSAIinTA[:])
	}

	return out
}

func buildRerouteNASRequest(value []byte) NGAPMessageValue {
	m, err := ngap.ParseRerouteNASRequest(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Reroute NAS Request: %v", err)}
	}

	ies := []IE{ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID))}

	if m.AMFUENGAPID != nil {
		ies = append(ies, ie(ngap.IDAMFUENGAPID, ngap.CriticalityIgnore, int64(*m.AMFUENGAPID)))
	}

	ies = append(ies,
		ie(ngap.IDNGAPMessage, ngap.CriticalityReject, utils.NewRawOctets(m.NGAPMessage)),
		ie(ngap.IDAMFSetID, ngap.CriticalityReject, bitsHex(uint64(m.AMFSetID), 10)),
	)

	if len(m.AllowedNSSAI) > 0 {
		slices := make([]SNSSAI, 0, len(m.AllowedNSSAI))
		for _, item := range m.AllowedNSSAI {
			slices = append(slices, buildSNSSAIValue(item.SNSSAI))
		}

		ies = append(ies, ie(ngap.IDAllowedNSSAI, ngap.CriticalityReject, slices))
	}

	if m.SourceToTargetAMFInformationReroute != nil {
		ies = append(ies, ie(ngap.IDSourceToTargetAMFInformationReroute, ngap.CriticalityIgnore,
			sourceToTargetAMFInformationReroute(*m.SourceToTargetAMFInformationReroute)))
	}

	if len(m.PartiallyAllowedNSSAI) > 0 {
		slices := make([]SNSSAI, 0, len(m.PartiallyAllowedNSSAI))
		for _, item := range m.PartiallyAllowedNSSAI {
			slices = append(slices, buildSNSSAIValue(item.SNSSAI))
		}

		ies = append(ies, ie(ngap.IDPartiallyAllowedNSSAI, ngap.CriticalityIgnore, slices))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
