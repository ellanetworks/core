// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/s1ap"
)

// ERABToBeSwitchedDL is an E-RAB whose downlink GTP endpoint the target eNB asks
// the MME to switch to it (TS 36.413 §9.1.5.8).
type ERABToBeSwitchedDL struct {
	ERABID                        uint8            `json:"erab_id"`
	TransportLayerAddress         string           `json:"transport_layer_address"`
	GTPTEID                       uint32           `json:"gtp_teid"`
	IntegrityProtectionIndication *utils.EnumField `json:"integrity_protection_indication,omitempty"`
}

type ERABToBeSwitchedUL struct {
	ERABID                uint8  `json:"erab_id"`
	TransportLayerAddress string `json:"transport_layer_address"`
	GTPTEID               uint32 `json:"gtp_teid"`
}

func buildPathSwitchRequest(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParsePathSwitchRequest(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Path Switch Request: %v", err)}, ""
	}

	switched := make([]ERABToBeSwitchedDL, 0, len(m.ERABToBeSwitchedDL))
	for _, it := range m.ERABToBeSwitchedDL {
		switched = append(switched, ERABToBeSwitchedDL{
			ERABID:                uint8(it.ERABID),
			TransportLayerAddress: transportLayerAddress(it.TransportLayerAddress),
			GTPTEID:               uint32(it.GTPTEID),

			IntegrityProtectionIndication: integrityProtectionIndication(it.SecurityIndication),
		})
	}

	ies := []IE{
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(s1ap.IDERABToBeSwitchedDLList, s1ap.CriticalityReject, switched),
		ie(s1ap.IDSourceMMEUES1APID, s1ap.CriticalityReject, uint32(m.SourceMMEUES1APID)),
	}

	if m.EUTRANCGI != nil {
		ies = append(ies, ie(s1ap.IDEUTRANCGI, s1ap.CriticalityIgnore, eutranCGI(*m.EUTRANCGI)))
	}

	if m.TAI != nil {
		ies = append(ies, ie(s1ap.IDTAI, s1ap.CriticalityIgnore, tai(*m.TAI)))
	}

	if m.UESecurityCapabilities != nil {
		ies = append(ies, ie(s1ap.IDUESecurityCapabilities, s1ap.CriticalityIgnore, UESecurityCapabilities{
			EncryptionAlgorithms:          securityAlgorithms(m.UESecurityCapabilities.EncryptionAlgorithms.Value, "EEA"),
			IntegrityProtectionAlgorithms: securityAlgorithms(m.UESecurityCapabilities.IntegrityProtectionAlgorithms.Value, "EIA"),
		}))
	}

	if m.SourceMMEGUMMEI != nil {
		ies = append(ies, ie(s1ap.IDSourceMMEGUMMEI, s1ap.CriticalityIgnore, gummei(*m.SourceMMEGUMMEI)))
	}

	ies = appendTyped(ies, []typedIE{
		{s1ap.IDTunnelInformationForBBF, s1ap.CriticalityIgnore, m.TunnelInformationForBBF},
	})

	ies = appendTyped(ies, []typedIE{
		{s1ap.IDCSGId, s1ap.CriticalityIgnore, m.CSGId},
		{s1ap.IDCellAccessMode, s1ap.CriticalityIgnore, m.CellAccessMode},
		{s1ap.IDCSGMembershipStatus, s1ap.CriticalityIgnore, m.CSGMembershipStatus},
		{s1ap.IDLHNID, s1ap.CriticalityIgnore, m.LHNID},
		{s1ap.IDRRCResumeCause, s1ap.CriticalityIgnore, m.RRCResumeCause},
		{s1ap.IDNRUESecurityCapabilities, s1ap.CriticalityIgnore, m.NRUESecurityCapabilities},
		{s1ap.IDPSCellInformation, s1ap.CriticalityIgnore, m.PSCellInformation},
		{s1ap.IDLTENTNTAIInformation, s1ap.CriticalityIgnore, m.LTENTNTAIInformation},
	})

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Path Switch Request (eNB-UE %d, source MME-UE %d, %d E-RAB)", m.ENBUES1APID, m.SourceMMEUES1APID, len(m.ERABToBeSwitchedDL))
}

func buildPathSwitchRequestAcknowledge(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParsePathSwitchRequestAcknowledge(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Path Switch Request Acknowledge: %v", err)}, ""
	}

	var ies []IE

	if m.MMEUES1APID != nil {
		ies = append(ies, ie(s1ap.IDMMEUES1APID, s1ap.CriticalityIgnore, uint32(*m.MMEUES1APID)))
	}

	if m.ENBUES1APID != nil {
		ies = append(ies, ie(s1ap.IDENBUES1APID, s1ap.CriticalityIgnore, uint32(*m.ENBUES1APID)))
	}

	if ambr := m.UEAggregateMaximumBitRate; ambr != nil {
		ies = append(ies, ie(s1ap.IDUEAggregateMaximumBitrate, s1ap.CriticalityIgnore, AMBR{DL: uint64(ambr.DL), UL: uint64(ambr.UL)}))
	}

	if len(m.ERABToBeSwitchedUL) > 0 {
		switched := make([]ERABToBeSwitchedUL, 0, len(m.ERABToBeSwitchedUL))
		for _, it := range m.ERABToBeSwitchedUL {
			switched = append(switched, ERABToBeSwitchedUL{
				ERABID:                uint8(it.ERABID),
				TransportLayerAddress: transportLayerAddress(it.TransportLayerAddress),
				GTPTEID:               uint32(it.GTPTEID),
			})
		}

		ies = append(ies, ie(s1ap.IDERABToBeSwitchedULList, s1ap.CriticalityIgnore, switched))
	}

	if len(m.ERABToBeReleased) > 0 {
		ies = append(ies, ie(s1ap.IDERABToBeReleasedList, s1ap.CriticalityIgnore, erabFailedItems(m.ERABToBeReleased)))
	}

	ies = append(ies, ie(s1ap.IDSecurityContext, s1ap.CriticalityReject, securityContext(m.SecurityContext)))

	if m.MMEUES1APID2 != nil {
		ies = append(ies, ie(s1ap.IDMMEUES1APID2, s1ap.CriticalityIgnore, uint32(*m.MMEUES1APID2)))
	}

	if m.UESecurityCapabilities != nil {
		ies = append(ies, ie(s1ap.IDUESecurityCapabilities, s1ap.CriticalityIgnore, UESecurityCapabilities{
			EncryptionAlgorithms:          securityAlgorithms(m.UESecurityCapabilities.EncryptionAlgorithms.Value, "EEA"),
			IntegrityProtectionAlgorithms: securityAlgorithms(m.UESecurityCapabilities.IntegrityProtectionAlgorithms.Value, "EIA"),
		}))
	}

	ies = appendTyped(ies, []typedIE{
		{s1ap.IDTimeRefDistribution, s1ap.CriticalityIgnore, m.TimeRefDistribution},
	})

	ies = appendTyped(ies, []typedIE{
		{s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, m.CriticalityDiagnostics},
		{s1ap.IDCSGMembershipStatus, s1ap.CriticalityIgnore, m.CSGMembershipStatus},
		{s1ap.IDProSeAuthorized, s1ap.CriticalityIgnore, m.ProSeAuthorized},
		{s1ap.IDUEUserPlaneCIoTSupportIndicator, s1ap.CriticalityIgnore, m.UEUserPlaneCIoTSupportIndicator},
		{s1ap.IDV2XServicesAuthorized, s1ap.CriticalityIgnore, m.V2XServicesAuthorized},
		{s1ap.IDUESidelinkAggregateMaximumBitrate, s1ap.CriticalityIgnore, m.UESidelinkAggregateMaximumBitrate},
		{s1ap.IDEnhancedCoverageRestricted, s1ap.CriticalityIgnore, m.EnhancedCoverageRestricted},
		{s1ap.IDNRUESecurityCapabilities, s1ap.CriticalityIgnore, m.NRUESecurityCapabilities},
		{s1ap.IDCEModeBRestricted, s1ap.CriticalityIgnore, m.CEModeBRestricted},
		{s1ap.IDAerialUEsubscriptionInformation, s1ap.CriticalityIgnore, m.AerialUEsubscriptionInformation},
		{s1ap.IDPendingDataIndication, s1ap.CriticalityIgnore, m.PendingDataIndication},
		{s1ap.IDSubscriptionBasedUEDifferentiationInfo, s1ap.CriticalityIgnore, m.SubscriptionBasedUEDifferentiationInfo},
		{s1ap.IDHandoverRestrictionList, s1ap.CriticalityIgnore, m.HandoverRestrictionList},
		{s1ap.IDAdditionalRRMPriorityIndex, s1ap.CriticalityIgnore, m.AdditionalRRMPriorityIndex},
		{s1ap.IDNRV2XServicesAuthorized, s1ap.CriticalityIgnore, m.NRV2XServicesAuthorized},
		{s1ap.IDNRUESidelinkAggregateMaximumBitrate, s1ap.CriticalityIgnore, m.NRUESidelinkAggregateMaximumBitrate},
		{s1ap.IDPC5QoSParameters, s1ap.CriticalityIgnore, m.PC5QoSParameters},
		{s1ap.IDUERadioCapabilityID, s1ap.CriticalityReject, m.UERadioCapabilityID},
		{s1ap.IDERABToBeUpdatedList, s1ap.CriticalityIgnore, m.ERABToBeUpdatedList},
	})

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Path Switch Request Acknowledge (MME-UE %s, eNB-UE %s)", ueIDText(m.MMEUES1APID), ueIDText(m.ENBUES1APID))
}

func buildPathSwitchRequestFailure(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParsePathSwitchRequestFailure(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Path Switch Request Failure: %v", err)}, ""
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

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Path Switch Request Failure (MME-UE %s, eNB-UE %s)", ueIDText(m.MMEUES1APID), ueIDText(m.ENBUES1APID))
}

func buildNASNonDeliveryIndication(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseNASNonDeliveryIndication(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse NAS Non Delivery Indication: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(s1ap.IDNASPDU, s1ap.CriticalityIgnore, nasPDU(m.NASPDU)),
	}

	if m.Cause != nil {
		ies = append(ies, ie(s1ap.IDCause, s1ap.CriticalityIgnore, cause(*m.Cause)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("NAS Non Delivery Indication (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}
