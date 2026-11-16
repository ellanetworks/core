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
)

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
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverNotify, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFUENGAPID)
		},
		Encode: func(m *HandoverNotify) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverNotify, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *HandoverNotify) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		ID: IDUserLocationInformation, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverNotify, r *per.Reader, enc per.Encoding) error {
			var v UserLocationInformation

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.UserLocationInformation = &v

			return nil
		},
		Encode: func(m *HandoverNotify) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
	{
		ID: IDNotifySourceNGRANNode, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverNotify, r *per.Reader, enc per.Encoding) error {
			var v NotifySourceNGRANNode

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.NotifySourceNGRANNode = &v

			return nil
		},
		Encode: func(m *HandoverNotify) (per.Marshaler, bool) {
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

func DecodeHandoverNotify(value []byte) (*HandoverNotify, error) {
	return decodeMessageBody[HandoverNotify](ProcHandoverNotification, handoverNotifyIEs, value)
}
