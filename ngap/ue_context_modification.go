// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

type UEContextModificationRequest struct {
	AMFUENGAPID                                 AMFUENGAPID
	RANUENGAPID                                 RANUENGAPID
	UEAggregateMaximumBitRate                   *UEAggregateMaximumBitRate
	NewAMFUENGAPID                              *AMFUENGAPID
	RANPagingPriority                           *RANPagingPriority
	SecurityKey                                 *SecurityKey
	IndexToRFSP                                 *IndexToRFSP
	UESecurityCapabilities                      *UESecurityCapabilities
	CoreNetworkAssistanceInformationForInactive *CoreNetworkAssistanceInformationForInactive
	EmergencyFallbackIndicator                  *EmergencyFallbackIndicator
	RRCInactiveTransitionReportRequest          *RRCInactiveTransitionReportRequest
	NewGUAMI                                    *GUAMI
	CNAssistedRANTuning                         *CNAssistedRANTuning
	SRVCCOperationPossible                      *SRVCCOperationPossible
	IABAuthorized                               *IABAuthorized
	NRV2XServicesAuthorized                     *NRV2XServicesAuthorized
	LTEV2XServicesAuthorized                    *LTEV2XServicesAuthorized
	NRUESidelinkAggregateMaximumBitrate         *NRUESidelinkAggregateMaximumBitrate
	LTEUESidelinkAggregateMaximumBitrate        *LTEUESidelinkAggregateMaximumBitrate
	PC5QoSParameters                            *PC5QoSParameters
	UERadioCapabilityID                         *UERadioCapabilityID
	RGLevelWirelineAccessCharacteristics        *RGLevelWirelineAccessCharacteristics
	TimeSyncAssistanceInfo                      *TimeSyncAssistanceInfo
	QMCConfigInfo                               *QMCConfigInfo
	QMCDeactivation                             *QMCDeactivation
	UESliceMaximumBitRateList                   UESliceMaximumBitRateList
	ManagementBasedMDTPLMNModificationList      MDTPLMNModificationList
	FiveGProSeAuthorized                        *FiveGProSeAuthorized
	FiveGProSeUEPC5AggregateMaximumBitRate      *NRUESidelinkAggregateMaximumBitrate
	FiveGProSePC5QoSParameters                  *FiveGProSePC5QoSParameters
	NetworkControlledRepeaterAuthorized         *NetworkControlledRepeaterAuthorized
	AerialUEsubscriptionInformation             *AerialUEsubscriptionInformation
	NRA2XServicesAuthorized                     *NRA2XServicesAuthorized
	LTEA2XServicesAuthorized                    *LTEA2XServicesAuthorized
	NRA2XUEPC5AggregateMaximumBitRate           *NRUESidelinkAggregateMaximumBitrate
	LTEA2XUEPC5AggregateMaximumBitRate          *LTEUESidelinkAggregateMaximumBitrate
	A2XPC5QoSParameters                         *A2XPC5QoSParameters
	MobileIABAuthorized                         *MobileIABAuthorized
	SLPositioningRangingServiceInfo             *SLPositioningRangingServiceInfo

	messageMeta
}

var uEContextModificationRequestIEs = []ieSpec[UEContextModificationRequest]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UEContextModificationRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFUENGAPID)
		},
		Encode: func(m *UEContextModificationRequest) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UEContextModificationRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *UEContextModificationRequest) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	apimpl.OptionalIE(IDRANPagingPriority, CriticalityIgnore, func(m *UEContextModificationRequest) **RANPagingPriority { return &m.RANPagingPriority }),
	apimpl.OptionalIE(IDSecurityKey, CriticalityReject, func(m *UEContextModificationRequest) **SecurityKey { return &m.SecurityKey }),
	apimpl.OptionalIE(IDIndexToRFSP, CriticalityIgnore, func(m *UEContextModificationRequest) **IndexToRFSP { return &m.IndexToRFSP }),
	{
		ID: IDUEAggregateMaximumBitRate, Presence: presenceOptional, Criticality: CriticalityIgnore,
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
	apimpl.OptionalIE(IDUESecurityCapabilities, CriticalityReject, func(m *UEContextModificationRequest) **UESecurityCapabilities { return &m.UESecurityCapabilities }),
	apimpl.OptionalIE(IDCoreNetworkAssistanceInformationForInactive, CriticalityIgnore, func(m *UEContextModificationRequest) **CoreNetworkAssistanceInformationForInactive {
		return &m.CoreNetworkAssistanceInformationForInactive
	}),
	apimpl.OptionalIE(IDEmergencyFallbackIndicator, CriticalityReject, func(m *UEContextModificationRequest) **EmergencyFallbackIndicator {
		return &m.EmergencyFallbackIndicator
	}),
	{
		ID: IDNewAMFUENGAPID, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *UEContextModificationRequest, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.NewAMFUENGAPID = &v

			return nil
		},
		Encode: func(m *UEContextModificationRequest) (per.Marshaler, bool) {
			if m.NewAMFUENGAPID == nil {
				return nil, false
			}

			return m.NewAMFUENGAPID, true
		},
	},
	apimpl.OptionalIE(IDRRCInactiveTransitionReportRequest, CriticalityIgnore, func(m *UEContextModificationRequest) **RRCInactiveTransitionReportRequest {
		return &m.RRCInactiveTransitionReportRequest
	}),
	apimpl.OptionalIE(IDNewGUAMI, CriticalityReject, func(m *UEContextModificationRequest) **GUAMI { return &m.NewGUAMI }),
	apimpl.OptionalIE(IDCNAssistedRANTuning, CriticalityIgnore, func(m *UEContextModificationRequest) **CNAssistedRANTuning { return &m.CNAssistedRANTuning }),
	apimpl.OptionalIE(IDSRVCCOperationPossible, CriticalityIgnore, func(m *UEContextModificationRequest) **SRVCCOperationPossible { return &m.SRVCCOperationPossible }),
	apimpl.OptionalIE(IDIABAuthorized, CriticalityIgnore, func(m *UEContextModificationRequest) **IABAuthorized { return &m.IABAuthorized }),
	apimpl.OptionalIE(IDNRV2XServicesAuthorized, CriticalityIgnore, func(m *UEContextModificationRequest) **NRV2XServicesAuthorized { return &m.NRV2XServicesAuthorized }),
	apimpl.OptionalIE(IDLTEV2XServicesAuthorized, CriticalityIgnore, func(m *UEContextModificationRequest) **LTEV2XServicesAuthorized { return &m.LTEV2XServicesAuthorized }),
	apimpl.OptionalIE(IDNRUESidelinkAggregateMaximumBitrate, CriticalityIgnore, func(m *UEContextModificationRequest) **NRUESidelinkAggregateMaximumBitrate {
		return &m.NRUESidelinkAggregateMaximumBitrate
	}),
	apimpl.OptionalIE(IDLTEUESidelinkAggregateMaximumBitrate, CriticalityIgnore, func(m *UEContextModificationRequest) **LTEUESidelinkAggregateMaximumBitrate {
		return &m.LTEUESidelinkAggregateMaximumBitrate
	}),
	apimpl.OptionalIE(IDPC5QoSParameters, CriticalityIgnore, func(m *UEContextModificationRequest) **PC5QoSParameters { return &m.PC5QoSParameters }),
	apimpl.OptionalIE(IDUERadioCapabilityID, CriticalityReject, func(m *UEContextModificationRequest) **UERadioCapabilityID { return &m.UERadioCapabilityID }),
	apimpl.OptionalIE(IDRGLevelWirelineAccessCharacteristics, CriticalityIgnore, func(m *UEContextModificationRequest) **RGLevelWirelineAccessCharacteristics {
		return &m.RGLevelWirelineAccessCharacteristics
	}),
	apimpl.OptionalIE(IDTimeSyncAssistanceInfo, CriticalityIgnore, func(m *UEContextModificationRequest) **TimeSyncAssistanceInfo { return &m.TimeSyncAssistanceInfo }),
	apimpl.OptionalIE(IDQMCConfigInfo, CriticalityIgnore, func(m *UEContextModificationRequest) **QMCConfigInfo { return &m.QMCConfigInfo }),
	apimpl.OptionalIE(IDQMCDeactivation, CriticalityIgnore, func(m *UEContextModificationRequest) **QMCDeactivation { return &m.QMCDeactivation }),
	apimpl.ListIE(IDUESliceMaximumBitRateList, CriticalityIgnore, func(m *UEContextModificationRequest) *UESliceMaximumBitRateList { return &m.UESliceMaximumBitRateList }),
	apimpl.ListIE(IDManagementBasedMDTPLMNModificationList, CriticalityIgnore, func(m *UEContextModificationRequest) *MDTPLMNModificationList {
		return &m.ManagementBasedMDTPLMNModificationList
	}),
	apimpl.OptionalIE(IDFiveGProSeAuthorized, CriticalityIgnore, func(m *UEContextModificationRequest) **FiveGProSeAuthorized { return &m.FiveGProSeAuthorized }),
	apimpl.OptionalIE(IDFiveGProSeUEPC5AggregateMaximumBitRate, CriticalityIgnore, func(m *UEContextModificationRequest) **NRUESidelinkAggregateMaximumBitrate {
		return &m.FiveGProSeUEPC5AggregateMaximumBitRate
	}),
	apimpl.OptionalIE(IDFiveGProSePC5QoSParameters, CriticalityIgnore, func(m *UEContextModificationRequest) **FiveGProSePC5QoSParameters {
		return &m.FiveGProSePC5QoSParameters
	}),
	apimpl.OptionalIE(IDNetworkControlledRepeaterAuthorized, CriticalityIgnore, func(m *UEContextModificationRequest) **NetworkControlledRepeaterAuthorized {
		return &m.NetworkControlledRepeaterAuthorized
	}),
	apimpl.OptionalIE(IDAerialUEsubscriptionInformation, CriticalityIgnore, func(m *UEContextModificationRequest) **AerialUEsubscriptionInformation {
		return &m.AerialUEsubscriptionInformation
	}),
	apimpl.OptionalIE(IDNRA2XServicesAuthorized, CriticalityIgnore, func(m *UEContextModificationRequest) **NRA2XServicesAuthorized { return &m.NRA2XServicesAuthorized }),
	apimpl.OptionalIE(IDLTEA2XServicesAuthorized, CriticalityIgnore, func(m *UEContextModificationRequest) **LTEA2XServicesAuthorized { return &m.LTEA2XServicesAuthorized }),
	apimpl.OptionalIE(IDNRA2XUEPC5AggregateMaximumBitRate, CriticalityIgnore, func(m *UEContextModificationRequest) **NRUESidelinkAggregateMaximumBitrate {
		return &m.NRA2XUEPC5AggregateMaximumBitRate
	}),
	apimpl.OptionalIE(IDLTEA2XUEPC5AggregateMaximumBitRate, CriticalityIgnore, func(m *UEContextModificationRequest) **LTEUESidelinkAggregateMaximumBitrate {
		return &m.LTEA2XUEPC5AggregateMaximumBitRate
	}),
	apimpl.OptionalIE(IDA2XPC5QoSParameters, CriticalityIgnore, func(m *UEContextModificationRequest) **A2XPC5QoSParameters { return &m.A2XPC5QoSParameters }),
	apimpl.OptionalIE(IDMobileIABAuthorized, CriticalityIgnore, func(m *UEContextModificationRequest) **MobileIABAuthorized { return &m.MobileIABAuthorized }),
	apimpl.OptionalIE(IDSLPositioningRangingServiceInfo, CriticalityIgnore, func(m *UEContextModificationRequest) **SLPositioningRangingServiceInfo {
		return &m.SLPositioningRangingServiceInfo
	}),
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
	AMFUENGAPID             *AMFUENGAPID
	RANUENGAPID             *RANUENGAPID
	UserLocationInformation *UserLocationInformation
	CriticalityDiagnostics  *CriticalityDiagnostics
	RRCState                *RRCState

	messageMeta
}

var uEContextModificationResponseIEs = []ieSpec[UEContextModificationResponse]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextModificationResponse, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		Encode: func(m *UEContextModificationResponse) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextModificationResponse, r *per.Reader, enc per.Encoding) error {
			var v RANUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		Encode: func(m *UEContextModificationResponse) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	apimpl.OptionalIE(IDRRCState, CriticalityIgnore, func(m *UEContextModificationResponse) **RRCState { return &m.RRCState }),
	{
		ID: IDUserLocationInformation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextModificationResponse, r *per.Reader, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := decodeIE(r, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		Encode: func(m *UEContextModificationResponse) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
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
	AMFUENGAPID            *AMFUENGAPID
	RANUENGAPID            *RANUENGAPID
	Cause                  *Cause
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var uEContextModificationFailureIEs = []ieSpec[UEContextModificationFailure]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextModificationFailure, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		Encode: func(m *UEContextModificationFailure) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextModificationFailure, r *per.Reader, enc per.Encoding) error {
			var v RANUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		Encode: func(m *UEContextModificationFailure) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
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
