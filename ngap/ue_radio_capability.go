// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// TS 38.413 §9.2.13.1.
type UERadioCapabilityInfoIndication struct {
	AMFUENGAPID       AMFUENGAPID
	RANUENGAPID       RANUENGAPID
	UERadioCapability UERadioCapability
	// A SEQUENCE of separate NR and E-UTRA capabilities, where S1AP's
	// counterpart is one opaque OCTET STRING (§9.3.1.68).
	UERadioCapabilityForPaging   *UERadioCapabilityForPaging
	UERadioCapabilityEUTRAFormat UERadioCapability
	XrDeviceWith2Rx              *XrDeviceWith2Rx

	messageMeta
}

var uERadioCapabilityInfoIndicationIEs = []ieSpec[UERadioCapabilityInfoIndication]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UERadioCapabilityInfoIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFUENGAPID)
		},
		Encode: func(m *UERadioCapabilityInfoIndication) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UERadioCapabilityInfoIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *UERadioCapabilityInfoIndication) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		ID: IDUERadioCapability, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UERadioCapabilityInfoIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.UERadioCapability)
		},
		Encode: func(m *UERadioCapabilityInfoIndication) (per.Marshaler, bool) {
			if m.UERadioCapability == nil {
				return nil, false
			}

			return m.UERadioCapability, true
		},
	},
	{
		ID: IDUERadioCapabilityForPaging, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *UERadioCapabilityInfoIndication, r *per.Reader, enc per.Encoding) error {
			var v UERadioCapabilityForPaging

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.UERadioCapabilityForPaging = &v

			return nil
		},
		Encode: func(m *UERadioCapabilityInfoIndication) (per.Marshaler, bool) {
			if m.UERadioCapabilityForPaging == nil {
				return nil, false
			}

			return m.UERadioCapabilityForPaging, true
		},
	},
	listExtension(IDUERadioCapabilityEUTRAFormat, CriticalityIgnore, func(m *UERadioCapabilityInfoIndication) *UERadioCapability { return &m.UERadioCapabilityEUTRAFormat }),
	optionalExtension(IDXrDeviceWith2Rx, CriticalityIgnore, func(m *UERadioCapabilityInfoIndication) **XrDeviceWith2Rx { return &m.XrDeviceWith2Rx }),
}

func (m *UERadioCapabilityInfoIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUERadioCapabilityInfoIndication, uERadioCapabilityInfoIndicationIEs, m)
}

func (m *UERadioCapabilityInfoIndication) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUERadioCapabilityInfoIndication,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseUERadioCapabilityInfoIndication(value []byte) (*UERadioCapabilityInfoIndication, error) {
	return parseMessageBody[UERadioCapabilityInfoIndication](ProcUERadioCapabilityInfoIndication, TriggeringInitiatingMessage, uERadioCapabilityInfoIndicationIEs, value)
}

func DecodeUERadioCapabilityInfoIndication(value []byte) (*UERadioCapabilityInfoIndication, error) {
	return decodeMessageBody[UERadioCapabilityInfoIndication](ProcUERadioCapabilityInfoIndication, uERadioCapabilityInfoIndicationIEs, value)
}
