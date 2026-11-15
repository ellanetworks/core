// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// TS 38.413 §9.2.3.4. The AMF asks the target NG-RAN node to reserve resources.
// Only the mandatory IEs are modelled, as in TS 36.413 §9.1.5.4; NGAP adds
// AllowedNSSAI and GUAMI, both 5G-only, and orders the security IEs before the
// session list where S1AP orders them after.
type HandoverRequest struct {
	AMFUENGAPID               AMFUENGAPID
	HandoverType              HandoverType
	Cause                     *Cause
	UEAggregateMaximumBitRate UEAggregateMaximumBitRate
	UESecurityCapabilities    UESecurityCapabilities
	SecurityContext           SecurityContext

	// The two elements an EPS to 5GS handover adds. NewSecurityContextInd tells
	// the target the context is one the UE has not used yet, and NASC carries the
	// S1 mode to N1 mode NAS transparent container the target embeds in the
	// Target-to-Source container for the UE (TS 33.501 §8.4.2, TS 24.501
	// §9.11.2.9). Both are optional in the ASN.1 and neither is optional in
	// practice for that direction: without them the UE cannot take the mapped 5G
	// context into use.
	NewSecurityContextInd *NewSecurityContextInd
	NASC                  NASPDU

	PDUSessionResourceSetupListHOReq   PDUSessionResourceSetupListHOReq
	AllowedNSSAI                       AllowedNSSAI
	TraceActivation                    *TraceActivation
	SourceToTargetTransparentContainer SourceToTargetTransparentContainer

	// MobilityRestrictionList is optional in the ASN.1 and not optional in
	// practice: TS 38.413 §8.4.2.4 has the target NG-RAN node reject the handover
	// when the list is absent and it cannot determine the serving PLMN otherwise.
	MobilityRestrictionList *MobilityRestrictionList

	GUAMI                                       GUAMI
	AMFUENGAPID2                                *AMFUENGAPID
	TargetRANNodeID                             *GlobalRANNodeID
	CoreNetworkAssistanceInformationForInactive *CoreNetworkAssistanceInformationForInactive
	MaskedIMEISV                                *MaskedIMEISV
	LocationReportingRequestType                *LocationReportingRequestType
	RRCInactiveTransitionReportRequest          *RRCInactiveTransitionReportRequest
	RedirectionVoiceFallback                    *RedirectionVoiceFallback
	CNAssistedRANTuning                         *CNAssistedRANTuning
	SRVCCOperationPossible                      *SRVCCOperationPossible
	IABAuthorized                               *IABAuthorized
	EnhancedCoverageRestriction                 *EnhancedCoverageRestriction
	UEDifferentiationInfo                       *UEDifferentiationInfo
	NRV2XServicesAuthorized                     *NRV2XServicesAuthorized
	LTEV2XServicesAuthorized                    *LTEV2XServicesAuthorized
	NRUESidelinkAggregateMaximumBitrate         *NRUESidelinkAggregateMaximumBitrate
	LTEUESidelinkAggregateMaximumBitrate        *LTEUESidelinkAggregateMaximumBitrate
	PC5QoSParameters                            *PC5QoSParameters
	CEmodeBrestricted                           *CEmodeBrestricted
	UEUPCIoTSupport                             *UEUPCIoTSupport
	ManagementBasedMDTPLMNList                  MDTPLMNList
	UERadioCapabilityID                         *UERadioCapabilityID
	ExtendedConnectedTime                       *ExtendedConnectedTime
	TimeSyncAssistanceInfo                      *TimeSyncAssistanceInfo
	UESliceMaximumBitRateList                   UESliceMaximumBitRateList
	FiveGProSeAuthorized                        *FiveGProSeAuthorized
	FiveGProSeUEPC5AggregateMaximumBitRate      *NRUESidelinkAggregateMaximumBitrate
	FiveGProSePC5QoSParameters                  *FiveGProSePC5QoSParameters
	AerialUEsubscriptionInformation             *AerialUEsubscriptionInformation
	NRA2XServicesAuthorized                     *NRA2XServicesAuthorized
	LTEA2XServicesAuthorized                    *LTEA2XServicesAuthorized
	NRA2XUEPC5AggregateMaximumBitRate           *NRUESidelinkAggregateMaximumBitrate
	LTEA2XUEPC5AggregateMaximumBitRate          *LTEUESidelinkAggregateMaximumBitrate
	A2XPC5QoSParameters                         *A2XPC5QoSParameters
	MobileIABAuthorized                         *MobileIABAuthorized
	NoPDUSessionIndication                      *NoPDUSessionIndication
	PartiallyAllowedNSSAI                       PartiallyAllowedNSSAI
	SLPositioningRangingServiceInfo             *SLPositioningRangingServiceInfo

	messageMeta
}

var handoverRequestIEs = []ieSpec[HandoverRequest]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFUENGAPID)
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		ID: IDHandoverType, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.HandoverType)
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.HandoverType, true },
	},
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		ID: IDUEAggregateMaximumBitRate, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.UEAggregateMaximumBitRate)
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.UEAggregateMaximumBitRate, true },
	},
	apimpl.OptionalIE(IDCoreNetworkAssistanceInformationForInactive, CriticalityIgnore, func(m *HandoverRequest) **CoreNetworkAssistanceInformationForInactive {
		return &m.CoreNetworkAssistanceInformationForInactive
	}),
	{
		ID: IDUESecurityCapabilities, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.UESecurityCapabilities)
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.UESecurityCapabilities, true },
	},
	{
		ID: IDSecurityContext, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SecurityContext)
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.SecurityContext, true },
	},
	{
		ID: IDNewSecurityContextInd, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			var v NewSecurityContextInd

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.NewSecurityContextInd = &v

			return nil
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.NewSecurityContextInd == nil {
				return nil, false
			}

			return m.NewSecurityContextInd, true
		},
	},
	{
		ID: IDNASC, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.NASC)
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.NASC == nil {
				return nil, false
			}

			return m.NASC, true
		},
	},
	{
		ID: IDPDUSessionResourceSetupListHOReq, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceSetupListHOReq)
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.PDUSessionResourceSetupListHOReq == nil {
				return nil, false
			}

			return m.PDUSessionResourceSetupListHOReq, true
		},
	},
	{
		ID: IDAllowedNSSAI, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AllowedNSSAI)
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.AllowedNSSAI == nil {
				return nil, false
			}

			return m.AllowedNSSAI, true
		},
	},
	{
		ID: IDTraceActivation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			var v TraceActivation

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.TraceActivation = &v

			return nil
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.TraceActivation == nil {
				return nil, false
			}

			return m.TraceActivation, true
		},
	},
	apimpl.OptionalIE(IDMaskedIMEISV, CriticalityIgnore, func(m *HandoverRequest) **MaskedIMEISV { return &m.MaskedIMEISV }),
	{
		ID: IDSourceToTargetTransparentContainer, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SourceToTargetTransparentContainer)
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.SourceToTargetTransparentContainer == nil {
				return nil, false
			}

			return m.SourceToTargetTransparentContainer, true
		},
	},
	{
		ID: IDMobilityRestrictionList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			var v MobilityRestrictionList

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.MobilityRestrictionList = &v

			return nil
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.MobilityRestrictionList == nil {
				return nil, false
			}

			return m.MobilityRestrictionList, true
		},
	},
	apimpl.OptionalIE(IDLocationReportingRequestType, CriticalityIgnore, func(m *HandoverRequest) **LocationReportingRequestType { return &m.LocationReportingRequestType }),
	apimpl.OptionalIE(IDRRCInactiveTransitionReportRequest, CriticalityIgnore, func(m *HandoverRequest) **RRCInactiveTransitionReportRequest {
		return &m.RRCInactiveTransitionReportRequest
	}),
	{
		ID: IDGUAMI, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.GUAMI)
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.GUAMI, true },
	},
	apimpl.OptionalIE(IDRedirectionVoiceFallback, CriticalityIgnore, func(m *HandoverRequest) **RedirectionVoiceFallback { return &m.RedirectionVoiceFallback }),
	apimpl.OptionalIE(IDCNAssistedRANTuning, CriticalityIgnore, func(m *HandoverRequest) **CNAssistedRANTuning { return &m.CNAssistedRANTuning }),
	apimpl.OptionalIE(IDSRVCCOperationPossible, CriticalityIgnore, func(m *HandoverRequest) **SRVCCOperationPossible { return &m.SRVCCOperationPossible }),
	apimpl.OptionalIE(IDIABAuthorized, CriticalityReject, func(m *HandoverRequest) **IABAuthorized { return &m.IABAuthorized }),
	apimpl.OptionalIE(IDEnhancedCoverageRestriction, CriticalityIgnore, func(m *HandoverRequest) **EnhancedCoverageRestriction { return &m.EnhancedCoverageRestriction }),
	apimpl.OptionalIE(IDUEDifferentiationInfo, CriticalityIgnore, func(m *HandoverRequest) **UEDifferentiationInfo { return &m.UEDifferentiationInfo }),
	apimpl.OptionalIE(IDNRV2XServicesAuthorized, CriticalityIgnore, func(m *HandoverRequest) **NRV2XServicesAuthorized { return &m.NRV2XServicesAuthorized }),
	apimpl.OptionalIE(IDLTEV2XServicesAuthorized, CriticalityIgnore, func(m *HandoverRequest) **LTEV2XServicesAuthorized { return &m.LTEV2XServicesAuthorized }),
	apimpl.OptionalIE(IDNRUESidelinkAggregateMaximumBitrate, CriticalityIgnore, func(m *HandoverRequest) **NRUESidelinkAggregateMaximumBitrate {
		return &m.NRUESidelinkAggregateMaximumBitrate
	}),
	apimpl.OptionalIE(IDLTEUESidelinkAggregateMaximumBitrate, CriticalityIgnore, func(m *HandoverRequest) **LTEUESidelinkAggregateMaximumBitrate {
		return &m.LTEUESidelinkAggregateMaximumBitrate
	}),
	apimpl.OptionalIE(IDPC5QoSParameters, CriticalityIgnore, func(m *HandoverRequest) **PC5QoSParameters { return &m.PC5QoSParameters }),
	apimpl.OptionalIE(IDCEmodeBrestricted, CriticalityIgnore, func(m *HandoverRequest) **CEmodeBrestricted { return &m.CEmodeBrestricted }),
	apimpl.OptionalIE(IDUEUPCIoTSupport, CriticalityIgnore, func(m *HandoverRequest) **UEUPCIoTSupport { return &m.UEUPCIoTSupport }),
	apimpl.ListIE(IDManagementBasedMDTPLMNList, CriticalityIgnore, func(m *HandoverRequest) *MDTPLMNList { return &m.ManagementBasedMDTPLMNList }),
	apimpl.OptionalIE(IDUERadioCapabilityID, CriticalityReject, func(m *HandoverRequest) **UERadioCapabilityID { return &m.UERadioCapabilityID }),
	apimpl.OptionalIE(IDExtendedConnectedTime, CriticalityIgnore, func(m *HandoverRequest) **ExtendedConnectedTime { return &m.ExtendedConnectedTime }),
	apimpl.OptionalIE(IDTimeSyncAssistanceInfo, CriticalityIgnore, func(m *HandoverRequest) **TimeSyncAssistanceInfo { return &m.TimeSyncAssistanceInfo }),
	apimpl.ListIE(IDUESliceMaximumBitRateList, CriticalityIgnore, func(m *HandoverRequest) *UESliceMaximumBitRateList { return &m.UESliceMaximumBitRateList }),
	apimpl.OptionalIE(IDFiveGProSeAuthorized, CriticalityIgnore, func(m *HandoverRequest) **FiveGProSeAuthorized { return &m.FiveGProSeAuthorized }),
	apimpl.OptionalIE(IDFiveGProSeUEPC5AggregateMaximumBitRate, CriticalityIgnore, func(m *HandoverRequest) **NRUESidelinkAggregateMaximumBitrate {
		return &m.FiveGProSeUEPC5AggregateMaximumBitRate
	}),
	apimpl.OptionalIE(IDFiveGProSePC5QoSParameters, CriticalityIgnore, func(m *HandoverRequest) **FiveGProSePC5QoSParameters { return &m.FiveGProSePC5QoSParameters }),
	apimpl.OptionalIE(IDAerialUEsubscriptionInformation, CriticalityIgnore, func(m *HandoverRequest) **AerialUEsubscriptionInformation { return &m.AerialUEsubscriptionInformation }),
	apimpl.OptionalIE(IDNRA2XServicesAuthorized, CriticalityIgnore, func(m *HandoverRequest) **NRA2XServicesAuthorized { return &m.NRA2XServicesAuthorized }),
	apimpl.OptionalIE(IDLTEA2XServicesAuthorized, CriticalityIgnore, func(m *HandoverRequest) **LTEA2XServicesAuthorized { return &m.LTEA2XServicesAuthorized }),
	apimpl.OptionalIE(IDNRA2XUEPC5AggregateMaximumBitRate, CriticalityIgnore, func(m *HandoverRequest) **NRUESidelinkAggregateMaximumBitrate {
		return &m.NRA2XUEPC5AggregateMaximumBitRate
	}),
	apimpl.OptionalIE(IDLTEA2XUEPC5AggregateMaximumBitRate, CriticalityIgnore, func(m *HandoverRequest) **LTEUESidelinkAggregateMaximumBitrate {
		return &m.LTEA2XUEPC5AggregateMaximumBitRate
	}),
	apimpl.OptionalIE(IDA2XPC5QoSParameters, CriticalityIgnore, func(m *HandoverRequest) **A2XPC5QoSParameters { return &m.A2XPC5QoSParameters }),
	apimpl.OptionalIE(IDMobileIABAuthorized, CriticalityIgnore, func(m *HandoverRequest) **MobileIABAuthorized { return &m.MobileIABAuthorized }),
	apimpl.OptionalIE(IDNoPDUSessionIndication, CriticalityIgnore, func(m *HandoverRequest) **NoPDUSessionIndication { return &m.NoPDUSessionIndication }),
	apimpl.ListIE(IDPartiallyAllowedNSSAI, CriticalityIgnore, func(m *HandoverRequest) *PartiallyAllowedNSSAI { return &m.PartiallyAllowedNSSAI }),
	apimpl.OptionalIE(IDSLPositioningRangingServiceInfo, CriticalityIgnore, func(m *HandoverRequest) **SLPositioningRangingServiceInfo { return &m.SLPositioningRangingServiceInfo }),
	{
		ID: IDAMFUENGAPID2, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFUENGAPID2 = &v

			return nil
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.AMFUENGAPID2 == nil {
				return nil, false
			}

			return m.AMFUENGAPID2, true
		},
	},
	{
		ID: IDTargetRANNodeID, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			var v GlobalRANNodeID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.TargetRANNodeID = &v

			return nil
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.TargetRANNodeID == nil {
				return nil, false
			}

			return m.TargetRANNodeID, true
		},
	},
}

func (m *HandoverRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverResourceAllocation, handoverRequestIEs, m)
}

func (m *HandoverRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcHandoverResourceAllocation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseHandoverRequest(value []byte) (*HandoverRequest, error) {
	return parseMessageBody[HandoverRequest](ProcHandoverResourceAllocation, TriggeringInitiatingMessage, handoverRequestIEs, value)
}

func DecodeHandoverRequest(value []byte) (*HandoverRequest, error) {
	return decodeMessageBody[HandoverRequest](ProcHandoverResourceAllocation, handoverRequestIEs, value)
}

// TS 38.413 §9.2.3.5. The target NG-RAN node reports what it admitted.
// TS 36.413 §9.1.5.5 carries the same five IEs plus Criticality Diagnostics.
type HandoverRequestAcknowledge struct {
	AMFUENGAPID                        *AMFUENGAPID
	RANUENGAPID                        *RANUENGAPID
	PDUSessionResourceAdmittedList     PDUSessionResourceAdmittedList
	PDUSessionResourceFailedToSetup    PDUSessionResourceFailedToSetupListHOAck
	TargetToSourceTransparentContainer TargetToSourceTransparentContainer
	CriticalityDiagnostics             *CriticalityDiagnostics
	NPNAccessInformation               *NPNAccessInformation
	RedCapIndication                   *RedCapIndication
	ERedCapIndication                  *ERedCapIndication

	messageMeta
}

var handoverRequestAcknowledgeIEs = []ieSpec[HandoverRequestAcknowledge]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		Encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			var v RANUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		Encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		ID: IDPDUSessionResourceAdmittedList, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceAdmittedList)
		},
		Encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.PDUSessionResourceAdmittedList == nil {
				return nil, false
			}

			return m.PDUSessionResourceAdmittedList, true
		},
	},
	{
		ID: IDPDUSessionResourceFailedToSetupListHOAck, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceFailedToSetup)
		},
		Encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.PDUSessionResourceFailedToSetup == nil {
				return nil, false
			}

			return m.PDUSessionResourceFailedToSetup, true
		},
	},
	{
		ID: IDTargetToSourceTransparentContainer, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.TargetToSourceTransparentContainer)
		},
		Encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.TargetToSourceTransparentContainer == nil {
				return nil, false
			}

			return m.TargetToSourceTransparentContainer, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		Encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	apimpl.OptionalIE(IDNPNAccessInformation, CriticalityReject, func(m *HandoverRequestAcknowledge) **NPNAccessInformation { return &m.NPNAccessInformation }),
	apimpl.OptionalIE(IDRedCapIndication, CriticalityIgnore, func(m *HandoverRequestAcknowledge) **RedCapIndication { return &m.RedCapIndication }),
	apimpl.OptionalIE(IDERedCapIndication, CriticalityIgnore, func(m *HandoverRequestAcknowledge) **ERedCapIndication { return &m.ERedCapIndication }),
}

func (m *HandoverRequestAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverResourceAllocation, handoverRequestAcknowledgeIEs, m)
}

func (m *HandoverRequestAcknowledge) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcHandoverResourceAllocation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseHandoverRequestAcknowledge(value []byte) (*HandoverRequestAcknowledge, error) {
	return parseMessageBody[HandoverRequestAcknowledge](ProcHandoverResourceAllocation, TriggeringSuccessfulOutcome, handoverRequestAcknowledgeIEs, value)
}

func DecodeHandoverRequestAcknowledge(value []byte) (*HandoverRequestAcknowledge, error) {
	return decodeMessageBody[HandoverRequestAcknowledge](ProcHandoverResourceAllocation, handoverRequestAcknowledgeIEs, value)
}

// TS 38.413 §9.2.3.6. The target NG-RAN node admitted nothing and holds no UE
// context. TS 36.413 §9.1.5.6 carries the same three IEs; NGAP adds the failure
// container, which has no S1AP counterpart.
type HandoverFailure struct {
	AMFUENGAPID                               *AMFUENGAPID
	Cause                                     *Cause
	CriticalityDiagnostics                    *CriticalityDiagnostics
	TargettoSourceFailureTransparentContainer TargettoSourceFailureTransparentContainer

	messageMeta
}

var handoverFailureIEs = []ieSpec[HandoverFailure]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverFailure, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		Encode: func(m *HandoverFailure) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverFailure, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *HandoverFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverFailure, r *per.Reader, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		Encode: func(m *HandoverFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		ID: IDTargettoSourceFailureTransparentContainer, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverFailure, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.TargettoSourceFailureTransparentContainer)
		},
		Encode: func(m *HandoverFailure) (per.Marshaler, bool) {
			if m.TargettoSourceFailureTransparentContainer == nil {
				return nil, false
			}

			return m.TargettoSourceFailureTransparentContainer, true
		},
	},
}

func (m *HandoverFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverResourceAllocation, handoverFailureIEs, m)
}

func (m *HandoverFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcHandoverResourceAllocation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseHandoverFailure(value []byte) (*HandoverFailure, error) {
	return parseMessageBody[HandoverFailure](ProcHandoverResourceAllocation, TriggeringUnsuccessfulOutcome, handoverFailureIEs, value)
}

func DecodeHandoverFailure(value []byte) (*HandoverFailure, error) {
	return decodeMessageBody[HandoverFailure](ProcHandoverResourceAllocation, handoverFailureIEs, value)
}
