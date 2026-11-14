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
	PDUSessionID PDUSessionID
	NASPDU       *NASPDU `per:",optional"`
	Transfer     TransferContainer
	SNSSAI       *SNSSAI                `per:"-"`
	iEExtensions messageMeta            `per:",exttable:pDUSessionResourceModifyItemModReqExtIEs"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

var pDUSessionResourceModifyItemModReqExtIEs = []ieSpec[PDUSessionResourceModifyItemModReq]{
	optionalExtension(IDSNSSAI, CriticalityReject, func(m *PDUSessionResourceModifyItemModReq) **SNSSAI {
		return &m.SNSSAI
	}),
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
	iEExtensions *rawIEExtensions       `per:",optional"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

// PDUSessionResourceModifyListModRes ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceModifyItemModRes.
type PDUSessionResourceModifyListModRes []PDUSessionResourceModifyItemModRes

// PDUSessionResourceFailedToModifyItemModRes ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceModifyUnsuccessfulTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceFailedToModifyItemModRes struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceFailedToModifyListModRes ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceFailedToModifyItemModRes.
type PDUSessionResourceFailedToModifyListModRes []PDUSessionResourceFailedToModifyItemModRes

// TS 38.413 §9.2.1.5.
type PDUSessionResourceModifyRequest struct {
	AMFUENGAPID              AMFUENGAPID
	RANUENGAPID              RANUENGAPID
	PDUSessionResourceModify PDUSessionResourceModifyListModReq

	messageMeta
}

var pDUSessionResourceModifyRequestIEs = []ieSpec[PDUSessionResourceModifyRequest]{
	{
		id: IDAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceModifyRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *PDUSessionResourceModifyRequest) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: IDRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceModifyRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *PDUSessionResourceModifyRequest) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: IDPDUSessionResourceModifyListModReq, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceModifyRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceModify)
		},
		encode: func(m *PDUSessionResourceModifyRequest) (per.Marshaler, bool) {
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
		id: IDAMFUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceModifyResponse, raw []byte, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		encode: func(m *PDUSessionResourceModifyResponse) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		id: IDRANUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceModifyResponse, raw []byte, enc per.Encoding) error {
			var v RANUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		encode: func(m *PDUSessionResourceModifyResponse) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		id: IDPDUSessionResourceModifyListModRes, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceModifyResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceModify)
		},
		encode: func(m *PDUSessionResourceModifyResponse) (per.Marshaler, bool) {
			if m.PDUSessionResourceModify == nil {
				return nil, false
			}

			return m.PDUSessionResourceModify, true
		},
	},
	{
		id: IDPDUSessionResourceFailedToModifyListModRes, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceModifyResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceFailed)
		},
		encode: func(m *PDUSessionResourceModifyResponse) (per.Marshaler, bool) {
			if m.PDUSessionResourceFailed == nil {
				return nil, false
			}

			return m.PDUSessionResourceFailed, true
		},
	},
	{
		id: IDUserLocationInformation, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceModifyResponse, raw []byte, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := perIEDecode(raw, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		encode: func(m *PDUSessionResourceModifyResponse) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceModifyResponse, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *PDUSessionResourceModifyResponse) (per.Marshaler, bool) {
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

	messageMeta
}

var pDUSessionResourceModifyRequestTransferIEs = []ieSpec[PDUSessionResourceModifyRequestTransfer]{
	{
		id: IDPDUSessionAggregateMaximumBitRate, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceModifyRequestTransfer, raw []byte, enc per.Encoding) error {
			var v PDUSessionAggregateMaximumBitRate

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.PDUSessionAggregateMaximumBitRate = &v

			return nil
		},
		encode: func(m *PDUSessionResourceModifyRequestTransfer) (per.Marshaler, bool) {
			if m.PDUSessionAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.PDUSessionAggregateMaximumBitRate, true
		},
	},
	{
		id: IDULNGUUPTNLModifyList, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceModifyRequestTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ULNGUUPTNLModify)
		},
		encode: func(m *PDUSessionResourceModifyRequestTransfer) (per.Marshaler, bool) {
			if m.ULNGUUPTNLModify == nil {
				return nil, false
			}

			return m.ULNGUUPTNLModify, true
		},
	},
	{
		id: IDNetworkInstance, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceModifyRequestTransfer, raw []byte, enc per.Encoding) error {
			var v NetworkInstance

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.NetworkInstance = &v

			return nil
		},
		encode: func(m *PDUSessionResourceModifyRequestTransfer) (per.Marshaler, bool) {
			if m.NetworkInstance == nil {
				return nil, false
			}

			return m.NetworkInstance, true
		},
	},
	{
		id: IDQosFlowAddOrModifyRequestList, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceModifyRequestTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.QosFlowAddOrModifyRequest)
		},
		encode: func(m *PDUSessionResourceModifyRequestTransfer) (per.Marshaler, bool) {
			if m.QosFlowAddOrModifyRequest == nil {
				return nil, false
			}

			return m.QosFlowAddOrModifyRequest, true
		},
	},
	{
		id: IDQosFlowToReleaseList, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceModifyRequestTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.QosFlowToRelease)
		},
		encode: func(m *PDUSessionResourceModifyRequestTransfer) (per.Marshaler, bool) {
			if m.QosFlowToRelease == nil {
				return nil, false
			}

			return m.QosFlowToRelease, true
		},
	},
	{
		id: IDAdditionalULNGUUPTNLInformation, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceModifyRequestTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AdditionalULNGUUPTNLInformation)
		},
		encode: func(m *PDUSessionResourceModifyRequestTransfer) (per.Marshaler, bool) {
			if m.AdditionalULNGUUPTNLInformation == nil {
				return nil, false
			}

			return m.AdditionalULNGUUPTNLInformation, true
		},
	},
	listExtension(IDAdditionalRedundantULNGUUPTNLInformation, CriticalityIgnore, func(m *PDUSessionResourceModifyRequestTransfer) *UPTransportLayerInformationList {
		return &m.AdditionalRedundantULNGUUPTNLInformation
	}),
	optionalExtension(IDRedundantULNGUUPTNLInformation, CriticalityIgnore, func(m *PDUSessionResourceModifyRequestTransfer) **UPTransportLayerInformation {
		return &m.RedundantULNGUUPTNLInformation
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
	return parseBoundedMessageBody[PDUSessionResourceModifyRequestTransfer](ProcPDUSessionResourceModify, TriggeringInitiatingMessage, pDUSessionResourceModifyRequestTransferIEs, b, maxProtocolIEs)
}

// QosFlowAddOrModifyResponseItem ::= SEQUENCE { qosFlowIdentifier,
// iE-Extensions OPTIONAL } (extensible), an inline type of the TS 38.413
// §9.3.4.4 table.
type QosFlowAddOrModifyResponseItem struct {
	QosFlowIdentifier     QosFlowIdentifier
	DLNGUUPTNLInformation *UPTransportLayerInformation `per:"-"`
	iEExtensions          messageMeta                  `per:",exttable:qosFlowAddOrModifyResponseItemExtIEs"`
	extensions            per.ExtensionAdditions       `per:",extraw"`
}

var qosFlowAddOrModifyResponseItemExtIEs = []ieSpec[QosFlowAddOrModifyResponseItem]{
	optionalExtension(IDDLNGUUPTNLInformation, CriticalityIgnore, func(m *QosFlowAddOrModifyResponseItem) **UPTransportLayerInformation {
		return &m.DLNGUUPTNLInformation
	}),
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
	DLNGUUPTNLInformation                         *UPTransportLayerInformation        `per:",optional"`
	ULNGUUPTNLInformation                         *UPTransportLayerInformation        `per:",optional"`
	QosFlowAddOrModifyResponse                    QosFlowAddOrModifyResponseList      `per:",optional"`
	AdditionalDLQosFlowPerTNLInformation          QosFlowPerTNLInformationList        `per:",optional"`
	QosFlowFailedToAddOrModify                    QosFlowListWithCause                `per:",optional"`
	AdditionalNGUUPTNLInformation                 UPTransportLayerInformationPairList `per:"-"`
	RedundantDLNGUUPTNLInformation                *UPTransportLayerInformation        `per:"-"`
	RedundantULNGUUPTNLInformation                *UPTransportLayerInformation        `per:"-"`
	AdditionalRedundantDLQosFlowPerTNLInformation QosFlowPerTNLInformationList        `per:"-"`
	AdditionalRedundantNGUUPTNLInformation        UPTransportLayerInformationPairList `per:"-"`
	iEExtensions                                  messageMeta                         `per:",exttable:pDUSessionResourceModifyResponseTransferExtIEs"`
	extensions                                    per.ExtensionAdditions              `per:",extraw"`
}

var pDUSessionResourceModifyResponseTransferExtIEs = []ieSpec[PDUSessionResourceModifyResponseTransfer]{
	listExtension(IDAdditionalNGUUPTNLInformation, CriticalityIgnore, func(m *PDUSessionResourceModifyResponseTransfer) *UPTransportLayerInformationPairList {
		return &m.AdditionalNGUUPTNLInformation
	}),
	optionalExtension(IDRedundantDLNGUUPTNLInformation, CriticalityIgnore, func(m *PDUSessionResourceModifyResponseTransfer) **UPTransportLayerInformation {
		return &m.RedundantDLNGUUPTNLInformation
	}),
	optionalExtension(IDRedundantULNGUUPTNLInformation, CriticalityIgnore, func(m *PDUSessionResourceModifyResponseTransfer) **UPTransportLayerInformation {
		return &m.RedundantULNGUUPTNLInformation
	}),
	listExtension(IDAdditionalRedundantDLQosFlowPerTNLInformation, CriticalityIgnore, func(m *PDUSessionResourceModifyResponseTransfer) *QosFlowPerTNLInformationList {
		return &m.AdditionalRedundantDLQosFlowPerTNLInformation
	}),
	listExtension(IDAdditionalRedundantNGUUPTNLInformation, CriticalityIgnore, func(m *PDUSessionResourceModifyResponseTransfer) *UPTransportLayerInformationPairList {
		return &m.AdditionalRedundantNGUUPTNLInformation
	}),
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
	_                      [0]struct{} `per:"extseq"`
	Cause                  Cause
	CriticalityDiagnostics *CriticalityDiagnostics `per:",optional"`
	_                      ieExtensions            `per:",skip"`
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
	var t PDUSessionResourceModifyUnsuccessfulTransfer

	if err := t.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, err
	}

	return &t, nil
}

// ParsePDUSessionResourceModifyResponseTransfer decodes the transfer the NG-RAN
// node returns for a modified session.
func ParsePDUSessionResourceModifyResponseTransfer(b TransferContainer) (*PDUSessionResourceModifyResponseTransfer, error) {
	var t PDUSessionResourceModifyResponseTransfer

	if err := t.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, err
	}

	return &t, nil
}
