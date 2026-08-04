// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// PDUSessionResourceSetupItemSUReq ::= SEQUENCE { pDUSessionID,
// pDUSessionNAS-PDU OPTIONAL, s-NSSAI, pDUSessionResourceSetupRequestTransfer,
// iE-Extensions OPTIONAL } (extensible). Its own ASN.1 type, so it stays
// distinct from the identically shaped CxtReq item.
type PDUSessionResourceSetupItemSUReq struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	NASPDU       *NASPDU `per:",optional"`
	SNSSAI       SNSSAI
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceSetupListSUReq ::= SEQUENCE (SIZE(1..maxnoofPDUSessions))
// OF PDUSessionResourceSetupItemSUReq.
type PDUSessionResourceSetupListSUReq []PDUSessionResourceSetupItemSUReq

// PDUSessionResourceSetupItemSURes ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceSetupResponseTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceSetupItemSURes struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceSetupListSURes ::= SEQUENCE (SIZE(1..maxnoofPDUSessions))
// OF PDUSessionResourceSetupItemSURes.
type PDUSessionResourceSetupListSURes []PDUSessionResourceSetupItemSURes

// PDUSessionResourceFailedToSetupItemSURes ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceSetupUnsuccessfulTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceFailedToSetupItemSURes struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceFailedToSetupListSURes ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceFailedToSetupItemSURes.
type PDUSessionResourceFailedToSetupListSURes []PDUSessionResourceFailedToSetupItemSURes

// PDUSessionResourceSetupUnsuccessfulTransfer ::= SEQUENCE { cause,
// criticalityDiagnostics OPTIONAL, iE-Extensions OPTIONAL } (extensible) —
// TS 38.413 §9.3.4.16. Carried in the Failed to Setup lists of both the setup
// response and the initial context setup response.
type PDUSessionResourceSetupUnsuccessfulTransfer struct {
	_                      [0]struct{} `per:"extseq"`
	Cause                  Cause
	CriticalityDiagnostics *CriticalityDiagnostics `per:",optional"`
	_                      ieExtensions            `per:",skip"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *PDUSessionResourceSetupUnsuccessfulTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParsePDUSessionResourceSetupUnsuccessfulTransfer decodes the transfer an
// NG-RAN node returns for a session it could not set up.
func ParsePDUSessionResourceSetupUnsuccessfulTransfer(b TransferContainer) (*PDUSessionResourceSetupUnsuccessfulTransfer, error) {
	var t PDUSessionResourceSetupUnsuccessfulTransfer

	if err := t.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, err
	}

	return &t, nil
}

// TS 38.413 §9.2.1.1.
type PDUSessionResourceSetupRequest struct {
	AMFUENGAPID AMFUENGAPID
	RANUENGAPID RANUENGAPID
	// Optional but reject criticality, unlike the ignore-criticality NAS-PDU of
	// an INITIAL CONTEXT SETUP REQUEST.
	NASPDU                    *NASPDU
	PDUSessionResourceSetup   PDUSessionResourceSetupListSUReq
	UEAggregateMaximumBitRate *UEAggregateMaximumBitRate

	messageMeta
}

var pDUSessionResourceSetupRequestIEs = []ieSpec[PDUSessionResourceSetupRequest]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *PDUSessionResourceSetupRequest) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *PDUSessionResourceSetupRequest) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: idNASPDU, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceSetupRequest, raw []byte, enc per.Encoding) error {
			var v NASPDU

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.NASPDU = &v

			return nil
		},
		encode: func(m *PDUSessionResourceSetupRequest) (per.Marshaler, bool) {
			if m.NASPDU == nil {
				return nil, false
			}

			return m.NASPDU, true
		},
	},
	{
		id: idPDUSessionResourceSetupListSUReq, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceSetup)
		},
		encode: func(m *PDUSessionResourceSetupRequest) (per.Marshaler, bool) {
			if m.PDUSessionResourceSetup == nil {
				return nil, false
			}

			return m.PDUSessionResourceSetup, true
		},
	},
	{
		id: idUEAggregateMaximumBitRate, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceSetupRequest, raw []byte, enc per.Encoding) error {
			var v UEAggregateMaximumBitRate

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.UEAggregateMaximumBitRate = &v

			return nil
		},
		encode: func(m *PDUSessionResourceSetupRequest) (per.Marshaler, bool) {
			if m.UEAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.UEAggregateMaximumBitRate, true
		},
	},
}

func (m *PDUSessionResourceSetupRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPDUSessionResourceSetup, pDUSessionResourceSetupRequestIEs, m)
}

func (m *PDUSessionResourceSetupRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcPDUSessionResourceSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePDUSessionResourceSetupRequest(value []byte) (*PDUSessionResourceSetupRequest, error) {
	return parseMessageBody[PDUSessionResourceSetupRequest](ProcPDUSessionResourceSetup, TriggeringInitiatingMessage, pDUSessionResourceSetupRequestIEs, value)
}

// TS 38.413 §9.2.1.2.
type PDUSessionResourceSetupResponse struct {
	AMFUENGAPID              *AMFUENGAPID
	RANUENGAPID              *RANUENGAPID
	PDUSessionResourceSetup  PDUSessionResourceSetupListSURes
	PDUSessionResourceFailed PDUSessionResourceFailedToSetupListSURes
	CriticalityDiagnostics   *CriticalityDiagnostics
	UserLocationInformation  *UserLocationInformation

	messageMeta
}

var pDUSessionResourceSetupResponseIEs = []ieSpec[PDUSessionResourceSetupResponse]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceSetupResponse, raw []byte, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		encode: func(m *PDUSessionResourceSetupResponse) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceSetupResponse, raw []byte, enc per.Encoding) error {
			var v RANUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		encode: func(m *PDUSessionResourceSetupResponse) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		id: idPDUSessionResourceSetupListSURes, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceSetupResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceSetup)
		},
		encode: func(m *PDUSessionResourceSetupResponse) (per.Marshaler, bool) {
			if m.PDUSessionResourceSetup == nil {
				return nil, false
			}

			return m.PDUSessionResourceSetup, true
		},
	},
	{
		id: idPDUSessionResourceFailedToSetupListSURes, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceSetupResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceFailed)
		},
		encode: func(m *PDUSessionResourceSetupResponse) (per.Marshaler, bool) {
			if m.PDUSessionResourceFailed == nil {
				return nil, false
			}

			return m.PDUSessionResourceFailed, true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceSetupResponse, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *PDUSessionResourceSetupResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		id: idUserLocationInformation, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceSetupResponse, raw []byte, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := perIEDecode(raw, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		encode: func(m *PDUSessionResourceSetupResponse) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
}

func (m *PDUSessionResourceSetupResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPDUSessionResourceSetup, pDUSessionResourceSetupResponseIEs, m)
}

func (m *PDUSessionResourceSetupResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcPDUSessionResourceSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePDUSessionResourceSetupResponse(value []byte) (*PDUSessionResourceSetupResponse, error) {
	return parseMessageBody[PDUSessionResourceSetupResponse](ProcPDUSessionResourceSetup, TriggeringSuccessfulOutcome, pDUSessionResourceSetupResponseIEs, value)
}

// DataForwardingNotPossible ::= ENUMERATED { data-forwarding-not-possible,
// ... }.
type DataForwardingNotPossible uint8

const (
	DataForwardingNotPossibleTrue DataForwardingNotPossible = iota

	dataForwardingNotPossibleRootCount = 1
)

func (d DataForwardingNotPossible) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, dataForwardingNotPossibleRootCount, int64(d), "DataForwardingNotPossible")
}

func (d *DataForwardingNotPossible) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, dataForwardingNotPossibleRootCount, "DataForwardingNotPossible")
	if err != nil {
		return err
	}

	*d = DataForwardingNotPossible(idx)

	return nil
}

// PDUSessionResourceSetupRequestTransfer ::= SEQUENCE { protocolIEs
// ProtocolIE-Container } (extensible) — TS 38.413 §9.3.4.1. Unlike the
// unsuccessful transfers this one is an IE container, not a plain SEQUENCE, so
// it carries per-IE criticality and reuses the message-body engine. The SMF
// builds it; the NG-RAN node consumes it.
type PDUSessionResourceSetupRequestTransfer struct {
	PDUSessionAggregateMaximumBitRate *PDUSessionAggregateMaximumBitRate
	ULNGUUPTNLInformation             UPTransportLayerInformation
	AdditionalULNGUUPTNLInformation   UPTransportLayerInformationList
	DataForwardingNotPossible         *DataForwardingNotPossible
	PDUSessionType                    PDUSessionType
	SecurityIndication                *SecurityIndication
	NetworkInstance                   *NetworkInstance
	QosFlowSetupRequest               QosFlowSetupRequestList

	messageMeta
}

var pDUSessionResourceSetupRequestTransferIEs = []ieSpec[PDUSessionResourceSetupRequestTransfer]{
	{
		id: idPDUSessionAggregateMaximumBitRate, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceSetupRequestTransfer, raw []byte, enc per.Encoding) error {
			var v PDUSessionAggregateMaximumBitRate

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.PDUSessionAggregateMaximumBitRate = &v

			return nil
		},
		encode: func(m *PDUSessionResourceSetupRequestTransfer) (per.Marshaler, bool) {
			if m.PDUSessionAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.PDUSessionAggregateMaximumBitRate, true
		},
	},
	{
		id: idULNGUUPTNLInformation, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceSetupRequestTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ULNGUUPTNLInformation)
		},
		encode: func(m *PDUSessionResourceSetupRequestTransfer) (per.Marshaler, bool) {
			return &m.ULNGUUPTNLInformation, true
		},
	},
	{
		id: idAdditionalULNGUUPTNLInformation, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceSetupRequestTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AdditionalULNGUUPTNLInformation)
		},
		encode: func(m *PDUSessionResourceSetupRequestTransfer) (per.Marshaler, bool) {
			if m.AdditionalULNGUUPTNLInformation == nil {
				return nil, false
			}

			return m.AdditionalULNGUUPTNLInformation, true
		},
	},
	{
		id: idDataForwardingNotPossible, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceSetupRequestTransfer, raw []byte, enc per.Encoding) error {
			var d DataForwardingNotPossible

			if err := perIEDecode(raw, &d); err != nil {
				return err
			}

			m.DataForwardingNotPossible = &d

			return nil
		},
		encode: func(m *PDUSessionResourceSetupRequestTransfer) (per.Marshaler, bool) {
			if m.DataForwardingNotPossible == nil {
				return nil, false
			}

			return m.DataForwardingNotPossible, true
		},
	},
	{
		id: idPDUSessionType, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceSetupRequestTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionType)
		},
		encode: func(m *PDUSessionResourceSetupRequestTransfer) (per.Marshaler, bool) {
			return &m.PDUSessionType, true
		},
	},
	{
		id: idSecurityIndication, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceSetupRequestTransfer, raw []byte, enc per.Encoding) error {
			var v SecurityIndication

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.SecurityIndication = &v

			return nil
		},
		encode: func(m *PDUSessionResourceSetupRequestTransfer) (per.Marshaler, bool) {
			if m.SecurityIndication == nil {
				return nil, false
			}

			return m.SecurityIndication, true
		},
	},
	{
		id: idNetworkInstance, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceSetupRequestTransfer, raw []byte, enc per.Encoding) error {
			var v NetworkInstance

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.NetworkInstance = &v

			return nil
		},
		encode: func(m *PDUSessionResourceSetupRequestTransfer) (per.Marshaler, bool) {
			if m.NetworkInstance == nil {
				return nil, false
			}

			return m.NetworkInstance, true
		},
	},
	{
		id: idQosFlowSetupRequestList, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceSetupRequestTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.QosFlowSetupRequest)
		},
		encode: func(m *PDUSessionResourceSetupRequestTransfer) (per.Marshaler, bool) {
			if m.QosFlowSetupRequest == nil {
				return nil, false
			}

			return m.QosFlowSetupRequest, true
		},
	},
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *PDUSessionResourceSetupRequestTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := encodeMessageBody(w, per.Aligned, ProcPDUSessionResourceSetup, pDUSessionResourceSetupRequestTransferIEs, t); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParsePDUSessionResourceSetupRequestTransfer decodes the transfer the SMF
// sends for a session the NG-RAN node is to set up. Errors carry the enclosing
// procedure, which is what the AMF reports in Criticality Diagnostics.
func ParsePDUSessionResourceSetupRequestTransfer(b TransferContainer) (*PDUSessionResourceSetupRequestTransfer, error) {
	return parseMessageBody[PDUSessionResourceSetupRequestTransfer](ProcPDUSessionResourceSetup, TriggeringInitiatingMessage, pDUSessionResourceSetupRequestTransferIEs, b)
}

// PDUSessionResourceSetupResponseTransfer ::= SEQUENCE {
// dLQosFlowPerTNLInformation, additionalDLQosFlowPerTNLInformation OPTIONAL,
// securityResult OPTIONAL, qosFlowFailedToSetupList OPTIONAL, iE-Extensions
// OPTIONAL } (extensible) — TS 38.413 §9.3.4.2. A plain SEQUENCE, unlike the
// request transfer. The NG-RAN node builds it; the SMF consumes it.
type PDUSessionResourceSetupResponseTransfer struct {
	_                                    [0]struct{} `per:"extseq"`
	DLQosFlowPerTNLInformation           QosFlowPerTNLInformation
	AdditionalDLQosFlowPerTNLInformation QosFlowPerTNLInformationList `per:",optional"`
	SecurityResult                       *SecurityResult              `per:",optional"`
	QosFlowFailedToSetup                 QosFlowListWithCause         `per:",optional"`
	_                                    ieExtensions                 `per:",skip"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *PDUSessionResourceSetupResponseTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParsePDUSessionResourceSetupResponseTransfer decodes the transfer an NG-RAN
// node returns for a session it set up.
func ParsePDUSessionResourceSetupResponseTransfer(b TransferContainer) (*PDUSessionResourceSetupResponseTransfer, error) {
	var t PDUSessionResourceSetupResponseTransfer

	if err := t.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, err
	}

	return &t, nil
}
