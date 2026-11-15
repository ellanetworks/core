// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// TS 38.413 §9.2.5.1.
//
// S1AP carries the location as TAI (mandatory reject) plus E-UTRAN CGI
// (mandatory ignore); NGAP merges both into User Location Information, which is
// mandatory reject and therefore a value type here — where UPLINK NAS TRANSPORT
// marks the same IE ignore and it is nil-able.
type InitialUEMessage struct {
	RANUENGAPID             RANUENGAPID
	NASPDU                  NASPDU
	UserLocationInformation UserLocationInformation
	RRCEstablishmentCause   *RRCEstablishmentCause
	// §8.6.1.2: the NG-RAN node includes the 5G-S-TMSI whenever the UE supplied
	// it over the radio interface, and the AMF Set ID only on a message it has
	// rerouted, which the AMF acts on per TS 23.502. The AMF Set ID is not the
	// counterpart of S1AP's GUMMEI, which marks an eNB that does not run NNSF.
	FiveGSTMSI                          *FiveGSTMSI
	AMFSetID                            *AMFSetID
	UEContextRequest                    *UEContextRequest
	AllowedNSSAI                        AllowedNSSAI
	GUAMI                               *GUAMI
	GUAMIType                           *GUAMIType
	RequestedNSSAI                      RequestedNSSAI
	SourceToTargetAMFInformationReroute *SourceToTargetAMFInformationReroute
	SelectedPLMNIdentity                *PLMNIdentity
	IABNodeIndication                   *IABNodeIndication
	CEmodeBSupportIndicator             *CEmodeBSupportIndicator
	LTEMIndication                      *LTEMIndication
	EDTSession                          *EDTSession
	AuthenticatedIndication             *AuthenticatedIndication
	NPNAccessInformation                *NPNAccessInformation
	RedCapIndication                    *RedCapIndication
	SelectedNID                         *NID
	MobileIABNodeIndication             *MobileIABNodeIndication
	PartiallyAllowedNSSAI               PartiallyAllowedNSSAI
	ERedCapIndication                   *ERedCapIndication
	AUN3DeviceAccessInfo                *AUN3DeviceAccessInfo

	messageMeta
}

var initialUEMessageIEs = []ieSpec[InitialUEMessage]{
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		ID: IDNASPDU, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.NASPDU)
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.NASPDU == nil {
				return nil, false
			}

			return &m.NASPDU, true
		},
	},
	{
		ID: IDUserLocationInformation, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.UserLocationInformation)
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) { return &m.UserLocationInformation, true },
	},
	{
		ID: IDRRCEstablishmentCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			var v RRCEstablishmentCause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.RRCEstablishmentCause = &v

			return nil
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.RRCEstablishmentCause == nil {
				return nil, false
			}

			return m.RRCEstablishmentCause, true
		},
	},
	{
		ID: IDFiveGSTMSI, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			var v FiveGSTMSI

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.FiveGSTMSI = &v

			return nil
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.FiveGSTMSI == nil {
				return nil, false
			}

			return m.FiveGSTMSI, true
		},
	},
	{
		ID: IDAMFSetID, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			var v AMFSetID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFSetID = &v

			return nil
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.AMFSetID == nil {
				return nil, false
			}

			return m.AMFSetID, true
		},
	},
	{
		ID: IDUEContextRequest, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			var v UEContextRequest

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.UEContextRequest = &v

			return nil
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.UEContextRequest == nil {
				return nil, false
			}

			return m.UEContextRequest, true
		},
	},
	{
		// Modeled but not acted on: the IE is reject, so not comprehending it
		// would reject an otherwise valid INITIAL UE MESSAGE (§10.3.4.2).
		ID: IDAllowedNSSAI, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AllowedNSSAI)
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.AllowedNSSAI == nil {
				return nil, false
			}

			return m.AllowedNSSAI, true
		},
	},
	apimpl.OptionalIE(IDSourceToTargetAMFInformationReroute, CriticalityIgnore, func(m *InitialUEMessage) **SourceToTargetAMFInformationReroute {
		return &m.SourceToTargetAMFInformationReroute
	}),
	apimpl.OptionalIE(IDSelectedPLMNIdentity, CriticalityIgnore, func(m *InitialUEMessage) **PLMNIdentity { return &m.SelectedPLMNIdentity }),
	apimpl.OptionalIE(IDIABNodeIndication, CriticalityReject, func(m *InitialUEMessage) **IABNodeIndication { return &m.IABNodeIndication }),
	apimpl.OptionalIE(IDCEmodeBSupportIndicator, CriticalityReject, func(m *InitialUEMessage) **CEmodeBSupportIndicator { return &m.CEmodeBSupportIndicator }),
	apimpl.OptionalIE(IDLTEMIndication, CriticalityIgnore, func(m *InitialUEMessage) **LTEMIndication { return &m.LTEMIndication }),
	apimpl.OptionalIE(IDEDTSession, CriticalityIgnore, func(m *InitialUEMessage) **EDTSession { return &m.EDTSession }),
	apimpl.OptionalIE(IDAuthenticatedIndication, CriticalityIgnore, func(m *InitialUEMessage) **AuthenticatedIndication { return &m.AuthenticatedIndication }),
	apimpl.OptionalIE(IDNPNAccessInformation, CriticalityReject, func(m *InitialUEMessage) **NPNAccessInformation { return &m.NPNAccessInformation }),
	apimpl.OptionalIE(IDRedCapIndication, CriticalityIgnore, func(m *InitialUEMessage) **RedCapIndication { return &m.RedCapIndication }),
	apimpl.OptionalIE(IDSelectedNID, CriticalityIgnore, func(m *InitialUEMessage) **NID { return &m.SelectedNID }),
	apimpl.OptionalIE(IDMobileIABNodeIndication, CriticalityReject, func(m *InitialUEMessage) **MobileIABNodeIndication { return &m.MobileIABNodeIndication }),
	apimpl.ListIE(IDPartiallyAllowedNSSAI, CriticalityIgnore, func(m *InitialUEMessage) *PartiallyAllowedNSSAI { return &m.PartiallyAllowedNSSAI }),
	apimpl.OptionalIE(IDERedCapIndication, CriticalityIgnore, func(m *InitialUEMessage) **ERedCapIndication { return &m.ERedCapIndication }),
	apimpl.OptionalIE(IDAUN3DeviceAccessInfo, CriticalityIgnore, func(m *InitialUEMessage) **AUN3DeviceAccessInfo { return &m.AUN3DeviceAccessInfo }),
	{
		ID: IDGUAMI, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			var v GUAMI

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.GUAMI = &v

			return nil
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.GUAMI == nil {
				return nil, false
			}

			return m.GUAMI, true
		},
	},
	{
		ID: IDGUAMIType, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			var v GUAMIType

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.GUAMIType = &v

			return nil
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.GUAMIType == nil {
				return nil, false
			}

			return m.GUAMIType, true
		},
	},
	{
		ID: IDRequestedNSSAI, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RequestedNSSAI)
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if len(m.RequestedNSSAI) == 0 {
				return nil, false
			}

			return m.RequestedNSSAI, true
		},
	},
}

func (m *InitialUEMessage) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcInitialUEMessage, initialUEMessageIEs, m)
}

func (m *InitialUEMessage) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcInitialUEMessage,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseInitialUEMessage(value []byte) (*InitialUEMessage, error) {
	return parseMessageBody[InitialUEMessage](ProcInitialUEMessage, TriggeringInitiatingMessage, initialUEMessageIEs, value)
}

func DecodeInitialUEMessage(value []byte) (*InitialUEMessage, error) {
	return decodeMessageBody[InitialUEMessage](ProcInitialUEMessage, initialUEMessageIEs, value)
}

// TS 38.413 §9.2.5.3.
//
// S1AP carries the location as two mandatory IEs, E-UTRAN CGI and TAI; NGAP
// carries the single User Location Information CHOICE (§9.3.1.16).
type UplinkNASTransport struct {
	AMFUENGAPID             AMFUENGAPID
	RANUENGAPID             RANUENGAPID
	NASPDU                  NASPDU
	UserLocationInformation *UserLocationInformation
	WAGFIdentityInformation *IdentityInformation
	TNGFIdentityInformation *IdentityInformation
	TWIFIdentityInformation *IdentityInformation

	messageMeta
}

type IdentityInformation []byte

func (v IdentityInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *IdentityInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	*v = b

	return err
}

var uplinkNASTransportIEs = []ieSpec[UplinkNASTransport]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UplinkNASTransport, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFUENGAPID)
		},
		Encode: func(m *UplinkNASTransport) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UplinkNASTransport, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *UplinkNASTransport) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		ID: IDNASPDU, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UplinkNASTransport, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.NASPDU)
		},
		Encode: func(m *UplinkNASTransport) (per.Marshaler, bool) {
			if m.NASPDU == nil {
				return nil, false
			}

			return &m.NASPDU, true
		},
	},
	{
		ID: IDUserLocationInformation, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UplinkNASTransport, r *per.Reader, enc per.Encoding) error {
			var v UserLocationInformation

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.UserLocationInformation = &v

			return nil
		},
		Encode: func(m *UplinkNASTransport) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
	apimpl.OptionalIE(IDWAGFIdentityInformation, CriticalityReject, func(m *UplinkNASTransport) **IdentityInformation { return &m.WAGFIdentityInformation }),
	apimpl.OptionalIE(IDTNGFIdentityInformation, CriticalityReject, func(m *UplinkNASTransport) **IdentityInformation { return &m.TNGFIdentityInformation }),
	apimpl.OptionalIE(IDTWIFIdentityInformation, CriticalityReject, func(m *UplinkNASTransport) **IdentityInformation { return &m.TWIFIdentityInformation }),
}

func (m *UplinkNASTransport) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUplinkNASTransport,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func (m *UplinkNASTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUplinkNASTransport, uplinkNASTransportIEs, m)
}

func ParseUplinkNASTransport(value []byte) (*UplinkNASTransport, error) {
	return parseMessageBody[UplinkNASTransport](ProcUplinkNASTransport, TriggeringInitiatingMessage, uplinkNASTransportIEs, value)
}

func DecodeUplinkNASTransport(value []byte) (*UplinkNASTransport, error) {
	return decodeMessageBody[UplinkNASTransport](ProcUplinkNASTransport, uplinkNASTransportIEs, value)
}

// TS 38.413 §9.2.5.2.
//
// NGAP places NAS-PDU after the optional Old AMF and RAN Paging Priority IEs
// where S1AP puts it third, but neither optional IE is modeled, so the modeled
// table order matches.
type DownlinkNASTransport struct {
	AMFUENGAPID                 AMFUENGAPID
	RANUENGAPID                 RANUENGAPID
	NASPDU                      NASPDU
	OldAMF                      *Name
	RANPagingPriority           *RANPagingPriority
	MobilityRestrictionList     *MobilityRestrictionList
	IndexToRFSP                 *IndexToRFSP
	UEAggregateMaximumBitRate   *UEAggregateMaximumBitRate
	AllowedNSSAI                AllowedNSSAI
	SRVCCOperationPossible      *SRVCCOperationPossible
	EnhancedCoverageRestriction *EnhancedCoverageRestriction
	ExtendedConnectedTime       *ExtendedConnectedTime
	UEDifferentiationInfo       *UEDifferentiationInfo
	CEmodeBrestricted           *CEmodeBrestricted
	UERadioCapability           UERadioCapability
	UECapabilityInfoRequest     *UECapabilityInfoRequest
	EndIndication               *EndIndication
	UERadioCapabilityID         *UERadioCapabilityID
	TargetNSSAIInformation      *TargetNSSAIInformation
	MaskedIMEISV                *MaskedIMEISV
	PartiallyAllowedNSSAI       PartiallyAllowedNSSAI
	MobileIABAuthorized         *MobileIABAuthorized
	ExtendedOldAMF              *ExtendedAMFName

	messageMeta
}

var downlinkNASTransportIEs = []ieSpec[DownlinkNASTransport]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *DownlinkNASTransport, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFUENGAPID)
		},
		Encode: func(m *DownlinkNASTransport) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *DownlinkNASTransport, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *DownlinkNASTransport) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	apimpl.OptionalIE(IDOldAMF, CriticalityReject, func(m *DownlinkNASTransport) **Name { return &m.OldAMF }),
	apimpl.OptionalIE(IDRANPagingPriority, CriticalityIgnore, func(m *DownlinkNASTransport) **RANPagingPriority { return &m.RANPagingPriority }),
	{
		ID: IDNASPDU, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *DownlinkNASTransport, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.NASPDU)
		},
		Encode: func(m *DownlinkNASTransport) (per.Marshaler, bool) {
			if m.NASPDU == nil {
				return nil, false
			}

			return &m.NASPDU, true
		},
	},
	apimpl.OptionalIE(IDMobilityRestrictionList, CriticalityIgnore, func(m *DownlinkNASTransport) **MobilityRestrictionList { return &m.MobilityRestrictionList }),
	apimpl.OptionalIE(IDIndexToRFSP, CriticalityIgnore, func(m *DownlinkNASTransport) **IndexToRFSP { return &m.IndexToRFSP }),
	apimpl.OptionalIE(IDUEAggregateMaximumBitRate, CriticalityIgnore, func(m *DownlinkNASTransport) **UEAggregateMaximumBitRate { return &m.UEAggregateMaximumBitRate }),
	apimpl.ListIE(IDAllowedNSSAI, CriticalityReject, func(m *DownlinkNASTransport) *AllowedNSSAI { return &m.AllowedNSSAI }),
	apimpl.OptionalIE(IDSRVCCOperationPossible, CriticalityIgnore, func(m *DownlinkNASTransport) **SRVCCOperationPossible { return &m.SRVCCOperationPossible }),
	apimpl.OptionalIE(IDEnhancedCoverageRestriction, CriticalityIgnore, func(m *DownlinkNASTransport) **EnhancedCoverageRestriction { return &m.EnhancedCoverageRestriction }),
	apimpl.OptionalIE(IDExtendedConnectedTime, CriticalityIgnore, func(m *DownlinkNASTransport) **ExtendedConnectedTime { return &m.ExtendedConnectedTime }),
	apimpl.OptionalIE(IDUEDifferentiationInfo, CriticalityIgnore, func(m *DownlinkNASTransport) **UEDifferentiationInfo { return &m.UEDifferentiationInfo }),
	apimpl.OptionalIE(IDCEmodeBrestricted, CriticalityIgnore, func(m *DownlinkNASTransport) **CEmodeBrestricted { return &m.CEmodeBrestricted }),
	apimpl.ListIE(IDUERadioCapability, CriticalityIgnore, func(m *DownlinkNASTransport) *UERadioCapability { return &m.UERadioCapability }),
	apimpl.OptionalIE(IDUECapabilityInfoRequest, CriticalityIgnore, func(m *DownlinkNASTransport) **UECapabilityInfoRequest { return &m.UECapabilityInfoRequest }),
	apimpl.OptionalIE(IDEndIndication, CriticalityIgnore, func(m *DownlinkNASTransport) **EndIndication { return &m.EndIndication }),
	apimpl.OptionalIE(IDUERadioCapabilityID, CriticalityReject, func(m *DownlinkNASTransport) **UERadioCapabilityID { return &m.UERadioCapabilityID }),
	apimpl.OptionalIE(IDTargetNSSAIInformation, CriticalityIgnore, func(m *DownlinkNASTransport) **TargetNSSAIInformation { return &m.TargetNSSAIInformation }),
	apimpl.OptionalIE(IDMaskedIMEISV, CriticalityIgnore, func(m *DownlinkNASTransport) **MaskedIMEISV { return &m.MaskedIMEISV }),
	apimpl.ListIE(IDPartiallyAllowedNSSAI, CriticalityIgnore, func(m *DownlinkNASTransport) *PartiallyAllowedNSSAI { return &m.PartiallyAllowedNSSAI }),
	apimpl.OptionalIE(IDMobileIABAuthorized, CriticalityIgnore, func(m *DownlinkNASTransport) **MobileIABAuthorized { return &m.MobileIABAuthorized }),
	apimpl.OptionalIE(IDExtendedOldAMF, CriticalityIgnore, func(m *DownlinkNASTransport) **ExtendedAMFName { return &m.ExtendedOldAMF }),
}

func (m *DownlinkNASTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcDownlinkNASTransport, downlinkNASTransportIEs, m)
}

func (m *DownlinkNASTransport) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcDownlinkNASTransport,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseDownlinkNASTransport(value []byte) (*DownlinkNASTransport, error) {
	return parseMessageBody[DownlinkNASTransport](ProcDownlinkNASTransport, TriggeringInitiatingMessage, downlinkNASTransportIEs, value)
}

func DecodeDownlinkNASTransport(value []byte) (*DownlinkNASTransport, error) {
	return decodeMessageBody[DownlinkNASTransport](ProcDownlinkNASTransport, downlinkNASTransportIEs, value)
}
