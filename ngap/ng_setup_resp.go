// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// TS 38.413 §9.2.6.2.
type NGSetupResponse struct {
	AMFName                string
	ServedGUAMIList        ServedGUAMIList
	RelativeAMFCapacity    *uint8
	PLMNSupportList        PLMNSupportList
	CriticalityDiagnostics *CriticalityDiagnostics
	UERetentionInformation *UERetentionInformation

	messageMeta
}

// relativeAMFCapacityBounds is RelativeAMFCapacity ::= INTEGER (0..255).
var relativeAMFCapacityBounds = per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true}

var nGSetupResponseIEs = []ieSpec[NGSetupResponse]{
	{
		id: idAMFName, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *NGSetupResponse, raw []byte, enc per.Encoding) error {
			var (
				err error
				n   Name
			)
			if err = perIEDecode(raw, &n); err == nil {
				m.AMFName = string(n)
			}

			return err
		},
		encode: func(m *NGSetupResponse) (per.Marshaler, bool) { return Name(m.AMFName), true },
	},
	{
		id: idServedGUAMIList, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *NGSetupResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ServedGUAMIList)
		},
		encode: func(m *NGSetupResponse) (per.Marshaler, bool) { return m.ServedGUAMIList, true },
	},
	{
		id: idRelativeAMFCapacity, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *NGSetupResponse, raw []byte, enc per.Encoding) error {
			v, err := per.DecodeInteger(per.NewReader(raw), enc, relativeAMFCapacityBounds)
			if err != nil {
				return err
			}

			c := uint8(v)
			m.RelativeAMFCapacity = &c

			return nil
		},
		encode: func(m *NGSetupResponse) (per.Marshaler, bool) {
			if m.RelativeAMFCapacity == nil {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return per.EncodeInteger(w, enc, relativeAMFCapacityBounds, int64(*m.RelativeAMFCapacity))
			}), true
		},
	},
	{
		id: idPLMNSupportList, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *NGSetupResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PLMNSupportList)
		},
		encode: func(m *NGSetupResponse) (per.Marshaler, bool) { return m.PLMNSupportList, true },
	},
	{
		id: idCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *NGSetupResponse, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *NGSetupResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		id: idUERetentionInformation, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *NGSetupResponse, raw []byte, enc per.Encoding) error {
			var v UERetentionInformation

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.UERetentionInformation = &v

			return nil
		},
		encode: func(m *NGSetupResponse) (per.Marshaler, bool) {
			if m.UERetentionInformation == nil {
				return nil, false
			}

			return m.UERetentionInformation, true
		},
	},
}

func (m *NGSetupResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcNGSetup, nGSetupResponseIEs, m)
}

func (m *NGSetupResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcNGSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseNGSetupResponse(value []byte) (*NGSetupResponse, error) {
	return parseMessageBody[NGSetupResponse](ProcNGSetup, TriggeringSuccessfulOutcome, nGSetupResponseIEs, value)
}
