// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// NotifySourceNGRANNode ::= ENUMERATED { notifySource, ... }, an inline IE of
// the TS 38.413 §9.2.3.7 table. Asks the AMF to tell the source node the
// handover completed.
// NGAP-only: TS 36.413 leaves the source eNB to infer completion.
type NotifySourceNGRANNode uint8

const (
	NotifySourceNGRANNodeNotifySource NotifySourceNGRANNode = iota

	notifySourceNGRANNodeRootCount = 1
)

func (n NotifySourceNGRANNode) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, notifySourceNGRANNodeRootCount, int64(n), "NotifySourceNGRANNode")
}

func (n *NotifySourceNGRANNode) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, notifySourceNGRANNodeRootCount, "NotifySourceNGRANNode")
	if err != nil {
		return err
	}

	*n = NotifySourceNGRANNode(idx)

	return nil
}

// TS 38.413 §9.2.3.7. The target NG-RAN node reports that the UE arrived.
// TS 36.413 §9.1.5.7 carries the same report but locates the UE with a separate
// EUTRAN-CGI and TAI, where NGAP uses the UserLocationInformation CHOICE.
type HandoverNotify struct {
	AMFUENGAPID             AMFUENGAPID
	RANUENGAPID             RANUENGAPID
	UserLocationInformation *UserLocationInformation
	NotifySourceNGRANNode   *NotifySourceNGRANNode

	messageMeta
}

var handoverNotifyIEs = []ieSpec[HandoverNotify]{
	{
		id: IDAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverNotify, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *HandoverNotify) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: IDRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverNotify, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *HandoverNotify) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: IDUserLocationInformation, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverNotify, raw []byte, enc per.Encoding) error {
			var v UserLocationInformation

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.UserLocationInformation = &v

			return nil
		},
		encode: func(m *HandoverNotify) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
	{
		id: IDNotifySourceNGRANNode, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *HandoverNotify, raw []byte, enc per.Encoding) error {
			var v NotifySourceNGRANNode

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.NotifySourceNGRANNode = &v

			return nil
		},
		encode: func(m *HandoverNotify) (per.Marshaler, bool) {
			if m.NotifySourceNGRANNode == nil {
				return nil, false
			}

			return m.NotifySourceNGRANNode, true
		},
	},
}

func (m *HandoverNotify) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverNotification, handoverNotifyIEs, m)
}

func (m *HandoverNotify) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcHandoverNotification,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseHandoverNotify(value []byte) (*HandoverNotify, error) {
	return parseMessageBody[HandoverNotify](ProcHandoverNotification, TriggeringInitiatingMessage, handoverNotifyIEs, value)
}
