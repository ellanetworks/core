// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// TS 36.413 §9.1.8.5.
type S1SetupResponse struct {
	MMEName                *string
	ServedGUMMEIs          ServedGUMMEIs
	RelativeMMECapacity    uint8
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

var s1SetupResponseIEs = []ieSpec[S1SetupResponse]{
	{
		id: idMMEname, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *S1SetupResponse, raw []byte, enc per.Encoding) error {
			var (
				err error
				n   Name
			)
			if err = perIEDecode(raw, &n); err == nil {
				name := string(n)
				m.MMEName = &name
			}

			return err
		},
		encode: func(m *S1SetupResponse) (per.Marshaler, bool) {
			if m.MMEName == nil {
				return nil, false
			}

			return Name(*m.MMEName), true
		},
	},
	{
		id: idServedGUMMEIs, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *S1SetupResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ServedGUMMEIs)
		},
		encode: func(m *S1SetupResponse) (per.Marshaler, bool) { return &m.ServedGUMMEIs, true },
	},
	{
		id: idRelativeMMECapacity, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *S1SetupResponse, raw []byte, enc per.Encoding) error {
			var (
				err error
				v   int64
			)

			v, err = per.DecodeInteger(per.NewReader(raw), enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true})
			m.RelativeMMECapacity = uint8(v)

			return err
		},
		encode: func(m *S1SetupResponse) (per.Marshaler, bool) {
			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true}, int64(m.RelativeMMECapacity))
			}), true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *S1SetupResponse, raw []byte, enc per.Encoding) error {
			var (
				err error
				cd  CriticalityDiagnostics
			)

			err = perIEDecode(raw, &cd)
			m.CriticalityDiagnostics = &cd

			return err
		},
		encode: func(m *S1SetupResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *S1SetupResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, s1SetupResponseIEs, m)
}

func (m *S1SetupResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcS1Setup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseS1SetupResponse(value []byte) (*S1SetupResponse, error) {
	return parseMessageBody[S1SetupResponse](ProcS1Setup, s1SetupResponseIEs, value)
}
