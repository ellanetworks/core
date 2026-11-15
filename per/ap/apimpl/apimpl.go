// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package apimpl

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/internal/impl"
)

type (
	Criticality     = impl.Criticality
	Presence        = impl.Presence
	WireCriticality = impl.WireCriticality
	Profile         = impl.Profile
	Envelope        = impl.Envelope
	RawValue        = impl.RawValue

	Field[ID ~uint16]               = impl.Field[ID]
	FieldValue[ID ~uint16]          = impl.FieldValue[ID]
	SingleContainer[ID ~uint16]     = impl.SingleContainer[ID]
	Preserved[ID ~uint16]           = impl.Preserved[ID]
	Message[ID ~uint16]             = impl.Message[ID]
	ListItem[ID ~uint16]            = impl.ListItem[ID]
	IESpec[ID ~uint16, M any]       = impl.IESpec[ID, M]
	Diagnosis[ID ~uint16]           = impl.Diagnosis[ID]
	Diagnostic[ID ~uint16]          = impl.Diagnostic[ID]
	Diagnostics[ID ~uint16]         = impl.Diagnostics[ID]
	AbstractSyntaxError[ID ~uint16] = impl.AbstractSyntaxError[ID]

	RawPDU[ID ~uint16, P ~uint8, T ~uint8] = impl.RawPDU[ID, P, T]
)

const (
	CriticalityReject = impl.CriticalityReject
	CriticalityIgnore = impl.CriticalityIgnore
	CriticalityNotify = impl.CriticalityNotify

	PresenceOptional    = impl.PresenceOptional
	PresenceConditional = impl.PresenceConditional
	PresenceMandatory   = impl.PresenceMandatory
)

func Wire(wire, spec Criticality) WireCriticality { return impl.Wire(wire, spec) }

func ListItemCriticality[ID ~uint16](wire Criticality) ListItem[ID] {
	return impl.ListItemCriticality[ID](wire)
}

func EncodePDU(p *Profile, e Envelope) ([]byte, error) { return impl.EncodePDU(p, e) }

func DecodePDU[P ~uint8](p *Profile, b []byte) (Envelope, error) { return impl.DecodePDU[P](p, b) }

func ParseRawPDU[ID ~uint16, P ~uint8, T ~uint8](p *Profile, b []byte) (*RawPDU[ID, P, T], error) {
	return impl.ParseRawPDU[ID, P, T](p, b)
}

func EncodeField[ID ~uint16](p *Profile, w *per.Writer, enc per.Encoding, f FieldValue[ID]) error {
	return impl.EncodeField(p, w, enc, f)
}

func EncodeIEContainer[ID ~uint16](p *Profile, w *per.Writer, enc per.Encoding, fields []FieldValue[ID]) error {
	return impl.EncodeIEContainer(p, w, enc, fields)
}

func DecodeIEContainer[ID ~uint16](p *Profile, r *per.Reader, enc per.Encoding) ([]Field[ID], error) {
	return impl.DecodeIEContainer[ID](p, r, enc)
}

func EncodeExtensionContainer[ID ~uint16](p *Profile, w *per.Writer, enc per.Encoding, fields []FieldValue[ID]) error {
	return impl.EncodeExtensionContainer(p, w, enc, fields)
}

func DecodeExtensionContainer[ID ~uint16, M any](p *Profile, r *per.Reader, enc per.Encoding, table []IESpec[ID, M], m *M, pres *Preserved[ID]) error {
	return impl.DecodeExtensionContainer(p, r, enc, table, m, pres)
}

func ExtensionFields[ID ~uint16, M any](table []IESpec[ID, M], m *M, pres Preserved[ID]) []FieldValue[ID] {
	return impl.ExtensionFields(table, m, pres)
}

func DiagnoseExtensions[ID ~uint16, M any](d *Diagnosis[ID], table []IESpec[ID, M], m *M, pres *Preserved[ID]) {
	impl.DiagnoseExtensions(d, table, m, pres)
}

func DecodeSingleContainerValue[ID ~uint16](p *Profile, r *per.Reader, enc per.Encoding) (SingleContainer[ID], error) {
	return impl.DecodeSingleContainerValue[ID](p, r, enc)
}

func DecodeValue(b []byte, u per.Unmarshaler) error { return impl.DecodeValue(b, u) }

func DiagnoseChoiceExtension[ID ~uint16, T any, PT interface {
	*T
	per.Marshaler
}](d *Diagnosis[ID], id ID, crit Criticality, alt *PT, raw **SingleContainer[ID],
) bool {
	return impl.DiagnoseChoiceExtension(d, id, crit, alt, raw)
}

func DiagnoseUnknownChoiceExtension[ID ~uint16](d *Diagnosis[ID], c *SingleContainer[ID]) bool {
	return impl.DiagnoseUnknownChoiceExtension(d, c)
}

func EncodeItemList[ID ~uint16, T any](p *Profile, w *per.Writer, enc per.Encoding, ub int64, id ID, crit Criticality, items []T) error {
	return impl.EncodeItemList(p, w, enc, ub, id, crit, items)
}

func DecodeItemList[ID ~uint16, T any](p *Profile, r *per.Reader, enc per.Encoding, ub int64, id ID, crit Criticality) ([]T, error) {
	return impl.DecodeItemList[ID, T](p, r, enc, ub, id, crit)
}

func DiagnoseItemList[ID ~uint16, T any](d *Diagnosis[ID], id ID, crit Criticality, items *[]T) bool {
	return impl.DiagnoseItemList(d, id, crit, items)
}

func MessagePreserved[ID ~uint16](m *Message[ID]) *Preserved[ID] { return impl.MessagePreserved(m) }

func MessageAdditions[ID ~uint16](m *Message[ID]) *per.ExtensionAdditions {
	return impl.MessageAdditions(m)
}

func EncodeMessageBody[ID ~uint16, P ~uint8, M any](p *Profile, w *per.Writer, enc per.Encoding, procedure P, table []IESpec[ID, M], m *M, meta *Message[ID]) error {
	return impl.EncodeMessageBody(p, w, enc, procedure, table, m, meta)
}

func DecodeMessageBody[ID ~uint16, P ~uint8, M any](p *Profile, procedure P, table []IESpec[ID, M], value []byte, m *M, meta *Message[ID]) error {
	return impl.DecodeMessageBody(p, procedure, table, value, m, meta)
}

func ParseMessageBody[ID ~uint16, P ~uint8, M any](p *Profile, procedure P, table []IESpec[ID, M], value []byte, m *M, meta *Message[ID]) error {
	return impl.ParseMessageBody(p, procedure, table, value, m, meta)
}

func DecodeTransfer[ID ~uint16, T any, PT interface {
	*T
	per.Unmarshaler
}](b []byte,
) (PT, error) {
	return impl.DecodeTransfer[ID, T, PT](b)
}

func ParseTransfer[ID ~uint16, T any, PT interface {
	*T
	per.Unmarshaler
}](b []byte,
) (PT, error) {
	return impl.ParseTransfer[ID, T, PT](b)
}

func Prune[ID ~uint16](v impl.Diagnosable[ID]) Diagnostics[ID] { return impl.Prune(v) }

func DiagnosticsOf[ID ~uint16](v any) Diagnostics[ID] { return impl.DiagnosticsOf[ID](v) }

func DiagnosticItems[ID ~uint16, T any](ds []Diagnostic[ID], limit int, item func(d Diagnostic[ID], missing bool) T) []T {
	return impl.DiagnosticItems(ds, limit, item)
}

func Reportable[T any](items []T, criticality func(T) Criticality) []T {
	return impl.Reportable(items, criticality)
}

func OptionalIE[ID ~uint16, M any, T any, PT interface {
	*T
	per.Marshaler
	per.Unmarshaler
}](id ID, crit Criticality, field func(m *M) **T,
) IESpec[ID, M] {
	return impl.OptionalIE[ID, M, T, PT](id, crit, field)
}

func ConditionalIE[ID ~uint16, M any, T any, PT interface {
	*T
	per.Marshaler
	per.Unmarshaler
}](id ID, crit Criticality, cond func(m *M) bool, field func(m *M) **T,
) IESpec[ID, M] {
	return impl.ConditionalIE[ID, M, T, PT](id, crit, cond, field)
}

func ListIE[ID ~uint16, M any, L interface {
	~[]E
	per.Marshaler
}, E any, PL interface {
	*L
	per.Marshaler
	per.Unmarshaler
}](id ID, crit Criticality, field func(m *M) *L,
) IESpec[ID, M] {
	return impl.ListIE[ID, M, L, E, PL](id, crit, field)
}
