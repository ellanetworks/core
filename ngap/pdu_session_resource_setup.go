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
	PDUSessionID                          PDUSessionID
	NASPDU                                *NASPDU `per:",optional"`
	SNSSAI                                SNSSAI
	Transfer                              TransferContainer
	PduSessionExpectedUEActivityBehaviour *ExpectedUEActivityBehaviour `per:",extie:IDPduSessionExpectedUEActivityBehaviour,crit:ignore"`
	iEExtensions                          extensionMeta                `per:",exttable"`
	extensions                            per.ExtensionAdditions       `per:",extraw"`
}

// PDUSessionResourceSetupListSUReq ::= SEQUENCE (SIZE(1..maxnoofPDUSessions))
// OF PDUSessionResourceSetupItemSUReq.
type PDUSessionResourceSetupListSUReq []PDUSessionResourceSetupItemSUReq

// PDUSessionResourceSetupItemSURes ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceSetupResponseTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceSetupItemSURes struct {
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

// PDUSessionResourceSetupListSURes ::= SEQUENCE (SIZE(1..maxnoofPDUSessions))
// OF PDUSessionResourceSetupItemSURes.
type PDUSessionResourceSetupListSURes []PDUSessionResourceSetupItemSURes

// PDUSessionResourceFailedToSetupItemSURes ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceSetupUnsuccessfulTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceFailedToSetupItemSURes struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta `per:",exttable"`
}

// PDUSessionResourceFailedToSetupListSURes ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceFailedToSetupItemSURes.
type PDUSessionResourceFailedToSetupListSURes []PDUSessionResourceFailedToSetupItemSURes

// PDUSessionResourceSetupUnsuccessfulTransfer ::= SEQUENCE { cause,
// criticalityDiagnostics OPTIONAL, iE-Extensions OPTIONAL } (extensible) —
// TS 38.413 §9.3.4.16. Carried in the Failed to Setup lists of both the setup
// response and the initial context setup response.
type PDUSessionResourceSetupUnsuccessfulTransfer struct {
	extensions             per.ExtensionAdditions `per:",extraw"`
	Cause                  Cause
	CriticalityDiagnostics *CriticalityDiagnostics `per:",optional"`
	iEExtensions           extensionMeta           `per:",exttable"`
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
	return parseTransfer[PDUSessionResourceSetupUnsuccessfulTransfer](b)
}

func DecodePDUSessionResourceSetupUnsuccessfulTransfer(b TransferContainer) (*PDUSessionResourceSetupUnsuccessfulTransfer, error) {
	return decodeTransfer[PDUSessionResourceSetupUnsuccessfulTransfer](b)
}

func (t *PDUSessionResourceSetupUnsuccessfulTransfer) Diagnostics() Diagnostics {
	return diagnosticsOf(t)
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
	RANPagingPriority         *RANPagingPriority
	UESliceMaximumBitRateList UESliceMaximumBitRateList

	messageMeta
}

var pDUSessionResourceSetupRequestIEs = []ieSpec[PDUSessionResourceSetupRequest]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFUENGAPID)
		},
		Encode: func(m *PDUSessionResourceSetupRequest) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *PDUSessionResourceSetupRequest) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	optionalExtension(IDRANPagingPriority, CriticalityIgnore, func(m *PDUSessionResourceSetupRequest) **RANPagingPriority { return &m.RANPagingPriority }),
	{
		ID: IDNASPDU, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceSetupRequest, r *per.Reader, enc per.Encoding) error {
			var v NASPDU

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.NASPDU = &v

			return nil
		},
		Encode: func(m *PDUSessionResourceSetupRequest) (per.Marshaler, bool) {
			if m.NASPDU == nil {
				return nil, false
			}

			return m.NASPDU, true
		},
	},
	{
		ID: IDPDUSessionResourceSetupListSUReq, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceSetup)
		},
		Encode: func(m *PDUSessionResourceSetupRequest) (per.Marshaler, bool) {
			if m.PDUSessionResourceSetup == nil {
				return nil, false
			}

			return m.PDUSessionResourceSetup, true
		},
	},
	{
		ID: IDUEAggregateMaximumBitRate, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceSetupRequest, r *per.Reader, enc per.Encoding) error {
			var v UEAggregateMaximumBitRate

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.UEAggregateMaximumBitRate = &v

			return nil
		},
		Encode: func(m *PDUSessionResourceSetupRequest) (per.Marshaler, bool) {
			if m.UEAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.UEAggregateMaximumBitRate, true
		},
	},
	listExtension(IDUESliceMaximumBitRateList, CriticalityIgnore, func(m *PDUSessionResourceSetupRequest) *UESliceMaximumBitRateList {
		return &m.UESliceMaximumBitRateList
	}),
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

func DecodePDUSessionResourceSetupRequest(value []byte) (*PDUSessionResourceSetupRequest, error) {
	return decodeMessageBody[PDUSessionResourceSetupRequest](ProcPDUSessionResourceSetup, pDUSessionResourceSetupRequestIEs, value)
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
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceSetupResponse, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		Encode: func(m *PDUSessionResourceSetupResponse) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceSetupResponse, r *per.Reader, enc per.Encoding) error {
			var v RANUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		Encode: func(m *PDUSessionResourceSetupResponse) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		ID: IDPDUSessionResourceSetupListSURes, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceSetupResponse, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceSetup)
		},
		Encode: func(m *PDUSessionResourceSetupResponse) (per.Marshaler, bool) {
			if m.PDUSessionResourceSetup == nil {
				return nil, false
			}

			return m.PDUSessionResourceSetup, true
		},
	},
	{
		ID: IDPDUSessionResourceFailedToSetupListSURes, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceSetupResponse, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceFailed)
		},
		Encode: func(m *PDUSessionResourceSetupResponse) (per.Marshaler, bool) {
			if m.PDUSessionResourceFailed == nil {
				return nil, false
			}

			return m.PDUSessionResourceFailed, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceSetupResponse, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *PDUSessionResourceSetupResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		ID: IDUserLocationInformation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceSetupResponse, r *per.Reader, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := decodeIE(r, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		Encode: func(m *PDUSessionResourceSetupResponse) (per.Marshaler, bool) {
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

func DecodePDUSessionResourceSetupResponse(value []byte) (*PDUSessionResourceSetupResponse, error) {
	return decodeMessageBody[PDUSessionResourceSetupResponse](ProcPDUSessionResourceSetup, pDUSessionResourceSetupResponseIEs, value)
}

// DataForwardingNotPossible ::= ENUMERATED { data-forwarding-not-possible,
// ... }.
type DataForwardingNotPossible uint8

const (
	DataForwardingNotPossibleTrue DataForwardingNotPossible = iota
)

// PDUSessionResourceSetupRequestTransfer ::= SEQUENCE { protocolIEs
// ProtocolIE-Container } (extensible) — TS 38.413 §9.3.4.1. Unlike the
// unsuccessful transfers this one is an IE container, not a plain SEQUENCE, so
// it carries per-IE criticality and reuses the message-body engine. The SMF
// builds it; the NG-RAN node consumes it.
type PDUSessionResourceSetupRequestTransfer struct {
	PDUSessionAggregateMaximumBitRate        *PDUSessionAggregateMaximumBitRate
	ULNGUUPTNLInformation                    UPTransportLayerInformation
	AdditionalULNGUUPTNLInformation          UPTransportLayerInformationList
	DataForwardingNotPossible                *DataForwardingNotPossible
	PDUSessionType                           PDUSessionType
	SecurityIndication                       *SecurityIndication
	NetworkInstance                          *NetworkInstance
	QosFlowSetupRequest                      QosFlowSetupRequestList
	DirectForwardingPathAvailability         *DirectForwardingPathAvailability
	RedundantULNGUUPTNLInformation           *UPTransportLayerInformation
	AdditionalRedundantULNGUUPTNLInformation UPTransportLayerInformationList
	CommonNetworkInstance                    CommonNetworkInstance
	RedundantCommonNetworkInstance           CommonNetworkInstance
	RedundantPDUSessionInformation           *RedundantPDUSessionInformation
	MBSSessionSetupRequestList               MBSSessionSetupRequestList
	DownlinkTLContainer                      TLContainer

	messageMeta
}

var pDUSessionResourceSetupRequestTransferIEs = []ieSpec[PDUSessionResourceSetupRequestTransfer]{
	{
		ID: IDPDUSessionAggregateMaximumBitRate, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceSetupRequestTransfer, r *per.Reader, enc per.Encoding) error {
			var v PDUSessionAggregateMaximumBitRate

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.PDUSessionAggregateMaximumBitRate = &v

			return nil
		},
		Encode: func(m *PDUSessionResourceSetupRequestTransfer) (per.Marshaler, bool) {
			if m.PDUSessionAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.PDUSessionAggregateMaximumBitRate, true
		},
	},
	{
		ID: IDULNGUUPTNLInformation, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceSetupRequestTransfer, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ULNGUUPTNLInformation)
		},
		Encode: func(m *PDUSessionResourceSetupRequestTransfer) (per.Marshaler, bool) {
			return &m.ULNGUUPTNLInformation, true
		},
	},
	{
		ID: IDAdditionalULNGUUPTNLInformation, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceSetupRequestTransfer, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AdditionalULNGUUPTNLInformation)
		},
		Encode: func(m *PDUSessionResourceSetupRequestTransfer) (per.Marshaler, bool) {
			if m.AdditionalULNGUUPTNLInformation == nil {
				return nil, false
			}

			return m.AdditionalULNGUUPTNLInformation, true
		},
	},
	{
		ID: IDDataForwardingNotPossible, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceSetupRequestTransfer, r *per.Reader, enc per.Encoding) error {
			var d DataForwardingNotPossible

			if err := decodeIE(r, &d); err != nil {
				return err
			}

			m.DataForwardingNotPossible = &d

			return nil
		},
		Encode: func(m *PDUSessionResourceSetupRequestTransfer) (per.Marshaler, bool) {
			if m.DataForwardingNotPossible == nil {
				return nil, false
			}

			return m.DataForwardingNotPossible, true
		},
	},
	{
		ID: IDPDUSessionType, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceSetupRequestTransfer, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionType)
		},
		Encode: func(m *PDUSessionResourceSetupRequestTransfer) (per.Marshaler, bool) {
			return &m.PDUSessionType, true
		},
	},
	{
		ID: IDSecurityIndication, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceSetupRequestTransfer, r *per.Reader, enc per.Encoding) error {
			var v SecurityIndication

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.SecurityIndication = &v

			return nil
		},
		Encode: func(m *PDUSessionResourceSetupRequestTransfer) (per.Marshaler, bool) {
			if m.SecurityIndication == nil {
				return nil, false
			}

			return m.SecurityIndication, true
		},
	},
	{
		ID: IDNetworkInstance, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceSetupRequestTransfer, r *per.Reader, enc per.Encoding) error {
			var v NetworkInstance

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.NetworkInstance = &v

			return nil
		},
		Encode: func(m *PDUSessionResourceSetupRequestTransfer) (per.Marshaler, bool) {
			if m.NetworkInstance == nil {
				return nil, false
			}

			return m.NetworkInstance, true
		},
	},
	{
		ID: IDQosFlowSetupRequestList, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceSetupRequestTransfer, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.QosFlowSetupRequest)
		},
		Encode: func(m *PDUSessionResourceSetupRequestTransfer) (per.Marshaler, bool) {
			if m.QosFlowSetupRequest == nil {
				return nil, false
			}

			return m.QosFlowSetupRequest, true
		},
	},
	listExtension(IDCommonNetworkInstance, CriticalityIgnore, func(m *PDUSessionResourceSetupRequestTransfer) *CommonNetworkInstance {
		return &m.CommonNetworkInstance
	}),
	{
		ID: IDDirectForwardingPathAvailability, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceSetupRequestTransfer, r *per.Reader, enc per.Encoding) error {
			var v DirectForwardingPathAvailability

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.DirectForwardingPathAvailability = &v

			return nil
		},
		Encode: func(m *PDUSessionResourceSetupRequestTransfer) (per.Marshaler, bool) {
			if m.DirectForwardingPathAvailability == nil {
				return nil, false
			}

			return m.DirectForwardingPathAvailability, true
		},
	},
	optionalExtension(IDRedundantULNGUUPTNLInformation, CriticalityIgnore, func(m *PDUSessionResourceSetupRequestTransfer) **UPTransportLayerInformation {
		return &m.RedundantULNGUUPTNLInformation
	}),
	listExtension(IDAdditionalRedundantULNGUUPTNLInformation, CriticalityIgnore, func(m *PDUSessionResourceSetupRequestTransfer) *UPTransportLayerInformationList {
		return &m.AdditionalRedundantULNGUUPTNLInformation
	}),
	listExtension(IDRedundantCommonNetworkInstance, CriticalityIgnore, func(m *PDUSessionResourceSetupRequestTransfer) *CommonNetworkInstance {
		return &m.RedundantCommonNetworkInstance
	}),
	optionalExtension(IDRedundantPDUSessionInformation, CriticalityIgnore, func(m *PDUSessionResourceSetupRequestTransfer) **RedundantPDUSessionInformation {
		return &m.RedundantPDUSessionInformation
	}),
	listExtension(IDMBSSessionSetupRequestList, CriticalityIgnore, func(m *PDUSessionResourceSetupRequestTransfer) *MBSSessionSetupRequestList {
		return &m.MBSSessionSetupRequestList
	}),
	listExtension(IDDownlinkTLContainer, CriticalityIgnore, func(m *PDUSessionResourceSetupRequestTransfer) *TLContainer { return &m.DownlinkTLContainer }),
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

func DecodePDUSessionResourceSetupRequestTransfer(b TransferContainer) (*PDUSessionResourceSetupRequestTransfer, error) {
	return decodeMessageBody[PDUSessionResourceSetupRequestTransfer](ProcPDUSessionResourceSetup, pDUSessionResourceSetupRequestTransferIEs, b)
}

// PDUSessionResourceSetupResponseTransfer ::= SEQUENCE {
// dLQosFlowPerTNLInformation, additionalDLQosFlowPerTNLInformation OPTIONAL,
// securityResult OPTIONAL, qosFlowFailedToSetupList OPTIONAL, iE-Extensions
// OPTIONAL } (extensible) — TS 38.413 §9.3.4.2. A plain SEQUENCE, unlike the
// request transfer. The NG-RAN node builds it; the SMF consumes it.
type PDUSessionResourceSetupResponseTransfer struct {
	DLQosFlowPerTNLInformation                       QosFlowPerTNLInformation
	AdditionalDLQosFlowPerTNLInformation             QosFlowPerTNLInformationList                     `per:",optional"`
	SecurityResult                                   *SecurityResult                                  `per:",optional"`
	QosFlowFailedToSetup                             QosFlowListWithCause                             `per:",optional"`
	RedundantDLQosFlowPerTNLInformation              *QosFlowPerTNLInformation                        `per:",extie:IDRedundantDLQosFlowPerTNLInformation,crit:ignore"`
	AdditionalRedundantDLQosFlowPerTNLInformation    QosFlowPerTNLInformationList                     `per:",extie:IDAdditionalRedundantDLQosFlowPerTNLInformation,crit:ignore"`
	UsedRSNInformation                               *RedundantPDUSessionInformation                  `per:",extie:IDUsedRSNInformation,crit:ignore"`
	GlobalRANNodeID                                  *GlobalRANNodeID                                 `per:",extie:IDGlobalRANNodeID,crit:ignore"`
	MBSSupportIndicator                              *MBSSupportIndicator                             `per:",extie:IDMBSSupportIndicator,crit:ignore"`
	MBSSessionSetupResponseList                      MBSSessionSetupResponseList                      `per:",extie:IDMBSSessionSetupResponseList,crit:ignore"`
	MBSSessionFailedtoSetupList                      MBSSessionFailedtoSetupList                      `per:",extie:IDMBSSessionFailedtoSetupList,crit:ignore"`
	QoSFlowTSCList                                   QoSFlowTSCList                                   `per:",extie:IDQoSFlowTSCList,crit:ignore"`
	UplinkTLContainer                                *TLContainer                                     `per:",extie:IDUplinkTLContainer,crit:ignore"`
	ECNMarkingorCongestionInformationReportingStatus ECNMarkingorCongestionInformationReportingStatus `per:",extie:IDECNMarkingorCongestionInformationReportingStatus,crit:ignore"`
	PDUSetbasedHandlingIndicator                     *PDUSetbasedHandlingIndicator                    `per:",extie:IDPDUSetbasedHandlingIndicator,crit:ignore"`
	iEExtensions                                     extensionMeta                                    `per:",exttable"`
	extensions                                       per.ExtensionAdditions                           `per:",extraw"`
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
	return parseTransfer[PDUSessionResourceSetupResponseTransfer](b)
}

func DecodePDUSessionResourceSetupResponseTransfer(b TransferContainer) (*PDUSessionResourceSetupResponseTransfer, error) {
	return decodeTransfer[PDUSessionResourceSetupResponseTransfer](b)
}

func (t *PDUSessionResourceSetupResponseTransfer) Diagnostics() Diagnostics { return diagnosticsOf(t) }
