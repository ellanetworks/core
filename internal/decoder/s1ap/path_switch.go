// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap"
)

// ERABToBeSwitchedDL is an E-RAB whose downlink GTP endpoint the target eNB asks
// the MME to switch to it (TS 36.413 §9.1.5.8).
type ERABToBeSwitchedDL struct {
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
			EncryptionAlgorithms:          securityAlgorithms(m.UESecurityCapabilities.EncryptionAlgorithms, "EEA"),
			IntegrityProtectionAlgorithms: securityAlgorithms(m.UESecurityCapabilities.IntegrityProtectionAlgorithms, "EIA"),
		}))
	}

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

	if len(m.ERABToBeReleased) > 0 {
		ies = append(ies, ie(s1ap.IDERABToBeReleasedList, s1ap.CriticalityIgnore, erabFailedItems(m.ERABToBeReleased)))
	}

	ies = append(ies, ie(s1ap.IDSecurityContext, s1ap.CriticalityReject, securityContext(m.SecurityContext)))

	if m.UESecurityCapabilities != nil {
		ies = append(ies, ie(s1ap.IDUESecurityCapabilities, s1ap.CriticalityIgnore, UESecurityCapabilities{
			EncryptionAlgorithms:          securityAlgorithms(m.UESecurityCapabilities.EncryptionAlgorithms, "EEA"),
			IntegrityProtectionAlgorithms: securityAlgorithms(m.UESecurityCapabilities.IntegrityProtectionAlgorithms, "EIA"),
		}))
	}

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
