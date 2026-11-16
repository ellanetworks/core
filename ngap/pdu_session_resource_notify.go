// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// PDUSessionResourceNotifyItem ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceNotifyTransfer, iE-Extensions OPTIONAL } (extensible).
type PDUSessionResourceNotifyItem struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta `per:",exttable"`
}

// PDUSessionResourceNotifyList ::= SEQUENCE (SIZE(1..maxnoofPDUSessions)) OF
// PDUSessionResourceNotifyItem.
type PDUSessionResourceNotifyList []PDUSessionResourceNotifyItem

// PDUSessionResourceReleasedItemNot ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceNotifyReleasedTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceReleasedItemNot struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta `per:",exttable"`
}

// PDUSessionResourceReleasedListNot ::= SEQUENCE (SIZE(1..maxnoofPDUSessions))
// OF PDUSessionResourceReleasedItemNot.
type PDUSessionResourceReleasedListNot []PDUSessionResourceReleasedItemNot

// TS 38.413 §9.2.1.7. The NG-RAN node reports that a QoS flow's fulfilled
// status changed, or that it released a session on its own initiative
// (§8.2.4). TS 36.413 defines no counterpart: an eNB reports the equivalent
// through E-RAB Modification Indication and E-RAB Release Indication.
type PDUSessionResourceNotify struct {
	AMFUENGAPID                AMFUENGAPID
	RANUENGAPID                RANUENGAPID
	PDUSessionResourceNotify   PDUSessionResourceNotifyList
	PDUSessionResourceReleased PDUSessionResourceReleasedListNot
	UserLocationInformation    *UserLocationInformation

	messageMeta
}

var pDUSessionResourceNotifyIEs = []ieSpec[PDUSessionResourceNotify]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceNotify, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFUENGAPID)
		},
		Encode: func(m *PDUSessionResourceNotify) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceNotify, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *PDUSessionResourceNotify) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		ID: IDPDUSessionResourceNotifyList, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceNotify, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceNotify)
		},
		Encode: func(m *PDUSessionResourceNotify) (per.Marshaler, bool) {
			if m.PDUSessionResourceNotify == nil {
				return nil, false
			}

			return m.PDUSessionResourceNotify, true
		},
	},
	{
		ID: IDPDUSessionResourceReleasedListNot, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceNotify, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceReleased)
		},
		Encode: func(m *PDUSessionResourceNotify) (per.Marshaler, bool) {
			if m.PDUSessionResourceReleased == nil {
				return nil, false
			}

			return m.PDUSessionResourceReleased, true
		},
	},
	{
		ID: IDUserLocationInformation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceNotify, r *per.Reader, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := decodeIE(r, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		Encode: func(m *PDUSessionResourceNotify) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
}

func (m *PDUSessionResourceNotify) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPDUSessionResourceNotify, pDUSessionResourceNotifyIEs, m)
}

func (m *PDUSessionResourceNotify) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcPDUSessionResourceNotify,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParsePDUSessionResourceNotify(value []byte) (*PDUSessionResourceNotify, error) {
	return parseMessageBody[PDUSessionResourceNotify](ProcPDUSessionResourceNotify, TriggeringInitiatingMessage, pDUSessionResourceNotifyIEs, value)
}

func DecodePDUSessionResourceNotify(value []byte) (*PDUSessionResourceNotify, error) {
	return decodeMessageBody[PDUSessionResourceNotify](ProcPDUSessionResourceNotify, pDUSessionResourceNotifyIEs, value)
}

// NotificationCause ::= ENUMERATED { fulfilled, not-fulfilled, ... } —
// TS 38.413 §9.3.4.5.
type NotificationCause uint8

const (
	NotificationCauseFulfilled      NotificationCause = 0
	NotificationCauseNotFulfilled   NotificationCause = 1
	NotificationCauseNotFulfilledDL NotificationCause = 2
	NotificationCauseNotFulfilledUL NotificationCause = 3
)

// QosFlowNotifyItem ::= SEQUENCE { qosFlowIdentifier, notificationCause,
// iE-Extensions OPTIONAL } (extensible).
type QosFlowNotifyItem struct {
	extensions                        per.ExtensionAdditions `per:",extraw"`
	QosFlowIdentifier                 QosFlowIdentifier
	NotificationCause                 NotificationCause
	CurrentQoSParaSetIndex            *AlternativeQoSParaSetNotifyIndex  `per:",extie:IDCurrentQoSParaSetIndex,crit:ignore"`
	TSCTrafficCharacteristicsFeedback *TSCTrafficCharacteristicsFeedback `per:",extie:IDTSCTrafficCharacteristicsFeedback,crit:ignore"`
	iEExtensions                      extensionMeta                      `per:",exttable"`
}

// QosFlowNotifyList ::= SEQUENCE (SIZE(1..maxnoofQosFlows)) OF QosFlowNotifyItem.
type QosFlowNotifyList []QosFlowNotifyItem

// PDUSessionResourceNotifyTransfer ::= SEQUENCE { qosFlowNotifyList OPTIONAL,
// qosFlowReleasedList OPTIONAL, iE-Extensions OPTIONAL } (extensible) —
// TS 38.413 §9.3.4.5.
type PDUSessionResourceNotifyTransfer struct {
	extensions                   per.ExtensionAdditions        `per:",extraw"`
	QosFlowNotify                QosFlowNotifyList             `per:",optional"`
	QosFlowReleased              QosFlowListWithCause          `per:",optional"`
	SecondaryRATUsageInformation *SecondaryRATUsageInformation `per:",extie:IDSecondaryRATUsageInformation,crit:ignore"`
	QosFlowFeedbackList          QosFlowFeedbackList           `per:",extie:IDQosFlowFeedbackList,crit:ignore"`
	iEExtensions                 extensionMeta                 `per:",exttable"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *PDUSessionResourceNotifyTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParsePDUSessionResourceNotifyTransfer decodes the QoS flow notifications an
// NG-RAN node reports for a PDU session.
func ParsePDUSessionResourceNotifyTransfer(b TransferContainer) (*PDUSessionResourceNotifyTransfer, error) {
	return parseTransfer[PDUSessionResourceNotifyTransfer](b)
}

func DecodePDUSessionResourceNotifyTransfer(b TransferContainer) (*PDUSessionResourceNotifyTransfer, error) {
	return decodeTransfer[PDUSessionResourceNotifyTransfer](b)
}

func (t *PDUSessionResourceNotifyTransfer) Diagnostics() Diagnostics { return diagnosticsOf(t) }
