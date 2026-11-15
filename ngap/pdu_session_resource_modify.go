// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// PDUSessionResourceModifyItemModReq ::= SEQUENCE { pDUSessionID, nAS-PDU
// OPTIONAL, pDUSessionResourceModifyRequestTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceModifyItemModReq struct {
	PDUSessionID                          PDUSessionID
	NASPDU                                *NASPDU `per:",optional"`
	Transfer                              TransferContainer
	SNSSAI                                *SNSSAI                      `per:",extie:IDSNSSAI,crit:reject"`
	PduSessionExpectedUEActivityBehaviour *ExpectedUEActivityBehaviour `per:",extie:IDPduSessionExpectedUEActivityBehaviour,crit:ignore"`
	iEExtensions                          extensionMeta                `per:",exttable"`
	extensions                            per.ExtensionAdditions       `per:",extraw"`
}

// PDUSessionResourceModifyListModReq ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceModifyItemModReq.
type PDUSessionResourceModifyListModReq []PDUSessionResourceModifyItemModReq

// PDUSessionResourceModifyItemModRes ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceModifyResponseTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceModifyItemModRes struct {
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

// PDUSessionResourceModifyListModRes ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceModifyItemModRes.
type PDUSessionResourceModifyListModRes []PDUSessionResourceModifyItemModRes

// PDUSessionResourceFailedToModifyItemModRes ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceModifyUnsuccessfulTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceFailedToModifyItemModRes struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta `per:",exttable"`
}

// PDUSessionResourceFailedToModifyListModRes ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceFailedToModifyItemModRes.
type PDUSessionResourceFailedToModifyListModRes []PDUSessionResourceFailedToModifyItemModRes

// TS 38.413 §9.2.1.5.
type PDUSessionResourceModifyRequest struct {
	AMFUENGAPID              AMFUENGAPID
	RANUENGAPID              RANUENGAPID
	PDUSessionResourceModify PDUSessionResourceModifyListModReq
	RANPagingPriority        *RANPagingPriority

	messageMeta
}

var pDUSessionResourceModifyRequestIEs = []ieSpec[PDUSessionResourceModifyRequest]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceModifyRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFUENGAPID)
		},
		Encode: func(m *PDUSessionResourceModifyRequest) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceModifyRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *PDUSessionResourceModifyRequest) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	optionalExtension(IDRANPagingPriority, CriticalityIgnore, func(m *PDUSessionResourceModifyRequest) **RANPagingPriority { return &m.RANPagingPriority }),
	{
		ID: IDPDUSessionResourceModifyListModReq, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceModifyRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceModify)
		},
		Encode: func(m *PDUSessionResourceModifyRequest) (per.Marshaler, bool) {
			if m.PDUSessionResourceModify == nil {
				return nil, false
			}

			return m.PDUSessionResourceModify, true
		},
	},
}

func (m *PDUSessionResourceModifyRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPDUSessionResourceModify, pDUSessionResourceModifyRequestIEs, m)
}

func (m *PDUSessionResourceModifyRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcPDUSessionResourceModify,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePDUSessionResourceModifyRequest(value []byte) (*PDUSessionResourceModifyRequest, error) {
	return parseMessageBody[PDUSessionResourceModifyRequest](ProcPDUSessionResourceModify, TriggeringInitiatingMessage, pDUSessionResourceModifyRequestIEs, value)
}

func DecodePDUSessionResourceModifyRequest(value []byte) (*PDUSessionResourceModifyRequest, error) {
	return decodeMessageBody[PDUSessionResourceModifyRequest](ProcPDUSessionResourceModify, pDUSessionResourceModifyRequestIEs, value)
}

// TS 38.413 §9.2.1.6.
type PDUSessionResourceModifyResponse struct {
	AMFUENGAPID              *AMFUENGAPID
	RANUENGAPID              *RANUENGAPID
	PDUSessionResourceModify PDUSessionResourceModifyListModRes
	PDUSessionResourceFailed PDUSessionResourceFailedToModifyListModRes
	UserLocationInformation  *UserLocationInformation
	CriticalityDiagnostics   *CriticalityDiagnostics

	messageMeta
}

var pDUSessionResourceModifyResponseIEs = []ieSpec[PDUSessionResourceModifyResponse]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceModifyResponse, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		Encode: func(m *PDUSessionResourceModifyResponse) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceModifyResponse, r *per.Reader, enc per.Encoding) error {
			var v RANUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		Encode: func(m *PDUSessionResourceModifyResponse) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		ID: IDPDUSessionResourceModifyListModRes, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceModifyResponse, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceModify)
		},
		Encode: func(m *PDUSessionResourceModifyResponse) (per.Marshaler, bool) {
			if m.PDUSessionResourceModify == nil {
				return nil, false
			}

			return m.PDUSessionResourceModify, true
		},
	},
	{
		ID: IDPDUSessionResourceFailedToModifyListModRes, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceModifyResponse, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceFailed)
		},
		Encode: func(m *PDUSessionResourceModifyResponse) (per.Marshaler, bool) {
			if m.PDUSessionResourceFailed == nil {
				return nil, false
			}

			return m.PDUSessionResourceFailed, true
		},
	},
	{
		ID: IDUserLocationInformation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceModifyResponse, r *per.Reader, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := decodeIE(r, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		Encode: func(m *PDUSessionResourceModifyResponse) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceModifyResponse, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *PDUSessionResourceModifyResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *PDUSessionResourceModifyResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPDUSessionResourceModify, pDUSessionResourceModifyResponseIEs, m)
}

func (m *PDUSessionResourceModifyResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcPDUSessionResourceModify,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePDUSessionResourceModifyResponse(value []byte) (*PDUSessionResourceModifyResponse, error) {
	return parseMessageBody[PDUSessionResourceModifyResponse](ProcPDUSessionResourceModify, TriggeringSuccessfulOutcome, pDUSessionResourceModifyResponseIEs, value)
}

func DecodePDUSessionResourceModifyResponse(value []byte) (*PDUSessionResourceModifyResponse, error) {
	return decodeMessageBody[PDUSessionResourceModifyResponse](ProcPDUSessionResourceModify, pDUSessionResourceModifyResponseIEs, value)
}

// PDUSessionResourceModifyRequestTransfer ::= SEQUENCE { protocolIEs
// ProtocolIE-Container } (extensible) — TS 38.413 §9.3.4.3. An IE container
// like the setup request transfer. Every IE is optional: a modify carries only
// what changes. SecurityIndication is ignore criticality here where the setup
// request transfer marks it reject, so it is left to the unknown-IE path.
type PDUSessionResourceModifyRequestTransfer struct {
	PDUSessionAggregateMaximumBitRate        *PDUSessionAggregateMaximumBitRate
	ULNGUUPTNLModify                         ULNGUUPTNLModifyList
	NetworkInstance                          *NetworkInstance
	QosFlowAddOrModifyRequest                QosFlowAddOrModifyRequestList
	QosFlowToRelease                         QosFlowListWithCause
	AdditionalULNGUUPTNLInformation          UPTransportLayerInformationList
	AdditionalRedundantULNGUUPTNLInformation UPTransportLayerInformationList
	RedundantULNGUUPTNLInformation           *UPTransportLayerInformation
	UserPlaneFailureIndication               *UserPlaneFailureIndication
	CommonNetworkInstance                    CommonNetworkInstance
	RedundantCommonNetworkInstance           CommonNetworkInstance
	SecurityIndication                       *SecurityIndication
	MBSSessionSetuporModifyRequestList       MBSSessionSetuporModifyRequestList
	MBSSessionToReleaseList                  MBSSessionToReleaseList

	messageMeta
}

var pDUSessionResourceModifyRequestTransferIEs = []ieSpec[PDUSessionResourceModifyRequestTransfer]{
	{
		ID: IDPDUSessionAggregateMaximumBitRate, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceModifyRequestTransfer, r *per.Reader, enc per.Encoding) error {
			var v PDUSessionAggregateMaximumBitRate

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.PDUSessionAggregateMaximumBitRate = &v

			return nil
		},
		Encode: func(m *PDUSessionResourceModifyRequestTransfer) (per.Marshaler, bool) {
			if m.PDUSessionAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.PDUSessionAggregateMaximumBitRate, true
		},
	},
	{
		ID: IDULNGUUPTNLModifyList, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceModifyRequestTransfer, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ULNGUUPTNLModify)
		},
		Encode: func(m *PDUSessionResourceModifyRequestTransfer) (per.Marshaler, bool) {
			if m.ULNGUUPTNLModify == nil {
				return nil, false
			}

			return m.ULNGUUPTNLModify, true
		},
	},
	{
		ID: IDNetworkInstance, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceModifyRequestTransfer, r *per.Reader, enc per.Encoding) error {
			var v NetworkInstance

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.NetworkInstance = &v

			return nil
		},
		Encode: func(m *PDUSessionResourceModifyRequestTransfer) (per.Marshaler, bool) {
			if m.NetworkInstance == nil {
				return nil, false
			}

			return m.NetworkInstance, true
		},
	},
	{
		ID: IDQosFlowAddOrModifyRequestList, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceModifyRequestTransfer, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.QosFlowAddOrModifyRequest)
		},
		Encode: func(m *PDUSessionResourceModifyRequestTransfer) (per.Marshaler, bool) {
			if m.QosFlowAddOrModifyRequest == nil {
				return nil, false
			}

			return m.QosFlowAddOrModifyRequest, true
		},
	},
	{
		ID: IDQosFlowToReleaseList, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceModifyRequestTransfer, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.QosFlowToRelease)
		},
		Encode: func(m *PDUSessionResourceModifyRequestTransfer) (per.Marshaler, bool) {
			if m.QosFlowToRelease == nil {
				return nil, false
			}

			return m.QosFlowToRelease, true
		},
	},
	{
		ID: IDAdditionalULNGUUPTNLInformation, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceModifyRequestTransfer, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AdditionalULNGUUPTNLInformation)
		},
		Encode: func(m *PDUSessionResourceModifyRequestTransfer) (per.Marshaler, bool) {
			if m.AdditionalULNGUUPTNLInformation == nil {
				return nil, false
			}

			return m.AdditionalULNGUUPTNLInformation, true
		},
	},
	listExtension(IDCommonNetworkInstance, CriticalityIgnore, func(m *PDUSessionResourceModifyRequestTransfer) *CommonNetworkInstance {
		return &m.CommonNetworkInstance
	}),
	listExtension(IDAdditionalRedundantULNGUUPTNLInformation, CriticalityIgnore, func(m *PDUSessionResourceModifyRequestTransfer) *UPTransportLayerInformationList {
		return &m.AdditionalRedundantULNGUUPTNLInformation
	}),
	listExtension(IDRedundantCommonNetworkInstance, CriticalityIgnore, func(m *PDUSessionResourceModifyRequestTransfer) *CommonNetworkInstance {
		return &m.RedundantCommonNetworkInstance
	}),
	optionalExtension(IDRedundantULNGUUPTNLInformation, CriticalityIgnore, func(m *PDUSessionResourceModifyRequestTransfer) **UPTransportLayerInformation {
		return &m.RedundantULNGUUPTNLInformation
	}),
	optionalExtension(IDSecurityIndication, CriticalityIgnore, func(m *PDUSessionResourceModifyRequestTransfer) **SecurityIndication { return &m.SecurityIndication }),
	listExtension(IDMBSSessionSetuporModifyRequestList, CriticalityIgnore, func(m *PDUSessionResourceModifyRequestTransfer) *MBSSessionSetuporModifyRequestList {
		return &m.MBSSessionSetuporModifyRequestList
	}),
	listExtension(IDMBSSessionToReleaseList, CriticalityIgnore, func(m *PDUSessionResourceModifyRequestTransfer) *MBSSessionToReleaseList {
		return &m.MBSSessionToReleaseList
	}),
	optionalExtension(IDUserPlaneFailureIndication, CriticalityIgnore, func(m *PDUSessionResourceModifyRequestTransfer) **UserPlaneFailureIndication {
		return &m.UserPlaneFailureIndication
	}),
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *PDUSessionResourceModifyRequestTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := encodeMessageBody(w, per.Aligned, ProcPDUSessionResourceModify, pDUSessionResourceModifyRequestTransferIEs, t); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParsePDUSessionResourceModifyRequestTransfer decodes the transfer the SMF
// sends for a session the NG-RAN node is to modify.
func ParsePDUSessionResourceModifyRequestTransfer(b TransferContainer) (*PDUSessionResourceModifyRequestTransfer, error) {
	return parseMessageBody[PDUSessionResourceModifyRequestTransfer](ProcPDUSessionResourceModify, TriggeringInitiatingMessage, pDUSessionResourceModifyRequestTransferIEs, b)
}

func DecodePDUSessionResourceModifyRequestTransfer(b TransferContainer) (*PDUSessionResourceModifyRequestTransfer, error) {
	return decodeMessageBody[PDUSessionResourceModifyRequestTransfer](ProcPDUSessionResourceModify, pDUSessionResourceModifyRequestTransferIEs, b)
}

// QosFlowAddOrModifyResponseItem ::= SEQUENCE { qosFlowIdentifier,
// iE-Extensions OPTIONAL } (extensible), an inline type of the TS 38.413
// §9.3.4.4 table.
type QosFlowAddOrModifyResponseItem struct {
	QosFlowIdentifier                 QosFlowIdentifier
	CurrentQoSParaSetIndex            *AlternativeQoSParaSetIndex        `per:",extie:IDCurrentQoSParaSetIndex,crit:ignore"`
	TSCTrafficCharacteristicsFeedback *TSCTrafficCharacteristicsFeedback `per:",extie:IDTSCTrafficCharacteristicsFeedback,crit:ignore"`
	UplinkTLContainer                 *TLContainer                       `per:",extie:IDUplinkTLContainer,crit:ignore"`
	DLNGUUPTNLInformation             *UPTransportLayerInformation       `per:",extie:IDDLNGUUPTNLInformation,crit:ignore"`
	ANPacketDelayBudgetUL             *ExtendedPacketDelayBudget         `per:",extie:IDANPacketDelayBudgetUL,crit:ignore"`
	iEExtensions                      extensionMeta                      `per:",exttable"`
	extensions                        per.ExtensionAdditions             `per:",extraw"`
}

// QosFlowAddOrModifyResponseList ::= SEQUENCE (SIZE(1..maxnoofQosFlows)) OF
// QosFlowAddOrModifyResponseItem.
type QosFlowAddOrModifyResponseList []QosFlowAddOrModifyResponseItem

// PDUSessionResourceModifyResponseTransfer ::= SEQUENCE {
// dL-NGU-UP-TNLInformation OPTIONAL, uL-NGU-UP-TNLInformation OPTIONAL,
// qosFlowAddOrModifyResponseList OPTIONAL, additionalDLQosFlowPerTNLInformation
// OPTIONAL, qosFlowFailedToAddOrModifyList OPTIONAL, iE-Extensions OPTIONAL }
// (extensible) — TS 38.413 §9.3.4.4. Every field is optional: an NG-RAN node
// that accepted a modification without changing a tunnel sends it empty.
type PDUSessionResourceModifyResponseTransfer struct {
	DLNGUUPTNLInformation                            *UPTransportLayerInformation                     `per:",optional"`
	ULNGUUPTNLInformation                            *UPTransportLayerInformation                     `per:",optional"`
	QosFlowAddOrModifyResponse                       QosFlowAddOrModifyResponseList                   `per:",optional"`
	AdditionalDLQosFlowPerTNLInformation             QosFlowPerTNLInformationList                     `per:",optional"`
	QosFlowFailedToAddOrModify                       QosFlowListWithCause                             `per:",optional"`
	AdditionalNGUUPTNLInformation                    UPTransportLayerInformationPairList              `per:",extie:IDAdditionalNGUUPTNLInformation,crit:ignore"`
	RedundantDLNGUUPTNLInformation                   *UPTransportLayerInformation                     `per:",extie:IDRedundantDLNGUUPTNLInformation,crit:ignore"`
	RedundantULNGUUPTNLInformation                   *UPTransportLayerInformation                     `per:",extie:IDRedundantULNGUUPTNLInformation,crit:ignore"`
	AdditionalRedundantDLQosFlowPerTNLInformation    QosFlowPerTNLInformationList                     `per:",extie:IDAdditionalRedundantDLQosFlowPerTNLInformation,crit:ignore"`
	AdditionalRedundantNGUUPTNLInformation           UPTransportLayerInformationPairList              `per:",extie:IDAdditionalRedundantNGUUPTNLInformation,crit:ignore"`
	SecondaryRATUsageInformation                     *SecondaryRATUsageInformation                    `per:",extie:IDSecondaryRATUsageInformation,crit:ignore"`
	MBSSupportIndicator                              *MBSSupportIndicator                             `per:",extie:IDMBSSupportIndicator,crit:ignore"`
	MBSSessionSetuporModifyResponseList              MBSSessionSetupResponseList                      `per:",extie:IDMBSSessionSetuporModifyResponseList,crit:ignore"`
	MBSSessionFailedtoSetuporModifyList              MBSSessionFailedtoSetupList                      `per:",extie:IDMBSSessionFailedtoSetuporModifyList,crit:ignore"`
	ECNMarkingorCongestionInformationReportingStatus ECNMarkingorCongestionInformationReportingStatus `per:",extie:IDECNMarkingorCongestionInformationReportingStatus,crit:ignore"`
	PDUSetbasedHandlingIndicator                     *PDUSetbasedHandlingIndicator                    `per:",extie:IDPDUSetbasedHandlingIndicator,crit:ignore"`
	UserPlaneFailureIndicationReport                 *UserPlaneFailureIndicationReport                `per:",extie:IDUserPlaneFailureIndicationReport,crit:ignore"`
	iEExtensions                                     extensionMeta                                    `per:",exttable"`
	extensions                                       per.ExtensionAdditions                           `per:",extraw"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *PDUSessionResourceModifyResponseTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// PDUSessionResourceModifyUnsuccessfulTransfer ::= SEQUENCE { cause,
// criticalityDiagnostics OPTIONAL, iE-Extensions OPTIONAL } (extensible) —
// TS 38.413 §9.3.4.17. Carried in the Failed to Modify list of the modify
// response.
type PDUSessionResourceModifyUnsuccessfulTransfer struct {
	extensions             per.ExtensionAdditions `per:",extraw"`
	Cause                  Cause
	CriticalityDiagnostics *CriticalityDiagnostics `per:",optional"`
	iEExtensions           extensionMeta           `per:",exttable"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *PDUSessionResourceModifyUnsuccessfulTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParsePDUSessionResourceModifyUnsuccessfulTransfer decodes the transfer an
// NG-RAN node returns for a session it could not modify.
func ParsePDUSessionResourceModifyUnsuccessfulTransfer(b TransferContainer) (*PDUSessionResourceModifyUnsuccessfulTransfer, error) {
	return parseTransfer[PDUSessionResourceModifyUnsuccessfulTransfer](b)
}

func DecodePDUSessionResourceModifyUnsuccessfulTransfer(b TransferContainer) (*PDUSessionResourceModifyUnsuccessfulTransfer, error) {
	return decodeTransfer[PDUSessionResourceModifyUnsuccessfulTransfer](b)
}

func (t *PDUSessionResourceModifyUnsuccessfulTransfer) Diagnostics() Diagnostics {
	return diagnosticsOf(t)
}

// ParsePDUSessionResourceModifyResponseTransfer decodes the transfer the NG-RAN
// node returns for a modified session.
func ParsePDUSessionResourceModifyResponseTransfer(b TransferContainer) (*PDUSessionResourceModifyResponseTransfer, error) {
	return parseTransfer[PDUSessionResourceModifyResponseTransfer](b)
}

func DecodePDUSessionResourceModifyResponseTransfer(b TransferContainer) (*PDUSessionResourceModifyResponseTransfer, error) {
	return decodeTransfer[PDUSessionResourceModifyResponseTransfer](b)
}

func (t *PDUSessionResourceModifyResponseTransfer) Diagnostics() Diagnostics { return diagnosticsOf(t) }
