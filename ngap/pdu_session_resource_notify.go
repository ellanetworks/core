// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// PDUSessionResourceNotifyItem ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceNotifyTransfer, iE-Extensions OPTIONAL } (extensible).
type PDUSessionResourceNotifyItem struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceNotifyList ::= SEQUENCE (SIZE(1..maxnoofPDUSessions)) OF
// PDUSessionResourceNotifyItem.
type PDUSessionResourceNotifyList []PDUSessionResourceNotifyItem

// PDUSessionResourceReleasedItemNot ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceNotifyReleasedTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceReleasedItemNot struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
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
		id: IDAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceNotify, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *PDUSessionResourceNotify) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: IDRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceNotify, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *PDUSessionResourceNotify) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: IDPDUSessionResourceNotifyList, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceNotify, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceNotify)
		},
		encode: func(m *PDUSessionResourceNotify) (per.Marshaler, bool) {
			if m.PDUSessionResourceNotify == nil {
				return nil, false
			}

			return m.PDUSessionResourceNotify, true
		},
	},
	{
		id: IDPDUSessionResourceReleasedListNot, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceNotify, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceReleased)
		},
		encode: func(m *PDUSessionResourceNotify) (per.Marshaler, bool) {
			if m.PDUSessionResourceReleased == nil {
				return nil, false
			}

			return m.PDUSessionResourceReleased, true
		},
	},
	{
		id: IDUserLocationInformation, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceNotify, raw []byte, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := perIEDecode(raw, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		encode: func(m *PDUSessionResourceNotify) (per.Marshaler, bool) {
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

// NotificationCause ::= ENUMERATED { fulfilled, not-fulfilled, ... } —
// TS 38.413 §9.3.4.5.
type NotificationCause uint8

const (
	NotificationCauseFulfilled    NotificationCause = 0
	NotificationCauseNotFulfilled NotificationCause = 1
)

// QosFlowNotifyItem ::= SEQUENCE { qosFlowIdentifier, notificationCause,
// iE-Extensions OPTIONAL } (extensible).
type QosFlowNotifyItem struct {
	_                 [0]struct{} `per:"extseq"`
	QosFlowIdentifier QosFlowIdentifier
	NotificationCause NotificationCause `per:"ENUMERATED,range:0..1,..."`
	_                 ieExtensions      `per:",skip"`
}

// QosFlowNotifyList ::= SEQUENCE (SIZE(1..maxnoofQosFlows)) OF QosFlowNotifyItem.
type QosFlowNotifyList []QosFlowNotifyItem

// PDUSessionResourceNotifyTransfer ::= SEQUENCE { qosFlowNotifyList OPTIONAL,
// qosFlowReleasedList OPTIONAL, iE-Extensions OPTIONAL } (extensible) —
// TS 38.413 §9.3.4.5.
type PDUSessionResourceNotifyTransfer struct {
	_               [0]struct{}          `per:"extseq"`
	QosFlowNotify   QosFlowNotifyList    `per:",optional"`
	QosFlowReleased QosFlowListWithCause `per:",optional"`
	_               ieExtensions         `per:",skip"`
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
	var t PDUSessionResourceNotifyTransfer

	if err := t.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, err
	}

	return &t, nil
}
