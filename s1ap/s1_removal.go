// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

type S1RemovalRequest struct {
	GlobalENBID GlobalENBID

	messageMeta
}

var s1RemovalRequestIEs = []ieSpec[S1RemovalRequest]{
	{
		ID: IDGlobalENBID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *S1RemovalRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.GlobalENBID)
		},
		Encode: func(m *S1RemovalRequest) (per.Marshaler, bool) { return &m.GlobalENBID, true },
	},
}

func (m *S1RemovalRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcS1Removal, s1RemovalRequestIEs, m)
}

func (m *S1RemovalRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcS1Removal,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseS1RemovalRequest(value []byte) (*S1RemovalRequest, error) {
	return parseMessageBody[S1RemovalRequest](ProcS1Removal, TriggeringInitiatingMessage, s1RemovalRequestIEs, value)
}

func DecodeS1RemovalRequest(value []byte) (*S1RemovalRequest, error) {
	return decodeMessageBody[S1RemovalRequest](ProcS1Removal, s1RemovalRequestIEs, value)
}

type S1RemovalResponse struct {
	MMEName                *Name
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var s1RemovalResponseIEs = []ieSpec[S1RemovalResponse]{
	{
		ID: IDMMEname, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *S1RemovalResponse, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.MMEName)
		},
		Encode: func(m *S1RemovalResponse) (per.Marshaler, bool) { return optionalIE(m.MMEName) },
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *S1RemovalResponse, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.CriticalityDiagnostics)
		},
		Encode: func(m *S1RemovalResponse) (per.Marshaler, bool) { return optionalIE(m.CriticalityDiagnostics) },
	},
}

func (m *S1RemovalResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcS1Removal, s1RemovalResponseIEs, m)
}

func (m *S1RemovalResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcS1Removal,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseS1RemovalResponse(value []byte) (*S1RemovalResponse, error) {
	return parseMessageBody[S1RemovalResponse](ProcS1Removal, TriggeringSuccessfulOutcome, s1RemovalResponseIEs, value)
}

func DecodeS1RemovalResponse(value []byte) (*S1RemovalResponse, error) {
	return decodeMessageBody[S1RemovalResponse](ProcS1Removal, s1RemovalResponseIEs, value)
}

type S1RemovalFailure struct {
	Cause                  *Cause
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var s1RemovalFailureIEs = []ieSpec[S1RemovalFailure]{
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *S1RemovalFailure, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.Cause)
		},
		Encode: func(m *S1RemovalFailure) (per.Marshaler, bool) { return optionalIE(m.Cause) },
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *S1RemovalFailure, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.CriticalityDiagnostics)
		},
		Encode: func(m *S1RemovalFailure) (per.Marshaler, bool) { return optionalIE(m.CriticalityDiagnostics) },
	},
}

func (m *S1RemovalFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcS1Removal, s1RemovalFailureIEs, m)
}

func (m *S1RemovalFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcS1Removal,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseS1RemovalFailure(value []byte) (*S1RemovalFailure, error) {
	return parseMessageBody[S1RemovalFailure](ProcS1Removal, TriggeringUnsuccessfulOutcome, s1RemovalFailureIEs, value)
}

func DecodeS1RemovalFailure(value []byte) (*S1RemovalFailure, error) {
	return decodeMessageBody[S1RemovalFailure](ProcS1Removal, s1RemovalFailureIEs, value)
}
