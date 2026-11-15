// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// PDUSessionResourceModifyItemModInd ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceModifyIndicationTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceModifyItemModInd struct {
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

// PDUSessionResourceModifyListModInd ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceModifyItemModInd.
type PDUSessionResourceModifyListModInd []PDUSessionResourceModifyItemModInd

// PDUSessionResourceModifyItemModCfm ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceModifyConfirmTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceModifyItemModCfm struct {
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

// PDUSessionResourceModifyListModCfm ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceModifyItemModCfm.
type PDUSessionResourceModifyListModCfm []PDUSessionResourceModifyItemModCfm

// PDUSessionResourceFailedToModifyItemModCfm ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceModifyIndicationUnsuccessfulTransfer, iE-Extensions
// OPTIONAL } (extensible).
type PDUSessionResourceFailedToModifyItemModCfm struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta `per:",exttable"`
}

// PDUSessionResourceFailedToModifyListModCfm ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceFailedToModifyItemModCfm.
type PDUSessionResourceFailedToModifyListModCfm []PDUSessionResourceFailedToModifyItemModCfm

// PDUSessionResourceModifyIndicationUnsuccessfulTransfer ::= SEQUENCE { cause,
// iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.4.22. The first of the
// §9.3.4 transfers modelled: the AMF builds this one itself when it cannot hand
// a session to the SMF, so it cannot stay opaque.
type PDUSessionResourceModifyIndicationUnsuccessfulTransfer struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	Cause        Cause
	iEExtensions extensionMeta `per:",exttable"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *PDUSessionResourceModifyIndicationUnsuccessfulTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParsePDUSessionResourceModifyIndicationUnsuccessfulTransfer decodes the
// transfer the AMF returns for a session it could not hand to the SMF.
func ParsePDUSessionResourceModifyIndicationUnsuccessfulTransfer(b TransferContainer) (*PDUSessionResourceModifyIndicationUnsuccessfulTransfer, error) {
	return parseTransfer[PDUSessionResourceModifyIndicationUnsuccessfulTransfer](b)
}

func DecodePDUSessionResourceModifyIndicationUnsuccessfulTransfer(b TransferContainer) (*PDUSessionResourceModifyIndicationUnsuccessfulTransfer, error) {
	return decodeTransfer[PDUSessionResourceModifyIndicationUnsuccessfulTransfer](b)
}

func (t *PDUSessionResourceModifyIndicationUnsuccessfulTransfer) Diagnostics() Diagnostics {
	return diagnosticsOf(t)
}

// TS 38.413 §9.2.1.8.
type PDUSessionResourceModifyIndication struct {
	AMFUENGAPID              AMFUENGAPID
	RANUENGAPID              RANUENGAPID
	PDUSessionResourceModify PDUSessionResourceModifyListModInd
	UserLocationInformation  *UserLocationInformation

	messageMeta
}

var pDUSessionResourceModifyIndicationIEs = []ieSpec[PDUSessionResourceModifyIndication]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceModifyIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFUENGAPID)
		},
		Encode: func(m *PDUSessionResourceModifyIndication) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceModifyIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *PDUSessionResourceModifyIndication) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		ID: IDPDUSessionResourceModifyListModInd, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceModifyIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceModify)
		},
		Encode: func(m *PDUSessionResourceModifyIndication) (per.Marshaler, bool) {
			if m.PDUSessionResourceModify == nil {
				return nil, false
			}

			return m.PDUSessionResourceModify, true
		},
	},
	{
		ID: IDUserLocationInformation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceModifyIndication, r *per.Reader, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := decodeIE(r, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		Encode: func(m *PDUSessionResourceModifyIndication) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
}

func (m *PDUSessionResourceModifyIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPDUSessionResourceModifyIndication, pDUSessionResourceModifyIndicationIEs, m)
}

func (m *PDUSessionResourceModifyIndication) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcPDUSessionResourceModifyIndication,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePDUSessionResourceModifyIndication(value []byte) (*PDUSessionResourceModifyIndication, error) {
	return parseMessageBody[PDUSessionResourceModifyIndication](ProcPDUSessionResourceModifyIndication, TriggeringInitiatingMessage, pDUSessionResourceModifyIndicationIEs, value)
}

func DecodePDUSessionResourceModifyIndication(value []byte) (*PDUSessionResourceModifyIndication, error) {
	return decodeMessageBody[PDUSessionResourceModifyIndication](ProcPDUSessionResourceModifyIndication, pDUSessionResourceModifyIndicationIEs, value)
}

// TS 38.413 §9.2.1.9.
type PDUSessionResourceModifyConfirm struct {
	AMFUENGAPID              *AMFUENGAPID
	RANUENGAPID              *RANUENGAPID
	PDUSessionResourceModify PDUSessionResourceModifyListModCfm
	PDUSessionResourceFailed PDUSessionResourceFailedToModifyListModCfm
	CriticalityDiagnostics   *CriticalityDiagnostics

	messageMeta
}

var pDUSessionResourceModifyConfirmIEs = []ieSpec[PDUSessionResourceModifyConfirm]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceModifyConfirm, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		Encode: func(m *PDUSessionResourceModifyConfirm) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceModifyConfirm, r *per.Reader, enc per.Encoding) error {
			var v RANUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		Encode: func(m *PDUSessionResourceModifyConfirm) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		ID: IDPDUSessionResourceModifyListModCfm, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceModifyConfirm, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceModify)
		},
		Encode: func(m *PDUSessionResourceModifyConfirm) (per.Marshaler, bool) {
			if m.PDUSessionResourceModify == nil {
				return nil, false
			}

			return m.PDUSessionResourceModify, true
		},
	},
	{
		ID: IDPDUSessionResourceFailedToModifyListModCfm, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceModifyConfirm, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceFailed)
		},
		Encode: func(m *PDUSessionResourceModifyConfirm) (per.Marshaler, bool) {
			if m.PDUSessionResourceFailed == nil {
				return nil, false
			}

			return m.PDUSessionResourceFailed, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceModifyConfirm, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *PDUSessionResourceModifyConfirm) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *PDUSessionResourceModifyConfirm) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPDUSessionResourceModifyIndication, pDUSessionResourceModifyConfirmIEs, m)
}

func (m *PDUSessionResourceModifyConfirm) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcPDUSessionResourceModifyIndication,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePDUSessionResourceModifyConfirm(value []byte) (*PDUSessionResourceModifyConfirm, error) {
	return parseMessageBody[PDUSessionResourceModifyConfirm](ProcPDUSessionResourceModifyIndication, TriggeringSuccessfulOutcome, pDUSessionResourceModifyConfirmIEs, value)
}

func DecodePDUSessionResourceModifyConfirm(value []byte) (*PDUSessionResourceModifyConfirm, error) {
	return decodeMessageBody[PDUSessionResourceModifyConfirm](ProcPDUSessionResourceModifyIndication, pDUSessionResourceModifyConfirmIEs, value)
}

// PDUSessionResourceModifyIndicationTransfer ::= SEQUENCE {
// dLQosFlowPerTNLInformation, additionalDLQosFlowPerTNLInformation OPTIONAL,
// iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.4.6. The NG-RAN node
// builds it; the SMF consumes it.
type PDUSessionResourceModifyIndicationTransfer struct {
	DLQosFlowPerTNLInformation                       QosFlowPerTNLInformation
	AdditionalDLQosFlowPerTNLInformation             QosFlowPerTNLInformationList                     `per:",optional"`
	SecondaryRATUsageInformation                     *SecondaryRATUsageInformation                    `per:",extie:IDSecondaryRATUsageInformation,crit:ignore"`
	SecurityResult                                   *SecurityResult                                  `per:",extie:IDSecurityResult,crit:ignore"`
	RedundantDLQosFlowPerTNLInformation              *QosFlowPerTNLInformation                        `per:",extie:IDRedundantDLQosFlowPerTNLInformation,crit:ignore"`
	AdditionalRedundantDLQosFlowPerTNLInformation    QosFlowPerTNLInformationList                     `per:",extie:IDAdditionalRedundantDLQosFlowPerTNLInformation,crit:ignore"`
	GlobalRANNodeID                                  *GlobalRANNodeID                                 `per:",extie:IDGlobalRANNodeID,crit:ignore"`
	ECNMarkingorCongestionInformationReportingStatus ECNMarkingorCongestionInformationReportingStatus `per:",extie:IDECNMarkingorCongestionInformationReportingStatus,crit:ignore"`
	iEExtensions                                     extensionMeta                                    `per:",exttable"`
	extensions                                       per.ExtensionAdditions                           `per:",extraw"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *PDUSessionResourceModifyIndicationTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParsePDUSessionResourceModifyIndicationTransfer decodes the transfer an
// NG-RAN node sends when it changes a session's downlink tunnel.
func ParsePDUSessionResourceModifyIndicationTransfer(b TransferContainer) (*PDUSessionResourceModifyIndicationTransfer, error) {
	return parseTransfer[PDUSessionResourceModifyIndicationTransfer](b)
}

func DecodePDUSessionResourceModifyIndicationTransfer(b TransferContainer) (*PDUSessionResourceModifyIndicationTransfer, error) {
	return decodeTransfer[PDUSessionResourceModifyIndicationTransfer](b)
}

func (t *PDUSessionResourceModifyIndicationTransfer) Diagnostics() Diagnostics {
	return diagnosticsOf(t)
}

// PDUSessionResourceModifyConfirmTransfer ::= SEQUENCE {
// qosFlowModifyConfirmList, uLNGU-UP-TNLInformation,
// additionalNG-UUPTNLInformation OPTIONAL, qosFlowFailedToModifyList OPTIONAL,
// iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.4.7. The SMF builds
// it; the NG-RAN node consumes it.
type PDUSessionResourceModifyConfirmTransfer struct {
	QosFlowModifyConfirm                   QosFlowModifyConfirmList
	ULNGUUPTNLInformation                  UPTransportLayerInformation
	AdditionalNGUUPTNLInformation          UPTransportLayerInformationPairList `per:",optional"`
	QosFlowFailedToModify                  QosFlowListWithCause                `per:",optional"`
	RedundantULNGUUPTNLInformation         *UPTransportLayerInformation        `per:",extie:IDRedundantULNGUUPTNLInformation,crit:ignore"`
	AdditionalRedundantNGUUPTNLInformation UPTransportLayerInformationPairList `per:",extie:IDAdditionalRedundantNGUUPTNLInformation,crit:ignore"`
	iEExtensions                           extensionMeta                       `per:",exttable"`
	extensions                             per.ExtensionAdditions              `per:",extraw"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *PDUSessionResourceModifyConfirmTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParsePDUSessionResourceModifyConfirmTransfer decodes the transfer the SMF
// returns confirming a modification.
func ParsePDUSessionResourceModifyConfirmTransfer(b TransferContainer) (*PDUSessionResourceModifyConfirmTransfer, error) {
	return parseTransfer[PDUSessionResourceModifyConfirmTransfer](b)
}

func DecodePDUSessionResourceModifyConfirmTransfer(b TransferContainer) (*PDUSessionResourceModifyConfirmTransfer, error) {
	return decodeTransfer[PDUSessionResourceModifyConfirmTransfer](b)
}

func (t *PDUSessionResourceModifyConfirmTransfer) Diagnostics() Diagnostics { return diagnosticsOf(t) }
