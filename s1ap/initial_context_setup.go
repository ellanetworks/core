// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// TS 36.413 §9.1.4.1.
type InitialContextSetupRequest struct {
	MMEUES1APID                            MMEUES1APID
	ENBUES1APID                            ENBUES1APID
	UEAggregateMaximumBitRate              UEAggregateMaximumBitRate
	ERABToBeSetup                          ERABToBeSetupListCtxtSUReq
	UESecurityCapabilities                 UESecurityCapabilities
	SecurityKey                            SecurityKey
	UERadioCapability                      UERadioCapability
	GUMMEI                                 *GUMMEI
	MMEUES1APID2                           *MMEUES1APID
	CoarseUELocation                       *CoarseUELocation
	TimeRefDistribution                    *TimeRefDistribution
	TraceActivation                        *TraceActivation
	HandoverRestrictionList                *HandoverRestrictionList
	SubscriberProfileIDforRFP              *SubscriberProfileIDforRFP
	CSFallbackIndicator                    *CSFallbackIndicator
	SRVCCOperationPossible                 *SRVCCOperationPossible
	CSGMembershipStatus                    *CSGMembershipStatus
	RegisteredLAI                          *LAI
	ManagementBasedMDTAllowed              *ManagementBasedMDTAllowed
	ManagementBasedMDTPLMNList             MDTPLMNList
	AdditionalCSFallbackIndicator          *AdditionalCSFallbackIndicator
	MaskedIMEISV                           *MaskedIMEISV
	ExpectedUEBehaviour                    *ExpectedUEBehaviour
	ProSeAuthorized                        *ProSeAuthorized
	UEUserPlaneCIoTSupportIndicator        *UEUserPlaneCIoTSupportIndicator
	V2XServicesAuthorized                  *V2XServicesAuthorized
	UESidelinkAggregateMaximumBitrate      *UESidelinkAggregateMaximumBitrate
	EnhancedCoverageRestricted             *EnhancedCoverageRestricted
	NRUESecurityCapabilities               *NRUESecurityCapabilities
	CEModeBRestricted                      *CEModeBRestricted
	AerialUEsubscriptionInformation        *AerialUEsubscriptionInformation
	PendingDataIndication                  *PendingDataIndication
	SubscriptionBasedUEDifferentiationInfo *SubscriptionBasedUEDifferentiationInfo
	AdditionalRRMPriorityIndex             *AdditionalRRMPriorityIndex
	IABAuthorized                          *IABAuthorized
	NRV2XServicesAuthorized                *NRV2XServicesAuthorized
	NRUESidelinkAggregateMaximumBitrate    *NRUESidelinkAggregateMaximumBitrate
	PC5QoSParameters                       *PC5QoSParameters
	UERadioCapabilityID                    *UERadioCapabilityID

	messageMeta
}

var initialContextSetupRequestIEs = []ieSpec[InitialContextSetupRequest]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDUEAggregateMaximumBitrate, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.UEAggregateMaximumBitRate)
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return &m.UEAggregateMaximumBitRate, true },
	},
	{
		ID: IDERABToBeSetupListCtxtSUReq, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABToBeSetup, err = decodeItemList[ERABToBeSetupItemCtxtSUReq](r, enc, maxnoofERABs, IDERABToBeSetupItemCtxtSUReq, CriticalityReject)

			return err
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			return containerList(maxnoofERABs, IDERABToBeSetupItemCtxtSUReq, CriticalityReject, &m.ERABToBeSetup), true
		},
	},
	{
		ID: IDUESecurityCapabilities, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.UESecurityCapabilities)
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return &m.UESecurityCapabilities, true },
	},
	{
		ID: IDSecurityKey, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SecurityKey)
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return &m.SecurityKey, true },
	},
	apimpl.OptionalIE(IDTraceActivation, CriticalityIgnore, func(m *InitialContextSetupRequest) **TraceActivation { return &m.TraceActivation }),
	apimpl.OptionalIE(IDHandoverRestrictionList, CriticalityIgnore, func(m *InitialContextSetupRequest) **HandoverRestrictionList { return &m.HandoverRestrictionList }),
	{
		ID: IDUERadioCapability, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.UERadioCapability)
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			if m.UERadioCapability == nil {
				return nil, false
			}

			return m.UERadioCapability, true
		},
	},
	apimpl.OptionalIE(IDSubscriberProfileIDforRFP, CriticalityIgnore, func(m *InitialContextSetupRequest) **SubscriberProfileIDforRFP { return &m.SubscriberProfileIDforRFP }),
	apimpl.OptionalIE(IDCSFallbackIndicator, CriticalityReject, func(m *InitialContextSetupRequest) **CSFallbackIndicator { return &m.CSFallbackIndicator }),
	apimpl.OptionalIE(IDSRVCCOperationPossible, CriticalityIgnore, func(m *InitialContextSetupRequest) **SRVCCOperationPossible { return &m.SRVCCOperationPossible }),
	apimpl.OptionalIE(IDCSGMembershipStatus, CriticalityIgnore, func(m *InitialContextSetupRequest) **CSGMembershipStatus { return &m.CSGMembershipStatus }),
	apimpl.OptionalIE(IDRegisteredLAI, CriticalityIgnore, func(m *InitialContextSetupRequest) **LAI { return &m.RegisteredLAI }),
	{
		ID: IDGUMMEI, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			var v GUMMEI

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.GUMMEI = &v

			return nil
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			if m.GUMMEI == nil {
				return nil, false
			}

			return m.GUMMEI, true
		},
	},
	{
		ID: IDMMEUES1APID2, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			var v MMEUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.MMEUES1APID2 = &v

			return nil
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			if m.MMEUES1APID2 == nil {
				return nil, false
			}

			return m.MMEUES1APID2, true
		},
	},
	apimpl.OptionalIE(IDManagementBasedMDTAllowed, CriticalityIgnore, func(m *InitialContextSetupRequest) **ManagementBasedMDTAllowed { return &m.ManagementBasedMDTAllowed }),
	apimpl.ListIE(IDManagementBasedMDTPLMNList, CriticalityIgnore, func(m *InitialContextSetupRequest) *MDTPLMNList { return &m.ManagementBasedMDTPLMNList }),
	apimpl.ConditionalIE(IDAdditionalCSFallbackIndicator, CriticalityIgnore, initialContextSetupCSFBHighPriority, func(m *InitialContextSetupRequest) **AdditionalCSFallbackIndicator {
		return &m.AdditionalCSFallbackIndicator
	}),
	apimpl.OptionalIE(IDMaskedIMEISV, CriticalityIgnore, func(m *InitialContextSetupRequest) **MaskedIMEISV { return &m.MaskedIMEISV }),
	apimpl.OptionalIE(IDExpectedUEBehaviour, CriticalityIgnore, func(m *InitialContextSetupRequest) **ExpectedUEBehaviour { return &m.ExpectedUEBehaviour }),
	apimpl.OptionalIE(IDProSeAuthorized, CriticalityIgnore, func(m *InitialContextSetupRequest) **ProSeAuthorized { return &m.ProSeAuthorized }),
	apimpl.OptionalIE(IDUEUserPlaneCIoTSupportIndicator, CriticalityIgnore, func(m *InitialContextSetupRequest) **UEUserPlaneCIoTSupportIndicator {
		return &m.UEUserPlaneCIoTSupportIndicator
	}),
	apimpl.OptionalIE(IDV2XServicesAuthorized, CriticalityIgnore, func(m *InitialContextSetupRequest) **V2XServicesAuthorized { return &m.V2XServicesAuthorized }),
	apimpl.OptionalIE(IDUESidelinkAggregateMaximumBitrate, CriticalityIgnore, func(m *InitialContextSetupRequest) **UESidelinkAggregateMaximumBitrate {
		return &m.UESidelinkAggregateMaximumBitrate
	}),
	apimpl.OptionalIE(IDEnhancedCoverageRestricted, CriticalityIgnore, func(m *InitialContextSetupRequest) **EnhancedCoverageRestricted { return &m.EnhancedCoverageRestricted }),
	apimpl.OptionalIE(IDNRUESecurityCapabilities, CriticalityIgnore, func(m *InitialContextSetupRequest) **NRUESecurityCapabilities { return &m.NRUESecurityCapabilities }),
	apimpl.OptionalIE(IDCEModeBRestricted, CriticalityIgnore, func(m *InitialContextSetupRequest) **CEModeBRestricted { return &m.CEModeBRestricted }),
	apimpl.OptionalIE(IDAerialUEsubscriptionInformation, CriticalityIgnore, func(m *InitialContextSetupRequest) **AerialUEsubscriptionInformation {
		return &m.AerialUEsubscriptionInformation
	}),
	apimpl.OptionalIE(IDPendingDataIndication, CriticalityIgnore, func(m *InitialContextSetupRequest) **PendingDataIndication { return &m.PendingDataIndication }),
	apimpl.OptionalIE(IDSubscriptionBasedUEDifferentiationInfo, CriticalityIgnore, func(m *InitialContextSetupRequest) **SubscriptionBasedUEDifferentiationInfo {
		return &m.SubscriptionBasedUEDifferentiationInfo
	}),
	apimpl.OptionalIE(IDAdditionalRRMPriorityIndex, CriticalityIgnore, func(m *InitialContextSetupRequest) **AdditionalRRMPriorityIndex { return &m.AdditionalRRMPriorityIndex }),
	apimpl.OptionalIE(IDIABAuthorized, CriticalityIgnore, func(m *InitialContextSetupRequest) **IABAuthorized { return &m.IABAuthorized }),
	apimpl.OptionalIE(IDNRV2XServicesAuthorized, CriticalityIgnore, func(m *InitialContextSetupRequest) **NRV2XServicesAuthorized { return &m.NRV2XServicesAuthorized }),
	apimpl.OptionalIE(IDNRUESidelinkAggregateMaximumBitrate, CriticalityIgnore, func(m *InitialContextSetupRequest) **NRUESidelinkAggregateMaximumBitrate {
		return &m.NRUESidelinkAggregateMaximumBitrate
	}),
	apimpl.OptionalIE(IDPC5QoSParameters, CriticalityIgnore, func(m *InitialContextSetupRequest) **PC5QoSParameters { return &m.PC5QoSParameters }),
	apimpl.OptionalIE(IDUERadioCapabilityID, CriticalityReject, func(m *InitialContextSetupRequest) **UERadioCapabilityID { return &m.UERadioCapabilityID }),
	{
		ID: IDCoarseUELocation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.CoarseUELocation)
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return optionalIE(m.CoarseUELocation) },
	},
	{
		ID: IDTimeRefDistribution, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.TimeRefDistribution)
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return optionalIE(m.TimeRefDistribution) },
	},
}

func (m *InitialContextSetupRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcInitialContextSetup, initialContextSetupRequestIEs, m)
}

func (m *InitialContextSetupRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcInitialContextSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseInitialContextSetupRequest(value []byte) (*InitialContextSetupRequest, error) {
	return parseMessageBody[InitialContextSetupRequest](ProcInitialContextSetup, TriggeringInitiatingMessage, initialContextSetupRequestIEs, value)
}

func DecodeInitialContextSetupRequest(value []byte) (*InitialContextSetupRequest, error) {
	return decodeMessageBody[InitialContextSetupRequest](ProcInitialContextSetup, initialContextSetupRequestIEs, value)
}

// TS 36.413 §9.1.4.3.
type InitialContextSetupResponse struct {
	MMEUES1APID            *MMEUES1APID
	ENBUES1APID            *ENBUES1APID
	ERABSetup              ERABSetupListCtxtSURes
	ERABFailedToSetup      []ERABItem
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var initialContextSetupResponseIEs = []ieSpec[InitialContextSetupResponse]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupResponse, r *per.Reader, enc per.Encoding) error {
			var v MMEUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		Encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupResponse, r *per.Reader, enc per.Encoding) error {
			var v ENBUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		Encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		ID: IDERABSetupListCtxtSURes, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupResponse, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABSetup, err = decodeItemList[ERABSetupItemCtxtSURes](r, enc, maxnoofERABs, IDERABSetupItemCtxtSURes, CriticalityIgnore)

			return err
		},
		Encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) {
			if len(m.ERABSetup) == 0 {
				return nil, false
			}

			return containerList(maxnoofERABs, IDERABSetupItemCtxtSURes, CriticalityIgnore, &m.ERABSetup), true
		},
	},
	{
		ID: IDERABFailedToSetupListCtxtSURes, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupResponse, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABFailedToSetup, err = decodeItemList[ERABItem](r, enc, maxnoofERABs, IDERABItem, CriticalityIgnore)

			return err
		},
		Encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) {
			if len(m.ERABFailedToSetup) == 0 {
				return nil, false
			}

			return containerList(maxnoofERABs, IDERABItem, CriticalityIgnore, &m.ERABFailedToSetup), true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupResponse, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *InitialContextSetupResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcInitialContextSetup, initialContextSetupResponseIEs, m)
}

func (m *InitialContextSetupResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcInitialContextSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseInitialContextSetupResponse(value []byte) (*InitialContextSetupResponse, error) {
	return parseMessageBody[InitialContextSetupResponse](ProcInitialContextSetup, TriggeringSuccessfulOutcome, initialContextSetupResponseIEs, value)
}

func DecodeInitialContextSetupResponse(value []byte) (*InitialContextSetupResponse, error) {
	return decodeMessageBody[InitialContextSetupResponse](ProcInitialContextSetup, initialContextSetupResponseIEs, value)
}

// TS 36.413 §9.1.4.4.
type InitialContextSetupFailure struct {
	MMEUES1APID            *MMEUES1APID
	ENBUES1APID            *ENBUES1APID
	Cause                  *Cause
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var initialContextSetupFailureIEs = []ieSpec[InitialContextSetupFailure]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupFailure, r *per.Reader, enc per.Encoding) error {
			var v MMEUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		Encode: func(m *InitialContextSetupFailure) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupFailure, r *per.Reader, enc per.Encoding) error {
			var v ENBUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		Encode: func(m *InitialContextSetupFailure) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupFailure, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *InitialContextSetupFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupFailure, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *InitialContextSetupFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *InitialContextSetupFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcInitialContextSetup, initialContextSetupFailureIEs, m)
}

func (m *InitialContextSetupFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcInitialContextSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseInitialContextSetupFailure(value []byte) (*InitialContextSetupFailure, error) {
	return parseMessageBody[InitialContextSetupFailure](ProcInitialContextSetup, TriggeringUnsuccessfulOutcome, initialContextSetupFailureIEs, value)
}

func DecodeInitialContextSetupFailure(value []byte) (*InitialContextSetupFailure, error) {
	return decodeMessageBody[InitialContextSetupFailure](ProcInitialContextSetup, initialContextSetupFailureIEs, value)
}

func initialContextSetupCSFBHighPriority(m *InitialContextSetupRequest) bool {
	return m.CSFallbackIndicator != nil && *m.CSFallbackIndicator == CSFallbackIndicatorCSFallbackHighPriority
}
