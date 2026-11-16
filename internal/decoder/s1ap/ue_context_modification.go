// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap"
)

func buildUEContextModificationRequest(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseUEContextModificationRequest(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse UE Context Modification Request: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
	}

	if ambr := m.UEAggregateMaximumBitRate; ambr != nil {
		ies = append(ies, ie(s1ap.IDUEAggregateMaximumBitrate, s1ap.CriticalityIgnore, AMBR{DL: uint64(ambr.DL), UL: uint64(ambr.UL)}))
	}

	ies = appendTyped(ies, []typedIE{
		{s1ap.IDTimeRefDistribution, s1ap.CriticalityIgnore, m.TimeRefDistribution},
	})

	ies = appendTyped(ies, []typedIE{
		{s1ap.IDSecurityKey, s1ap.CriticalityReject, m.SecurityKey},
		{s1ap.IDSubscriberProfileIDforRFP, s1ap.CriticalityIgnore, m.SubscriberProfileIDforRFP},
		{s1ap.IDCSFallbackIndicator, s1ap.CriticalityReject, m.CSFallbackIndicator},
		{s1ap.IDUESecurityCapabilities, s1ap.CriticalityReject, m.UESecurityCapabilities},
		{s1ap.IDCSGMembershipStatus, s1ap.CriticalityIgnore, m.CSGMembershipStatus},
		{s1ap.IDRegisteredLAI, s1ap.CriticalityIgnore, m.RegisteredLAI},
		{s1ap.IDAdditionalCSFallbackIndicator, s1ap.CriticalityIgnore, m.AdditionalCSFallbackIndicator},
		{s1ap.IDProSeAuthorized, s1ap.CriticalityIgnore, m.ProSeAuthorized},
		{s1ap.IDSRVCCOperationPossible, s1ap.CriticalityIgnore, m.SRVCCOperationPossible},
		{s1ap.IDSRVCCOperationNotPossible, s1ap.CriticalityIgnore, m.SRVCCOperationNotPossible},
		{s1ap.IDV2XServicesAuthorized, s1ap.CriticalityIgnore, m.V2XServicesAuthorized},
		{s1ap.IDUESidelinkAggregateMaximumBitrate, s1ap.CriticalityIgnore, m.UESidelinkAggregateMaximumBitrate},
		{s1ap.IDNRUESecurityCapabilities, s1ap.CriticalityIgnore, m.NRUESecurityCapabilities},
		{s1ap.IDAerialUEsubscriptionInformation, s1ap.CriticalityIgnore, m.AerialUEsubscriptionInformation},
		{s1ap.IDAdditionalRRMPriorityIndex, s1ap.CriticalityIgnore, m.AdditionalRRMPriorityIndex},
		{s1ap.IDIABAuthorized, s1ap.CriticalityIgnore, m.IABAuthorized},
		{s1ap.IDNRV2XServicesAuthorized, s1ap.CriticalityIgnore, m.NRV2XServicesAuthorized},
		{s1ap.IDNRUESidelinkAggregateMaximumBitrate, s1ap.CriticalityIgnore, m.NRUESidelinkAggregateMaximumBitrate},
		{s1ap.IDPC5QoSParameters, s1ap.CriticalityIgnore, m.PC5QoSParameters},
		{s1ap.IDUERadioCapabilityID, s1ap.CriticalityReject, m.UERadioCapabilityID},
	})

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("UE Context Modification Request (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}

func buildUEContextModificationResponse(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseUEContextModificationResponse(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse UE Context Modification Response: %v", err)}, ""
	}

	var ies []IE

	if m.MMEUES1APID != nil {
		ies = append(ies, ie(s1ap.IDMMEUES1APID, s1ap.CriticalityIgnore, uint32(*m.MMEUES1APID)))
	}

	if m.ENBUES1APID != nil {
		ies = append(ies, ie(s1ap.IDENBUES1APID, s1ap.CriticalityIgnore, uint32(*m.ENBUES1APID)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("UE Context Modification Response (MME-UE %s, eNB-UE %s)", ueIDText(m.MMEUES1APID), ueIDText(m.ENBUES1APID))
}

func buildUEContextModificationFailure(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseUEContextModificationFailure(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse UE Context Modification Failure: %v", err)}, ""
	}

	var ies []IE

	if m.MMEUES1APID != nil {
		ies = append(ies, ie(s1ap.IDMMEUES1APID, s1ap.CriticalityIgnore, uint32(*m.MMEUES1APID)))
	}

	if m.ENBUES1APID != nil {
		ies = append(ies, ie(s1ap.IDENBUES1APID, s1ap.CriticalityIgnore, uint32(*m.ENBUES1APID)))
	}

	if m.Cause != nil {
		ies = append(ies, ie(s1ap.IDCause, s1ap.CriticalityIgnore, cause(*m.Cause)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("UE Context Modification Failure (MME-UE %s, eNB-UE %s)", ueIDText(m.MMEUES1APID), ueIDText(m.ENBUES1APID))
}
