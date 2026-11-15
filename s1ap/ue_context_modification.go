// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

type UEContextModificationRequest struct {
	MMEUES1APID                         MMEUES1APID
	ENBUES1APID                         ENBUES1APID
	UEAggregateMaximumBitRate           *UEAggregateMaximumBitRate
	TimeRefDistribution                 *TimeRefDistribution
	SecurityKey                         *SecurityKey
	SubscriberProfileIDforRFP           *SubscriberProfileIDforRFP
	CSFallbackIndicator                 *CSFallbackIndicator
	UESecurityCapabilities              *UESecurityCapabilities
	CSGMembershipStatus                 *CSGMembershipStatus
	RegisteredLAI                       *LAI
	AdditionalCSFallbackIndicator       *AdditionalCSFallbackIndicator
	ProSeAuthorized                     *ProSeAuthorized
	SRVCCOperationPossible              *SRVCCOperationPossible
	SRVCCOperationNotPossible           *SRVCCOperationNotPossible
	V2XServicesAuthorized               *V2XServicesAuthorized
	UESidelinkAggregateMaximumBitrate   *UESidelinkAggregateMaximumBitrate
	NRUESecurityCapabilities            *NRUESecurityCapabilities
	AerialUEsubscriptionInformation     *AerialUEsubscriptionInformation
	AdditionalRRMPriorityIndex          *AdditionalRRMPriorityIndex
	IABAuthorized                       *IABAuthorized
	NRV2XServicesAuthorized             *NRV2XServicesAuthorized
	NRUESidelinkAggregateMaximumBitrate *NRUESidelinkAggregateMaximumBitrate
	PC5QoSParameters                    *PC5QoSParameters
	UERadioCapabilityID                 *UERadioCapabilityID

	messageMeta
}

var uEContextModificationRequestIEs = []ieSpec[UEContextModificationRequest]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UEContextModificationRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *UEContextModificationRequest) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UEContextModificationRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *UEContextModificationRequest) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	apimpl.OptionalIE(IDSecurityKey, CriticalityReject, func(m *UEContextModificationRequest) **SecurityKey { return &m.SecurityKey }),
	apimpl.OptionalIE(IDSubscriberProfileIDforRFP, CriticalityIgnore, func(m *UEContextModificationRequest) **SubscriberProfileIDforRFP { return &m.SubscriberProfileIDforRFP }),
	{
		ID: IDUEAggregateMaximumBitrate, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextModificationRequest, r *per.Reader, enc per.Encoding) error {
			var ambr UEAggregateMaximumBitRate

			if err := decodeIE(r, &ambr); err != nil {
				return err
			}

			m.UEAggregateMaximumBitRate = &ambr

			return nil
		},
		Encode: func(m *UEContextModificationRequest) (per.Marshaler, bool) {
			if m.UEAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.UEAggregateMaximumBitRate, true
		},
	},
	apimpl.OptionalIE(IDCSFallbackIndicator, CriticalityReject, func(m *UEContextModificationRequest) **CSFallbackIndicator { return &m.CSFallbackIndicator }),
	apimpl.OptionalIE(IDUESecurityCapabilities, CriticalityReject, func(m *UEContextModificationRequest) **UESecurityCapabilities { return &m.UESecurityCapabilities }),
	apimpl.OptionalIE(IDCSGMembershipStatus, CriticalityIgnore, func(m *UEContextModificationRequest) **CSGMembershipStatus { return &m.CSGMembershipStatus }),
	apimpl.OptionalIE(IDRegisteredLAI, CriticalityIgnore, func(m *UEContextModificationRequest) **LAI { return &m.RegisteredLAI }),
	apimpl.ConditionalIE(IDAdditionalCSFallbackIndicator, CriticalityIgnore, ueContextModificationCSFBHighPriority, func(m *UEContextModificationRequest) **AdditionalCSFallbackIndicator {
		return &m.AdditionalCSFallbackIndicator
	}),
	apimpl.OptionalIE(IDProSeAuthorized, CriticalityIgnore, func(m *UEContextModificationRequest) **ProSeAuthorized { return &m.ProSeAuthorized }),
	apimpl.OptionalIE(IDSRVCCOperationPossible, CriticalityIgnore, func(m *UEContextModificationRequest) **SRVCCOperationPossible { return &m.SRVCCOperationPossible }),
	apimpl.OptionalIE(IDSRVCCOperationNotPossible, CriticalityIgnore, func(m *UEContextModificationRequest) **SRVCCOperationNotPossible { return &m.SRVCCOperationNotPossible }),
	apimpl.OptionalIE(IDV2XServicesAuthorized, CriticalityIgnore, func(m *UEContextModificationRequest) **V2XServicesAuthorized { return &m.V2XServicesAuthorized }),
	apimpl.OptionalIE(IDUESidelinkAggregateMaximumBitrate, CriticalityIgnore, func(m *UEContextModificationRequest) **UESidelinkAggregateMaximumBitrate {
		return &m.UESidelinkAggregateMaximumBitrate
	}),
	apimpl.OptionalIE(IDNRUESecurityCapabilities, CriticalityIgnore, func(m *UEContextModificationRequest) **NRUESecurityCapabilities { return &m.NRUESecurityCapabilities }),
	apimpl.OptionalIE(IDAerialUEsubscriptionInformation, CriticalityIgnore, func(m *UEContextModificationRequest) **AerialUEsubscriptionInformation {
		return &m.AerialUEsubscriptionInformation
	}),
	apimpl.OptionalIE(IDAdditionalRRMPriorityIndex, CriticalityIgnore, func(m *UEContextModificationRequest) **AdditionalRRMPriorityIndex {
		return &m.AdditionalRRMPriorityIndex
	}),
	apimpl.OptionalIE(IDIABAuthorized, CriticalityIgnore, func(m *UEContextModificationRequest) **IABAuthorized { return &m.IABAuthorized }),
	apimpl.OptionalIE(IDNRV2XServicesAuthorized, CriticalityIgnore, func(m *UEContextModificationRequest) **NRV2XServicesAuthorized { return &m.NRV2XServicesAuthorized }),
	apimpl.OptionalIE(IDNRUESidelinkAggregateMaximumBitrate, CriticalityIgnore, func(m *UEContextModificationRequest) **NRUESidelinkAggregateMaximumBitrate {
		return &m.NRUESidelinkAggregateMaximumBitrate
	}),
	apimpl.OptionalIE(IDPC5QoSParameters, CriticalityIgnore, func(m *UEContextModificationRequest) **PC5QoSParameters { return &m.PC5QoSParameters }),
	apimpl.OptionalIE(IDUERadioCapabilityID, CriticalityReject, func(m *UEContextModificationRequest) **UERadioCapabilityID { return &m.UERadioCapabilityID }),
	{
		ID: IDTimeRefDistribution, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextModificationRequest, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.TimeRefDistribution)
		},
		Encode: func(m *UEContextModificationRequest) (per.Marshaler, bool) { return optionalIE(m.TimeRefDistribution) },
	},
}

func (m *UEContextModificationRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUEContextModification, uEContextModificationRequestIEs, m)
}

func (m *UEContextModificationRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUEContextModification,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseUEContextModificationRequest(value []byte) (*UEContextModificationRequest, error) {
	return parseMessageBody[UEContextModificationRequest](ProcUEContextModification, TriggeringInitiatingMessage, uEContextModificationRequestIEs, value)
}

func DecodeUEContextModificationRequest(value []byte) (*UEContextModificationRequest, error) {
	return decodeMessageBody[UEContextModificationRequest](ProcUEContextModification, uEContextModificationRequestIEs, value)
}

type UEContextModificationResponse struct {
	MMEUES1APID            *MMEUES1APID
	ENBUES1APID            *ENBUES1APID
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var uEContextModificationResponseIEs = []ieSpec[UEContextModificationResponse]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextModificationResponse, r *per.Reader, enc per.Encoding) error {
			var v MMEUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		Encode: func(m *UEContextModificationResponse) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextModificationResponse, r *per.Reader, enc per.Encoding) error {
			var v ENBUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		Encode: func(m *UEContextModificationResponse) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextModificationResponse, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *UEContextModificationResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *UEContextModificationResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUEContextModification, uEContextModificationResponseIEs, m)
}

func (m *UEContextModificationResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcUEContextModification,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseUEContextModificationResponse(value []byte) (*UEContextModificationResponse, error) {
	return parseMessageBody[UEContextModificationResponse](ProcUEContextModification, TriggeringSuccessfulOutcome, uEContextModificationResponseIEs, value)
}

func DecodeUEContextModificationResponse(value []byte) (*UEContextModificationResponse, error) {
	return decodeMessageBody[UEContextModificationResponse](ProcUEContextModification, uEContextModificationResponseIEs, value)
}

type UEContextModificationFailure struct {
	MMEUES1APID            *MMEUES1APID
	ENBUES1APID            *ENBUES1APID
	Cause                  *Cause
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var uEContextModificationFailureIEs = []ieSpec[UEContextModificationFailure]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextModificationFailure, r *per.Reader, enc per.Encoding) error {
			var v MMEUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		Encode: func(m *UEContextModificationFailure) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextModificationFailure, r *per.Reader, enc per.Encoding) error {
			var v ENBUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		Encode: func(m *UEContextModificationFailure) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextModificationFailure, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *UEContextModificationFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextModificationFailure, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *UEContextModificationFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *UEContextModificationFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUEContextModification, uEContextModificationFailureIEs, m)
}

func (m *UEContextModificationFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcUEContextModification,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseUEContextModificationFailure(value []byte) (*UEContextModificationFailure, error) {
	return parseMessageBody[UEContextModificationFailure](ProcUEContextModification, TriggeringUnsuccessfulOutcome, uEContextModificationFailureIEs, value)
}

func DecodeUEContextModificationFailure(value []byte) (*UEContextModificationFailure, error) {
	return decodeMessageBody[UEContextModificationFailure](ProcUEContextModification, uEContextModificationFailureIEs, value)
}

func ueContextModificationCSFBHighPriority(m *UEContextModificationRequest) bool {
	return m.CSFallbackIndicator != nil && *m.CSFallbackIndicator == CSFallbackIndicatorCSFallbackHighPriority
}
