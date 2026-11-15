// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// TS 38.413 §9.2.3.8. The target NG-RAN node asks the AMF to switch the
// downlink user-plane path after an Xn handover. TS 36.413 §9.1.5.8 splits the
// cell and tracking area into separate EUTRAN-CGI and TAI IEs where NGAP
// carries one UserLocationInformation, and orders the session list second where
// NGAP orders it fifth.
type PathSwitchRequest struct {
	RANUENGAPID                          RANUENGAPID
	SourceAMFUENGAPID                    AMFUENGAPID
	UserLocationInformation              *UserLocationInformation
	UESecurityCapabilities               *UESecurityCapabilities
	PDUSessionResourceToBeSwitchedDLList PDUSessionResourceToBeSwitchedDLList
	PDUSessionResourceFailedToSetup      PDUSessionResourceFailedToSetupListPSReq
	SourceAMFGUAMI                       *GUAMI
	RRCResumeCause                       *RRCEstablishmentCause
	RedCapIndication                     *RedCapIndication
	ERedCapIndication                    *ERedCapIndication

	messageMeta
}

var pathSwitchRequestIEs = []ieSpec[PathSwitchRequest]{
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PathSwitchRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *PathSwitchRequest) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		ID: IDSourceAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PathSwitchRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SourceAMFUENGAPID)
		},
		Encode: func(m *PathSwitchRequest) (per.Marshaler, bool) { return &m.SourceAMFUENGAPID, true },
	},
	{
		ID: IDUserLocationInformation, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequest, r *per.Reader, enc per.Encoding) error {
			var v UserLocationInformation

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.UserLocationInformation = &v

			return nil
		},
		Encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
	{
		ID: IDUESecurityCapabilities, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequest, r *per.Reader, enc per.Encoding) error {
			var v UESecurityCapabilities

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.UESecurityCapabilities = &v

			return nil
		},
		Encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			if m.UESecurityCapabilities == nil {
				return nil, false
			}

			return m.UESecurityCapabilities, true
		},
	},
	{
		ID: IDPDUSessionResourceToBeSwitchedDLList, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PathSwitchRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceToBeSwitchedDLList)
		},
		Encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			if m.PDUSessionResourceToBeSwitchedDLList == nil {
				return nil, false
			}

			return m.PDUSessionResourceToBeSwitchedDLList, true
		},
	},
	{
		ID: IDPDUSessionResourceFailedToSetupListPSReq, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceFailedToSetup)
		},
		Encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			if m.PDUSessionResourceFailedToSetup == nil {
				return nil, false
			}

			return m.PDUSessionResourceFailedToSetup, true
		},
	},
	apimpl.OptionalIE(IDRRCResumeCause, CriticalityIgnore, func(m *PathSwitchRequest) **RRCEstablishmentCause { return &m.RRCResumeCause }),
	apimpl.OptionalIE(IDRedCapIndication, CriticalityIgnore, func(m *PathSwitchRequest) **RedCapIndication { return &m.RedCapIndication }),
	apimpl.OptionalIE(IDERedCapIndication, CriticalityIgnore, func(m *PathSwitchRequest) **ERedCapIndication { return &m.ERedCapIndication }),
	{
		ID: IDSourceAMFGUAMI, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequest, r *per.Reader, enc per.Encoding) error {
			var v GUAMI

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.SourceAMFGUAMI = &v

			return nil
		},
		Encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			if m.SourceAMFGUAMI == nil {
				return nil, false
			}

			return m.SourceAMFGUAMI, true
		},
	},
}

func (m *PathSwitchRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPathSwitchRequest, pathSwitchRequestIEs, m)
}

func (m *PathSwitchRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcPathSwitchRequest,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePathSwitchRequest(value []byte) (*PathSwitchRequest, error) {
	return parseMessageBody[PathSwitchRequest](ProcPathSwitchRequest, TriggeringInitiatingMessage, pathSwitchRequestIEs, value)
}

func DecodePathSwitchRequest(value []byte) (*PathSwitchRequest, error) {
	return decodeMessageBody[PathSwitchRequest](ProcPathSwitchRequest, pathSwitchRequestIEs, value)
}

// TS 38.413 §9.2.3.9. The AMF confirms the switch and hands the target the
// {NH, NCC} pair for its next key derivation. TS 36.413 §9.1.5.9 carries a
// UEAggregateMaximumBitRate NGAP does not, and NGAP carries an AllowedNSSAI
// S1AP does not.
type PathSwitchRequestAcknowledge struct {
	AMFUENGAPID                    *AMFUENGAPID
	RANUENGAPID                    *RANUENGAPID
	UESecurityCapabilities         *UESecurityCapabilities
	SecurityContext                SecurityContext
	PDUSessionResourceSwitchedList PDUSessionResourceSwitchedList
	// Sessions the AMF could not switch, whose resources the NG-RAN node
	// releases (§8.4.4.2).
	PDUSessionResourceReleased                  PDUSessionResourceReleasedListPSAck
	AllowedNSSAI                                AllowedNSSAI
	CriticalityDiagnostics                      *CriticalityDiagnostics
	AMFUENGAPID2                                *AMFUENGAPID
	NewSecurityContextInd                       *NewSecurityContextInd
	CoreNetworkAssistanceInformationForInactive *CoreNetworkAssistanceInformationForInactive
	RRCInactiveTransitionReportRequest          *RRCInactiveTransitionReportRequest
	RedirectionVoiceFallback                    *RedirectionVoiceFallback
	CNAssistedRANTuning                         *CNAssistedRANTuning
	SRVCCOperationPossible                      *SRVCCOperationPossible
	EnhancedCoverageRestriction                 *EnhancedCoverageRestriction
	ExtendedConnectedTime                       *ExtendedConnectedTime
	UEDifferentiationInfo                       *UEDifferentiationInfo
	NRV2XServicesAuthorized                     *NRV2XServicesAuthorized
	LTEV2XServicesAuthorized                    *LTEV2XServicesAuthorized
	NRUESidelinkAggregateMaximumBitrate         *NRUESidelinkAggregateMaximumBitrate
	LTEUESidelinkAggregateMaximumBitrate        *LTEUESidelinkAggregateMaximumBitrate
	PC5QoSParameters                            *PC5QoSParameters
	CEmodeBrestricted                           *CEmodeBrestricted
	UEUPCIoTSupport                             *UEUPCIoTSupport
	UERadioCapabilityID                         *UERadioCapabilityID
	ManagementBasedMDTPLMNList                  MDTPLMNList
	TimeSyncAssistanceInfo                      *TimeSyncAssistanceInfo
	FiveGProSeAuthorized                        *FiveGProSeAuthorized
	FiveGProSeUEPC5AggregateMaximumBitRate      *NRUESidelinkAggregateMaximumBitrate
	FiveGProSePC5QoSParameters                  *FiveGProSePC5QoSParameters
	ManagementBasedMDTPLMNModificationList      MDTPLMNModificationList
	IABAuthorized                               *IABAuthorized
	AerialUEsubscriptionInformation             *AerialUEsubscriptionInformation
	NRA2XServicesAuthorized                     *NRA2XServicesAuthorized
	LTEA2XServicesAuthorized                    *LTEA2XServicesAuthorized
	NRA2XUEPC5AggregateMaximumBitRate           *NRUESidelinkAggregateMaximumBitrate
	LTEA2XUEPC5AggregateMaximumBitRate          *LTEUESidelinkAggregateMaximumBitrate
	A2XPC5QoSParameters                         *A2XPC5QoSParameters
	MobileIABAuthorized                         *MobileIABAuthorized
	PartiallyAllowedNSSAI                       PartiallyAllowedNSSAI
	SLPositioningRangingServiceInfo             *SLPositioningRangingServiceInfo

	messageMeta
}

var pathSwitchRequestAcknowledgeIEs = []ieSpec[PathSwitchRequestAcknowledge]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		Encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			var v RANUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		Encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		ID: IDUESecurityCapabilities, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *PathSwitchRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			var v UESecurityCapabilities

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.UESecurityCapabilities = &v

			return nil
		},
		Encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.UESecurityCapabilities == nil {
				return nil, false
			}

			return m.UESecurityCapabilities, true
		},
	},
	{
		ID: IDSecurityContext, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PathSwitchRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SecurityContext)
		},
		Encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) { return &m.SecurityContext, true },
	},
	apimpl.OptionalIE(IDNewSecurityContextInd, CriticalityReject, func(m *PathSwitchRequestAcknowledge) **NewSecurityContextInd { return &m.NewSecurityContextInd }),
	{
		ID: IDPDUSessionResourceSwitchedList, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceSwitchedList)
		},
		Encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.PDUSessionResourceSwitchedList == nil {
				return nil, false
			}

			return m.PDUSessionResourceSwitchedList, true
		},
	},
	{
		ID: IDPDUSessionResourceReleasedListPSAck, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceReleased)
		},
		Encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.PDUSessionResourceReleased == nil {
				return nil, false
			}

			return m.PDUSessionResourceReleased, true
		},
	},
	{
		ID: IDAllowedNSSAI, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PathSwitchRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AllowedNSSAI)
		},
		Encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.AllowedNSSAI == nil {
				return nil, false
			}

			return m.AllowedNSSAI, true
		},
	},
	apimpl.OptionalIE(IDCoreNetworkAssistanceInformationForInactive, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **CoreNetworkAssistanceInformationForInactive {
		return &m.CoreNetworkAssistanceInformationForInactive
	}),
	apimpl.OptionalIE(IDRRCInactiveTransitionReportRequest, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **RRCInactiveTransitionReportRequest {
		return &m.RRCInactiveTransitionReportRequest
	}),
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		Encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	apimpl.OptionalIE(IDRedirectionVoiceFallback, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **RedirectionVoiceFallback { return &m.RedirectionVoiceFallback }),
	apimpl.OptionalIE(IDCNAssistedRANTuning, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **CNAssistedRANTuning { return &m.CNAssistedRANTuning }),
	apimpl.OptionalIE(IDSRVCCOperationPossible, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **SRVCCOperationPossible { return &m.SRVCCOperationPossible }),
	apimpl.OptionalIE(IDEnhancedCoverageRestriction, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **EnhancedCoverageRestriction {
		return &m.EnhancedCoverageRestriction
	}),
	apimpl.OptionalIE(IDExtendedConnectedTime, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **ExtendedConnectedTime { return &m.ExtendedConnectedTime }),
	apimpl.OptionalIE(IDUEDifferentiationInfo, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **UEDifferentiationInfo { return &m.UEDifferentiationInfo }),
	apimpl.OptionalIE(IDNRV2XServicesAuthorized, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **NRV2XServicesAuthorized { return &m.NRV2XServicesAuthorized }),
	apimpl.OptionalIE(IDLTEV2XServicesAuthorized, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **LTEV2XServicesAuthorized { return &m.LTEV2XServicesAuthorized }),
	apimpl.OptionalIE(IDNRUESidelinkAggregateMaximumBitrate, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **NRUESidelinkAggregateMaximumBitrate {
		return &m.NRUESidelinkAggregateMaximumBitrate
	}),
	apimpl.OptionalIE(IDLTEUESidelinkAggregateMaximumBitrate, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **LTEUESidelinkAggregateMaximumBitrate {
		return &m.LTEUESidelinkAggregateMaximumBitrate
	}),
	apimpl.OptionalIE(IDPC5QoSParameters, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **PC5QoSParameters { return &m.PC5QoSParameters }),
	apimpl.OptionalIE(IDCEmodeBrestricted, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **CEmodeBrestricted { return &m.CEmodeBrestricted }),
	apimpl.OptionalIE(IDUEUPCIoTSupport, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **UEUPCIoTSupport { return &m.UEUPCIoTSupport }),
	apimpl.OptionalIE(IDUERadioCapabilityID, CriticalityReject, func(m *PathSwitchRequestAcknowledge) **UERadioCapabilityID { return &m.UERadioCapabilityID }),
	apimpl.ListIE(IDManagementBasedMDTPLMNList, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) *MDTPLMNList { return &m.ManagementBasedMDTPLMNList }),
	apimpl.OptionalIE(IDTimeSyncAssistanceInfo, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **TimeSyncAssistanceInfo { return &m.TimeSyncAssistanceInfo }),
	apimpl.OptionalIE(IDFiveGProSeAuthorized, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **FiveGProSeAuthorized { return &m.FiveGProSeAuthorized }),
	apimpl.OptionalIE(IDFiveGProSeUEPC5AggregateMaximumBitRate, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **NRUESidelinkAggregateMaximumBitrate {
		return &m.FiveGProSeUEPC5AggregateMaximumBitRate
	}),
	apimpl.OptionalIE(IDFiveGProSePC5QoSParameters, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **FiveGProSePC5QoSParameters {
		return &m.FiveGProSePC5QoSParameters
	}),
	apimpl.ListIE(IDManagementBasedMDTPLMNModificationList, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) *MDTPLMNModificationList {
		return &m.ManagementBasedMDTPLMNModificationList
	}),
	apimpl.OptionalIE(IDIABAuthorized, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **IABAuthorized { return &m.IABAuthorized }),
	apimpl.OptionalIE(IDAerialUEsubscriptionInformation, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **AerialUEsubscriptionInformation {
		return &m.AerialUEsubscriptionInformation
	}),
	apimpl.OptionalIE(IDNRA2XServicesAuthorized, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **NRA2XServicesAuthorized { return &m.NRA2XServicesAuthorized }),
	apimpl.OptionalIE(IDLTEA2XServicesAuthorized, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **LTEA2XServicesAuthorized { return &m.LTEA2XServicesAuthorized }),
	apimpl.OptionalIE(IDNRA2XUEPC5AggregateMaximumBitRate, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **NRUESidelinkAggregateMaximumBitrate {
		return &m.NRA2XUEPC5AggregateMaximumBitRate
	}),
	apimpl.OptionalIE(IDLTEA2XUEPC5AggregateMaximumBitRate, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **LTEUESidelinkAggregateMaximumBitrate {
		return &m.LTEA2XUEPC5AggregateMaximumBitRate
	}),
	apimpl.OptionalIE(IDA2XPC5QoSParameters, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **A2XPC5QoSParameters { return &m.A2XPC5QoSParameters }),
	apimpl.OptionalIE(IDMobileIABAuthorized, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **MobileIABAuthorized { return &m.MobileIABAuthorized }),
	apimpl.ListIE(IDPartiallyAllowedNSSAI, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) *PartiallyAllowedNSSAI { return &m.PartiallyAllowedNSSAI }),
	apimpl.OptionalIE(IDSLPositioningRangingServiceInfo, CriticalityIgnore, func(m *PathSwitchRequestAcknowledge) **SLPositioningRangingServiceInfo {
		return &m.SLPositioningRangingServiceInfo
	}),
	{
		ID: IDAMFUENGAPID2, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFUENGAPID2 = &v

			return nil
		},
		Encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.AMFUENGAPID2 == nil {
				return nil, false
			}

			return m.AMFUENGAPID2, true
		},
	},
}

func (m *PathSwitchRequestAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPathSwitchRequest, pathSwitchRequestAcknowledgeIEs, m)
}

func (m *PathSwitchRequestAcknowledge) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcPathSwitchRequest,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePathSwitchRequestAcknowledge(value []byte) (*PathSwitchRequestAcknowledge, error) {
	return parseMessageBody[PathSwitchRequestAcknowledge](ProcPathSwitchRequest, TriggeringSuccessfulOutcome, pathSwitchRequestAcknowledgeIEs, value)
}

func DecodePathSwitchRequestAcknowledge(value []byte) (*PathSwitchRequestAcknowledge, error) {
	return decodeMessageBody[PathSwitchRequestAcknowledge](ProcPathSwitchRequest, pathSwitchRequestAcknowledgeIEs, value)
}

// TS 38.413 §9.2.3.10. The AMF refuses the switch. Where TS 36.413 §9.1.5.10
// names one Cause for the whole message, NGAP reports per session: every
// session in the request is released, each with its own cause inside a
// PathSwitchRequestUnsuccessfulTransfer.
type PathSwitchRequestFailure struct {
	AMFUENGAPID                *AMFUENGAPID
	RANUENGAPID                *RANUENGAPID
	PDUSessionResourceReleased PDUSessionResourceReleasedListPSFail
	CriticalityDiagnostics     *CriticalityDiagnostics

	messageMeta
}

var pathSwitchRequestFailureIEs = []ieSpec[PathSwitchRequestFailure]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestFailure, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		Encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestFailure, r *per.Reader, enc per.Encoding) error {
			var v RANUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		Encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		ID: IDPDUSessionResourceReleasedListPSFail, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestFailure, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceReleased)
		},
		Encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) {
			if m.PDUSessionResourceReleased == nil {
				return nil, false
			}

			return m.PDUSessionResourceReleased, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PathSwitchRequestFailure, r *per.Reader, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		Encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *PathSwitchRequestFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPathSwitchRequest, pathSwitchRequestFailureIEs, m)
}

func (m *PathSwitchRequestFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcPathSwitchRequest,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePathSwitchRequestFailure(value []byte) (*PathSwitchRequestFailure, error) {
	return parseMessageBody[PathSwitchRequestFailure](ProcPathSwitchRequest, TriggeringUnsuccessfulOutcome, pathSwitchRequestFailureIEs, value)
}

func DecodePathSwitchRequestFailure(value []byte) (*PathSwitchRequestFailure, error) {
	return decodeMessageBody[PathSwitchRequestFailure](ProcPathSwitchRequest, pathSwitchRequestFailureIEs, value)
}
