// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// TS 36.413 §9.1.10.
type UECapabilityInfoIndication struct {
	MMEUES1APID       MMEUES1APID
	ENBUES1APID       ENBUES1APID
	UERadioCapability UERadioCapability
	// One opaque OCTET STRING, where NGAP's counterpart is a SEQUENCE of
	// separate NR and E-UTRA capabilities (§9.2.1.98).
	UERadioCapabilityForPaging              UERadioCapabilityForPaging
	UEApplicationLayerMeasurementCapability *UEApplicationLayerMeasurementCapability
	LTEMIndication                          *LTEMIndication
	UERadioCapabilityNRFormat               *UERadioCapability
	UERadioCapabilityForPagingNRFormat      *UERadioCapabilityForPaging

	messageMeta
}

var uECapabilityInfoIndicationIEs = []ieSpec[UECapabilityInfoIndication]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UECapabilityInfoIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *UECapabilityInfoIndication) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UECapabilityInfoIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *UECapabilityInfoIndication) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDUERadioCapability, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UECapabilityInfoIndication, r *per.Reader, enc per.Encoding) error {
			if err := decodeIE(r, &m.UERadioCapability); err != nil {
				return err
			}

			if m.UERadioCapability == nil {
				m.UERadioCapability = []byte{}
			}

			return nil
		},
		Encode: func(m *UECapabilityInfoIndication) (per.Marshaler, bool) {
			if m.UERadioCapability == nil {
				return nil, false
			}

			return m.UERadioCapability, true
		},
	},
	{
		ID: IDUERadioCapabilityForPaging, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *UECapabilityInfoIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.UERadioCapabilityForPaging)
		},
		Encode: func(m *UECapabilityInfoIndication) (per.Marshaler, bool) {
			if m.UERadioCapabilityForPaging == nil {
				return nil, false
			}

			return m.UERadioCapabilityForPaging, true
		},
	},
	apimpl.OptionalIE(IDUEApplicationLayerMeasurementCapability, CriticalityIgnore, func(m *UECapabilityInfoIndication) **UEApplicationLayerMeasurementCapability {
		return &m.UEApplicationLayerMeasurementCapability
	}),
	apimpl.OptionalIE(IDLTEMIndication, CriticalityIgnore, func(m *UECapabilityInfoIndication) **LTEMIndication { return &m.LTEMIndication }),
	apimpl.OptionalIE(IDUERadioCapabilityNRFormat, CriticalityIgnore, func(m *UECapabilityInfoIndication) **UERadioCapability { return &m.UERadioCapabilityNRFormat }),
	apimpl.OptionalIE(IDUERadioCapabilityForPagingNRFormat, CriticalityIgnore, func(m *UECapabilityInfoIndication) **UERadioCapabilityForPaging {
		return &m.UERadioCapabilityForPagingNRFormat
	}),
}

func (m *UECapabilityInfoIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUECapabilityInfoIndication, uECapabilityInfoIndicationIEs, m)
}

func (m *UECapabilityInfoIndication) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUECapabilityInfoIndication,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseUECapabilityInfoIndication(value []byte) (*UECapabilityInfoIndication, error) {
	return parseMessageBody[UECapabilityInfoIndication](ProcUECapabilityInfoIndication, TriggeringInitiatingMessage, uECapabilityInfoIndicationIEs, value)
}

func DecodeUECapabilityInfoIndication(value []byte) (*UECapabilityInfoIndication, error) {
	return decodeMessageBody[UECapabilityInfoIndication](ProcUECapabilityInfoIndication, uECapabilityInfoIndicationIEs, value)
}
