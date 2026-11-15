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
	RelativeMMECapacity    *uint8
	CriticalityDiagnostics *CriticalityDiagnostics
	// Whether the MME retained UE contexts across the setup (§8.7.3.2); Ella
	// Core never does, so it is left absent on send.
	UERetentionInformation *UERetentionInformation

	messageMeta
}

// relativeMMECapacityBounds is RelativeMMECapacity ::= INTEGER (0..255).
var relativeMMECapacityBounds = per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true}

var s1SetupResponseIEs = []ieSpec[S1SetupResponse]{
	{
		ID: IDMMEname, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *S1SetupResponse, r *per.Reader, enc per.Encoding) error {
			var (
				err error
				n   Name
			)
			if err = decodeIE(r, &n); err == nil {
				name := string(n)
				m.MMEName = &name
			}

			return err
		},
		Encode: func(m *S1SetupResponse) (per.Marshaler, bool) {
			if m.MMEName == nil {
				return nil, false
			}

			return Name(*m.MMEName), true
		},
	},
	{
		ID: IDServedGUMMEIs, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *S1SetupResponse, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ServedGUMMEIs)
		},
		Encode: func(m *S1SetupResponse) (per.Marshaler, bool) {
			if len(m.ServedGUMMEIs) == 0 {
				return nil, false
			}

			return &m.ServedGUMMEIs, true
		},
	},
	{
		ID: IDRelativeMMECapacity, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *S1SetupResponse, r *per.Reader, enc per.Encoding) error {
			v, err := per.DecodeInteger(r, enc, relativeMMECapacityBounds)
			if err != nil {
				return err
			}

			c := uint8(v)
			m.RelativeMMECapacity = &c

			return nil
		},
		Encode: func(m *S1SetupResponse) (per.Marshaler, bool) {
			if m.RelativeMMECapacity == nil {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return per.EncodeInteger(w, enc, relativeMMECapacityBounds, int64(*m.RelativeMMECapacity))
			}), true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *S1SetupResponse, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *S1SetupResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		ID: IDUERetentionInformation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *S1SetupResponse, r *per.Reader, enc per.Encoding) error {
			var v UERetentionInformation

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.UERetentionInformation = &v

			return nil
		},
		Encode: func(m *S1SetupResponse) (per.Marshaler, bool) {
			if m.UERetentionInformation == nil {
				return nil, false
			}

			return m.UERetentionInformation, true
		},
	},
}

func (m *S1SetupResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcS1Setup, s1SetupResponseIEs, m)
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
	return parseMessageBody[S1SetupResponse](ProcS1Setup, TriggeringSuccessfulOutcome, s1SetupResponseIEs, value)
}

func DecodeS1SetupResponse(value []byte) (*S1SetupResponse, error) {
	return decodeMessageBody[S1SetupResponse](ProcS1Setup, s1SetupResponseIEs, value)
}
