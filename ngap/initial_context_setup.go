// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// PDUSessionResourceSetupItemCxtReq ::= SEQUENCE { pDUSessionID, nAS-PDU
// OPTIONAL, s-NSSAI, pDUSessionResourceSetupRequestTransfer, iE-Extensions
// OPTIONAL } (extensible).
//
// The transfer is OCTET STRING (CONTAINING PDUSessionResourceSetupRequestTransfer)
// (§9.3.4.1). The AMF neither builds nor reads it — the SMF does — so it is
// carried opaquely.
type PDUSessionResourceSetupItemCxtReq struct {
	PDUSessionID                          PDUSessionID
	NASPDU                                *NASPDU `per:",optional"`
	SNSSAI                                SNSSAI
	Transfer                              TransferContainer
	PduSessionExpectedUEActivityBehaviour *ExpectedUEActivityBehaviour `per:",extie:IDPduSessionExpectedUEActivityBehaviour,crit:ignore"`
	iEExtensions                          extensionMeta                `per:",exttable"`
	extensions                            per.ExtensionAdditions       `per:",extraw"`
}

// PDUSessionResourceSetupListCxtReq ::= SEQUENCE (SIZE(1..maxnoofPDUSessions))
// OF PDUSessionResourceSetupItemCxtReq.
type PDUSessionResourceSetupListCxtReq []PDUSessionResourceSetupItemCxtReq

// PDUSessionResourceSetupItemCxtRes ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceSetupResponseTransfer, iE-Extensions OPTIONAL }
// (extensible). The transfer contains a PDUSessionResourceSetupResponseTransfer
// (§9.3.4.2).
type PDUSessionResourceSetupItemCxtRes struct {
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

// PDUSessionResourceSetupListCxtRes ::= SEQUENCE (SIZE(1..maxnoofPDUSessions))
// OF PDUSessionResourceSetupItemCxtRes.
type PDUSessionResourceSetupListCxtRes []PDUSessionResourceSetupItemCxtRes

// PDUSessionResourceFailedToSetupItemCxtRes ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceSetupUnsuccessfulTransfer, iE-Extensions OPTIONAL }
// (extensible). The transfer contains a
// PDUSessionResourceSetupUnsuccessfulTransfer (§9.3.4.16).
type PDUSessionResourceFailedToSetupItemCxtRes struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta `per:",exttable"`
}

// PDUSessionResourceFailedToSetupListCxtRes ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceFailedToSetupItemCxtRes.
type PDUSessionResourceFailedToSetupListCxtRes []PDUSessionResourceFailedToSetupItemCxtRes

// PDUSessionResourceFailedToSetupItemCxtFail has the same shape as its CxtRes
// counterpart but its own ASN.1 type, so the two lists stay distinct.
type PDUSessionResourceFailedToSetupItemCxtFail struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta `per:",exttable"`
}

// PDUSessionResourceFailedToSetupListCxtFail ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceFailedToSetupItemCxtFail.
type PDUSessionResourceFailedToSetupListCxtFail []PDUSessionResourceFailedToSetupItemCxtFail

// TS 38.413 §9.2.2.1.
type InitialContextSetupRequest struct {
	AMFUENGAPID AMFUENGAPID
	RANUENGAPID RANUENGAPID
	// C-ifPDUSessionResourceSetup: present exactly when PDUSessionResourceSetup
	// is (§9.2.2.1).
	UEAggregateMaximumBitRate                   *UEAggregateMaximumBitRate
	GUAMI                                       GUAMI
	PDUSessionResourceSetup                     PDUSessionResourceSetupListCxtReq
	AllowedNSSAI                                AllowedNSSAI
	UESecurityCapabilities                      UESecurityCapabilities
	SecurityKey                                 SecurityKey
	TraceActivation                             *TraceActivation
	UERadioCapability                           UERadioCapability
	NASPDU                                      *NASPDU
	UERadioCapabilityForPaging                  *UERadioCapabilityForPaging
	AMFUENGAPID2                                *AMFUENGAPID
	OldAMF                                      *Name
	CoreNetworkAssistanceInformationForInactive *CoreNetworkAssistanceInformationForInactive
	MobilityRestrictionList                     *MobilityRestrictionList
	IndexToRFSP                                 *IndexToRFSP
	MaskedIMEISV                                *MaskedIMEISV
	EmergencyFallbackIndicator                  *EmergencyFallbackIndicator
	RRCInactiveTransitionReportRequest          *RRCInactiveTransitionReportRequest
	RedirectionVoiceFallback                    *RedirectionVoiceFallback
	LocationReportingRequestType                *LocationReportingRequestType
	CNAssistedRANTuning                         *CNAssistedRANTuning
	SRVCCOperationPossible                      *SRVCCOperationPossible
	IABAuthorized                               *IABAuthorized
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
	RGLevelWirelineAccessCharacteristics        *RGLevelWirelineAccessCharacteristics
	ManagementBasedMDTPLMNList                  MDTPLMNList
	UERadioCapabilityID                         *UERadioCapabilityID
	TimeSyncAssistanceInfo                      *TimeSyncAssistanceInfo
	QMCConfigInfo                               *QMCConfigInfo
	TargetNSSAIInformation                      *TargetNSSAIInformation
	UESliceMaximumBitRateList                   UESliceMaximumBitRateList
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
	PartiallyAllowedNSSAI                       PartiallyAllowedNSSAI
	SLPositioningRangingServiceInfo             *SLPositioningRangingServiceInfo
	ExtendedOldAMF                              *ExtendedAMFName

	messageMeta
}

var initialContextSetupRequestIEs = []ieSpec[InitialContextSetupRequest]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFUENGAPID)
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	apimpl.OptionalIE(IDOldAMF, CriticalityReject, func(m *InitialContextSetupRequest) **Name { return &m.OldAMF }),
	{
		ID: IDUEAggregateMaximumBitRate, Presence: presenceConditional, Criticality: CriticalityReject,
		Condition: func(m *InitialContextSetupRequest) bool { return m.PDUSessionResourceSetup != nil },
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			var v UEAggregateMaximumBitRate

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.UEAggregateMaximumBitRate = &v

			return nil
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			if m.UEAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.UEAggregateMaximumBitRate, true
		},
	},
	apimpl.OptionalIE(IDCoreNetworkAssistanceInformationForInactive, CriticalityIgnore, func(m *InitialContextSetupRequest) **CoreNetworkAssistanceInformationForInactive {
		return &m.CoreNetworkAssistanceInformationForInactive
	}),
	{
		ID: IDGUAMI, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.GUAMI)
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return &m.GUAMI, true },
	},
	{
		ID: IDPDUSessionResourceSetupListCxtReq, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceSetup)
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			if m.PDUSessionResourceSetup == nil {
				return nil, false
			}

			return m.PDUSessionResourceSetup, true
		},
	},
	{
		ID: IDAllowedNSSAI, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AllowedNSSAI)
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			if m.AllowedNSSAI == nil {
				return nil, false
			}

			return m.AllowedNSSAI, true
		},
	},
	{
		ID: IDUESecurityCapabilities, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.UESecurityCapabilities)
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			return &m.UESecurityCapabilities, true
		},
	},
	{
		ID: IDSecurityKey, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SecurityKey)
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return &m.SecurityKey, true },
	},
	{
		ID: IDTraceActivation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			var v TraceActivation

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.TraceActivation = &v

			return nil
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			if m.TraceActivation == nil {
				return nil, false
			}

			return m.TraceActivation, true
		},
	},
	apimpl.OptionalIE(IDMobilityRestrictionList, CriticalityIgnore, func(m *InitialContextSetupRequest) **MobilityRestrictionList { return &m.MobilityRestrictionList }),
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
	apimpl.OptionalIE(IDIndexToRFSP, CriticalityIgnore, func(m *InitialContextSetupRequest) **IndexToRFSP { return &m.IndexToRFSP }),
	apimpl.OptionalIE(IDMaskedIMEISV, CriticalityIgnore, func(m *InitialContextSetupRequest) **MaskedIMEISV { return &m.MaskedIMEISV }),
	{
		ID: IDNASPDU, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			var v NASPDU

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.NASPDU = &v

			return nil
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			if m.NASPDU == nil {
				return nil, false
			}

			return m.NASPDU, true
		},
	},
	apimpl.OptionalIE(IDEmergencyFallbackIndicator, CriticalityReject, func(m *InitialContextSetupRequest) **EmergencyFallbackIndicator { return &m.EmergencyFallbackIndicator }),
	apimpl.OptionalIE(IDRRCInactiveTransitionReportRequest, CriticalityIgnore, func(m *InitialContextSetupRequest) **RRCInactiveTransitionReportRequest {
		return &m.RRCInactiveTransitionReportRequest
	}),
	{
		ID: IDUERadioCapabilityForPaging, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			var v UERadioCapabilityForPaging

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.UERadioCapabilityForPaging = &v

			return nil
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			if m.UERadioCapabilityForPaging == nil {
				return nil, false
			}

			return m.UERadioCapabilityForPaging, true
		},
	},
	apimpl.OptionalIE(IDRedirectionVoiceFallback, CriticalityIgnore, func(m *InitialContextSetupRequest) **RedirectionVoiceFallback { return &m.RedirectionVoiceFallback }),
	apimpl.OptionalIE(IDLocationReportingRequestType, CriticalityIgnore, func(m *InitialContextSetupRequest) **LocationReportingRequestType {
		return &m.LocationReportingRequestType
	}),
	apimpl.OptionalIE(IDCNAssistedRANTuning, CriticalityIgnore, func(m *InitialContextSetupRequest) **CNAssistedRANTuning { return &m.CNAssistedRANTuning }),
	apimpl.OptionalIE(IDSRVCCOperationPossible, CriticalityIgnore, func(m *InitialContextSetupRequest) **SRVCCOperationPossible { return &m.SRVCCOperationPossible }),
	apimpl.OptionalIE(IDIABAuthorized, CriticalityIgnore, func(m *InitialContextSetupRequest) **IABAuthorized { return &m.IABAuthorized }),
	apimpl.OptionalIE(IDEnhancedCoverageRestriction, CriticalityIgnore, func(m *InitialContextSetupRequest) **EnhancedCoverageRestriction {
		return &m.EnhancedCoverageRestriction
	}),
	apimpl.OptionalIE(IDExtendedConnectedTime, CriticalityIgnore, func(m *InitialContextSetupRequest) **ExtendedConnectedTime { return &m.ExtendedConnectedTime }),
	apimpl.OptionalIE(IDUEDifferentiationInfo, CriticalityIgnore, func(m *InitialContextSetupRequest) **UEDifferentiationInfo { return &m.UEDifferentiationInfo }),
	apimpl.OptionalIE(IDNRV2XServicesAuthorized, CriticalityIgnore, func(m *InitialContextSetupRequest) **NRV2XServicesAuthorized { return &m.NRV2XServicesAuthorized }),
	apimpl.OptionalIE(IDLTEV2XServicesAuthorized, CriticalityIgnore, func(m *InitialContextSetupRequest) **LTEV2XServicesAuthorized { return &m.LTEV2XServicesAuthorized }),
	apimpl.OptionalIE(IDNRUESidelinkAggregateMaximumBitrate, CriticalityIgnore, func(m *InitialContextSetupRequest) **NRUESidelinkAggregateMaximumBitrate {
		return &m.NRUESidelinkAggregateMaximumBitrate
	}),
	apimpl.OptionalIE(IDLTEUESidelinkAggregateMaximumBitrate, CriticalityIgnore, func(m *InitialContextSetupRequest) **LTEUESidelinkAggregateMaximumBitrate {
		return &m.LTEUESidelinkAggregateMaximumBitrate
	}),
	apimpl.OptionalIE(IDPC5QoSParameters, CriticalityIgnore, func(m *InitialContextSetupRequest) **PC5QoSParameters { return &m.PC5QoSParameters }),
	apimpl.OptionalIE(IDCEmodeBrestricted, CriticalityIgnore, func(m *InitialContextSetupRequest) **CEmodeBrestricted { return &m.CEmodeBrestricted }),
	apimpl.OptionalIE(IDUEUPCIoTSupport, CriticalityIgnore, func(m *InitialContextSetupRequest) **UEUPCIoTSupport { return &m.UEUPCIoTSupport }),
	apimpl.OptionalIE(IDRGLevelWirelineAccessCharacteristics, CriticalityIgnore, func(m *InitialContextSetupRequest) **RGLevelWirelineAccessCharacteristics {
		return &m.RGLevelWirelineAccessCharacteristics
	}),
	apimpl.ListIE(IDManagementBasedMDTPLMNList, CriticalityIgnore, func(m *InitialContextSetupRequest) *MDTPLMNList { return &m.ManagementBasedMDTPLMNList }),
	apimpl.OptionalIE(IDUERadioCapabilityID, CriticalityReject, func(m *InitialContextSetupRequest) **UERadioCapabilityID { return &m.UERadioCapabilityID }),
	apimpl.OptionalIE(IDTimeSyncAssistanceInfo, CriticalityIgnore, func(m *InitialContextSetupRequest) **TimeSyncAssistanceInfo { return &m.TimeSyncAssistanceInfo }),
	apimpl.OptionalIE(IDQMCConfigInfo, CriticalityIgnore, func(m *InitialContextSetupRequest) **QMCConfigInfo { return &m.QMCConfigInfo }),
	apimpl.OptionalIE(IDTargetNSSAIInformation, CriticalityIgnore, func(m *InitialContextSetupRequest) **TargetNSSAIInformation { return &m.TargetNSSAIInformation }),
	apimpl.ListIE(IDUESliceMaximumBitRateList, CriticalityIgnore, func(m *InitialContextSetupRequest) *UESliceMaximumBitRateList { return &m.UESliceMaximumBitRateList }),
	apimpl.OptionalIE(IDFiveGProSeAuthorized, CriticalityIgnore, func(m *InitialContextSetupRequest) **FiveGProSeAuthorized { return &m.FiveGProSeAuthorized }),
	apimpl.OptionalIE(IDFiveGProSeUEPC5AggregateMaximumBitRate, CriticalityIgnore, func(m *InitialContextSetupRequest) **NRUESidelinkAggregateMaximumBitrate {
		return &m.FiveGProSeUEPC5AggregateMaximumBitRate
	}),
	apimpl.OptionalIE(IDFiveGProSePC5QoSParameters, CriticalityIgnore, func(m *InitialContextSetupRequest) **FiveGProSePC5QoSParameters { return &m.FiveGProSePC5QoSParameters }),
	apimpl.OptionalIE(IDNetworkControlledRepeaterAuthorized, CriticalityIgnore, func(m *InitialContextSetupRequest) **NetworkControlledRepeaterAuthorized {
		return &m.NetworkControlledRepeaterAuthorized
	}),
	apimpl.OptionalIE(IDAerialUEsubscriptionInformation, CriticalityIgnore, func(m *InitialContextSetupRequest) **AerialUEsubscriptionInformation {
		return &m.AerialUEsubscriptionInformation
	}),
	apimpl.OptionalIE(IDNRA2XServicesAuthorized, CriticalityIgnore, func(m *InitialContextSetupRequest) **NRA2XServicesAuthorized { return &m.NRA2XServicesAuthorized }),
	apimpl.OptionalIE(IDLTEA2XServicesAuthorized, CriticalityIgnore, func(m *InitialContextSetupRequest) **LTEA2XServicesAuthorized { return &m.LTEA2XServicesAuthorized }),
	apimpl.OptionalIE(IDNRA2XUEPC5AggregateMaximumBitRate, CriticalityIgnore, func(m *InitialContextSetupRequest) **NRUESidelinkAggregateMaximumBitrate {
		return &m.NRA2XUEPC5AggregateMaximumBitRate
	}),
	apimpl.OptionalIE(IDLTEA2XUEPC5AggregateMaximumBitRate, CriticalityIgnore, func(m *InitialContextSetupRequest) **LTEUESidelinkAggregateMaximumBitrate {
		return &m.LTEA2XUEPC5AggregateMaximumBitRate
	}),
	apimpl.OptionalIE(IDA2XPC5QoSParameters, CriticalityIgnore, func(m *InitialContextSetupRequest) **A2XPC5QoSParameters { return &m.A2XPC5QoSParameters }),
	apimpl.OptionalIE(IDMobileIABAuthorized, CriticalityIgnore, func(m *InitialContextSetupRequest) **MobileIABAuthorized { return &m.MobileIABAuthorized }),
	apimpl.ListIE(IDPartiallyAllowedNSSAI, CriticalityIgnore, func(m *InitialContextSetupRequest) *PartiallyAllowedNSSAI { return &m.PartiallyAllowedNSSAI }),
	apimpl.OptionalIE(IDSLPositioningRangingServiceInfo, CriticalityIgnore, func(m *InitialContextSetupRequest) **SLPositioningRangingServiceInfo {
		return &m.SLPositioningRangingServiceInfo
	}),
	apimpl.OptionalIE(IDExtendedOldAMF, CriticalityIgnore, func(m *InitialContextSetupRequest) **ExtendedAMFName { return &m.ExtendedOldAMF }),
	{
		ID: IDAMFUENGAPID2, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupRequest, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFUENGAPID2 = &v

			return nil
		},
		Encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			if m.AMFUENGAPID2 == nil {
				return nil, false
			}

			return m.AMFUENGAPID2, true
		},
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

// TS 38.413 §9.2.2.2.
type InitialContextSetupResponse struct {
	AMFUENGAPID              *AMFUENGAPID
	RANUENGAPID              *RANUENGAPID
	PDUSessionResourceSetup  PDUSessionResourceSetupListCxtRes
	PDUSessionResourceFailed PDUSessionResourceFailedToSetupListCxtRes
	CriticalityDiagnostics   *CriticalityDiagnostics

	messageMeta
}

var initialContextSetupResponseIEs = []ieSpec[InitialContextSetupResponse]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupResponse, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		Encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupResponse, r *per.Reader, enc per.Encoding) error {
			var v RANUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		Encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		ID: IDPDUSessionResourceSetupListCxtRes, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupResponse, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceSetup)
		},
		Encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) {
			if m.PDUSessionResourceSetup == nil {
				return nil, false
			}

			return m.PDUSessionResourceSetup, true
		},
	},
	{
		ID: IDPDUSessionResourceFailedToSetupListCxtRes, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupResponse, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceFailed)
		},
		Encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) {
			if m.PDUSessionResourceFailed == nil {
				return nil, false
			}

			return m.PDUSessionResourceFailed, true
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

// TS 38.413 §9.2.2.3.
type InitialContextSetupFailure struct {
	AMFUENGAPID              *AMFUENGAPID
	RANUENGAPID              *RANUENGAPID
	PDUSessionResourceFailed PDUSessionResourceFailedToSetupListCxtFail
	Cause                    *Cause
	CriticalityDiagnostics   *CriticalityDiagnostics

	messageMeta
}

var initialContextSetupFailureIEs = []ieSpec[InitialContextSetupFailure]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupFailure, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		Encode: func(m *InitialContextSetupFailure) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupFailure, r *per.Reader, enc per.Encoding) error {
			var v RANUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		Encode: func(m *InitialContextSetupFailure) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		ID: IDPDUSessionResourceFailedToSetupListCxtFail, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialContextSetupFailure, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceFailed)
		},
		Encode: func(m *InitialContextSetupFailure) (per.Marshaler, bool) {
			if m.PDUSessionResourceFailed == nil {
				return nil, false
			}

			return m.PDUSessionResourceFailed, true
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
