// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap"
)

func buildInitialUEMessage(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseInitialUEMessage(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Initial UE Message: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(s1ap.IDNASPDU, s1ap.CriticalityReject, nasPDU(m.NASPDU)),
		ie(s1ap.IDTAI, s1ap.CriticalityReject, tai(m.TAI)),
	}

	if m.EUTRANCGI != nil {
		ies = append(ies, ie(s1ap.IDEUTRANCGI, s1ap.CriticalityIgnore, eutranCGI(*m.EUTRANCGI)))
	}

	if m.RRCEstablishmentCause != nil {
		ies = append(ies, ie(s1ap.IDRRCEstablishmentCause, s1ap.CriticalityIgnore, rrcCauseToEnum(*m.RRCEstablishmentCause)))
	}

	if m.STMSI != nil {
		ies = append(ies, ie(s1ap.IDSTMSI, s1ap.CriticalityReject, stmsi(*m.STMSI)))
	}

	if m.GUMMEI != nil {
		ies = append(ies, ie(s1ap.IDGUMMEI, s1ap.CriticalityReject, gummei(*m.GUMMEI)))
	}

	ies = appendTyped(ies, []typedIE{
		{s1ap.IDGWTransportLayerAddress, s1ap.CriticalityIgnore, m.GWTransportLayerAddress},
		{s1ap.IDTunnelInformationForBBF, s1ap.CriticalityIgnore, m.TunnelInformationForBBF},
		{s1ap.IDSIPTOLGWTransportLayerAddress, s1ap.CriticalityIgnore, m.SIPTOLGWTransportLayerAddress},
		{s1ap.IDCoarseUELocationRequested, s1ap.CriticalityIgnore, m.CoarseUELocationRequested},
	})

	ies = appendTyped(ies, []typedIE{
		{s1ap.IDCSGId, s1ap.CriticalityReject, m.CSGId},
		{s1ap.IDCellAccessMode, s1ap.CriticalityReject, m.CellAccessMode},
		{s1ap.IDRelayNodeIndicator, s1ap.CriticalityReject, m.RelayNodeIndicator},
		{s1ap.IDGUMMEIType, s1ap.CriticalityIgnore, m.GUMMEIType},
		{s1ap.IDLHNID, s1ap.CriticalityIgnore, m.LHNID},
		{s1ap.IDMMEGroupID, s1ap.CriticalityIgnore, m.MMEGroupID},
		{s1ap.IDUEUsageType, s1ap.CriticalityIgnore, m.UEUsageType},
		{s1ap.IDCEmodeBSupportIndicator, s1ap.CriticalityIgnore, m.CEmodeBSupportIndicator},
		{s1ap.IDDCNID, s1ap.CriticalityIgnore, m.DCNID},
		{s1ap.IDCoverageLevel, s1ap.CriticalityIgnore, m.CoverageLevel},
		{s1ap.IDUEApplicationLayerMeasurementCapability, s1ap.CriticalityIgnore, m.UEApplicationLayerMeasurementCapability},
		{s1ap.IDEDTSession, s1ap.CriticalityIgnore, m.EDTSession},
		{s1ap.IDIABNodeIndication, s1ap.CriticalityReject, m.IABNodeIndication},
		{s1ap.IDLTENTNTAIInformation, s1ap.CriticalityIgnore, m.LTENTNTAIInformation},
	})

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Initial UE Message (eNB-UE %d%s)", m.ENBUES1APID, nasSummary(m.NASPDU))
}

func buildUplinkNASTransport(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseUplinkNASTransport(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Uplink NAS Transport: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(s1ap.IDNASPDU, s1ap.CriticalityReject, nasPDU(m.NASPDU)),
	}

	if m.EUTRANCGI != nil {
		ies = append(ies, ie(s1ap.IDEUTRANCGI, s1ap.CriticalityIgnore, eutranCGI(*m.EUTRANCGI)))
	}

	if m.TAI != nil {
		ies = append(ies, ie(s1ap.IDTAI, s1ap.CriticalityIgnore, tai(*m.TAI)))
	}

	ies = appendTyped(ies, []typedIE{
		{s1ap.IDGWTransportLayerAddress, s1ap.CriticalityIgnore, m.GWTransportLayerAddress},
		{s1ap.IDSIPTOLGWTransportLayerAddress, s1ap.CriticalityIgnore, m.SIPTOLGWTransportLayerAddress},
	})

	ies = appendTyped(ies, []typedIE{
		{s1ap.IDLHNID, s1ap.CriticalityIgnore, m.LHNID},
		{s1ap.IDPSCellInformation, s1ap.CriticalityIgnore, m.PSCellInformation},
		{s1ap.IDLTENTNTAIInformation, s1ap.CriticalityIgnore, m.LTENTNTAIInformation},
	})

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Uplink NAS Transport (MME-UE %d, eNB-UE %d%s)", m.MMEUES1APID, m.ENBUES1APID, nasSummary(m.NASPDU))
}

func buildDownlinkNASTransport(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseDownlinkNASTransport(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Downlink NAS Transport: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(s1ap.IDNASPDU, s1ap.CriticalityReject, nasPDU(m.NASPDU)),
	}

	ies = appendTyped(ies, []typedIE{
		{s1ap.IDCoarseUELocation, s1ap.CriticalityIgnore, m.CoarseUELocation},
	})

	ies = appendTyped(ies, []typedIE{
		{s1ap.IDHandoverRestrictionList, s1ap.CriticalityIgnore, m.HandoverRestrictionList},
		{s1ap.IDSubscriberProfileIDforRFP, s1ap.CriticalityIgnore, m.SubscriberProfileIDforRFP},
		{s1ap.IDSRVCCOperationPossible, s1ap.CriticalityIgnore, m.SRVCCOperationPossible},
		{s1ap.IDUERadioCapability, s1ap.CriticalityIgnore, m.UERadioCapability},
		{s1ap.IDDLNASPDUDeliveryAckRequest, s1ap.CriticalityIgnore, m.DLNASPDUDeliveryAckRequest},
		{s1ap.IDEnhancedCoverageRestricted, s1ap.CriticalityIgnore, m.EnhancedCoverageRestricted},
		{s1ap.IDNRUESecurityCapabilities, s1ap.CriticalityIgnore, m.NRUESecurityCapabilities},
		{s1ap.IDCEModeBRestricted, s1ap.CriticalityIgnore, m.CEModeBRestricted},
		{s1ap.IDUECapabilityInfoRequest, s1ap.CriticalityIgnore, m.UECapabilityInfoRequest},
		{s1ap.IDEndIndication, s1ap.CriticalityIgnore, m.EndIndication},
		{s1ap.IDPendingDataIndication, s1ap.CriticalityIgnore, m.PendingDataIndication},
		{s1ap.IDSubscriptionBasedUEDifferentiationInfo, s1ap.CriticalityIgnore, m.SubscriptionBasedUEDifferentiationInfo},
		{s1ap.IDAdditionalRRMPriorityIndex, s1ap.CriticalityIgnore, m.AdditionalRRMPriorityIndex},
		{s1ap.IDUERadioCapabilityID, s1ap.CriticalityReject, m.UERadioCapabilityID},
		{s1ap.IDMaskedIMEISV, s1ap.CriticalityIgnore, m.MaskedIMEISV},
	})

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Downlink NAS Transport (MME-UE %d, eNB-UE %d%s)", m.MMEUES1APID, m.ENBUES1APID, nasSummary(m.NASPDU))
}
