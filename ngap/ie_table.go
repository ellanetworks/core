// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"errors"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// One row of a message's TS 38.413 §9.2/§9.3 IE table.
type ieSpec[M any] = apimpl.IESpec[ProtocolIEID, M]

const (
	presenceOptional    = apimpl.PresenceOptional
	presenceConditional = apimpl.PresenceConditional
	presenceMandatory   = apimpl.PresenceMandatory
)

type messageMeta struct {
	apimpl.Message[ProtocolIEID]
}

type extensionMeta = apimpl.Preserved[ProtocolIEID]

func (u *messageMeta) meta() *messageMeta { return u }

func (u *messageMeta) appendUnknown(f RawIE) { apimpl.MessagePreserved(&u.Message).Append(f) }

func (u *messageMeta) setExtensionAdditions(a per.ExtensionAdditions) {
	*apimpl.MessageAdditions(&u.Message) = a
}

func (u messageMeta) Diagnostics() Diagnostics { return Diagnostics{u.Message.Diagnostics()} }

type message interface {
	meta() *messageMeta
}

func encodeMessageBody[M any, PM interface {
	*M
	message
}](w *per.Writer, enc per.Encoding, procedure ProcedureCode, table []ieSpec[M], m PM,
) error {
	return apimpl.EncodeMessageBody(profile, w, enc, procedure, table, (*M)(m), &m.meta().Message)
}

func decodeMessageBody[M any, PM interface {
	*M
	message
}](procedure ProcedureCode, table []ieSpec[M], value []byte,
) (PM, error) {
	m := PM(new(M))

	if err := apimpl.DecodeMessageBody(profile, procedure, table, value, (*M)(m), &m.meta().Message); err != nil {
		return nil, err
	}

	return m, nil
}

func parseMessageBody[M any, PM interface {
	*M
	message
}](procedure ProcedureCode, trigger TriggeringMessage, table []ieSpec[M], value []byte,
) (PM, error) {
	m := PM(new(M))

	if err := apimpl.ParseMessageBody(profile, procedure, table, value, (*M)(m), &m.meta().Message); err != nil {
		return nil, newAbstractSyntaxError(procedure, trigger, err)
	}

	return m, nil
}

func newAbstractSyntaxError(procedure ProcedureCode, trigger TriggeringMessage, err error) error {
	var e *ap.AbstractSyntaxError[ProtocolIEID]
	if !errors.As(err, &e) {
		return err
	}

	cause := CauseProtocolAbstractSyntaxErrorReject
	if e.FalselyConstructed {
		cause = CauseProtocolAbstractSyntaxErrorFalselyConstructedMessage
	}

	return &AbstractSyntaxError{
		Procedure: procedure,
		Trigger:   trigger,
		Cause:     cause.Cause(),
		IEs:       criticalityDiagnosticsItems(e.IEs),
		decoded:   e.Decoded,
	}
}

func criticalityDiagnosticsItems(ds []DiagnosticIE) []CriticalityDiagnosticsIEItem {
	return apimpl.DiagnosticItems(ds, maxnoofErrors, func(d DiagnosticIE, missing bool) CriticalityDiagnosticsIEItem {
		kind := TypeOfErrorNotUnderstood
		if missing {
			kind = TypeOfErrorMissing
		}

		return CriticalityDiagnosticsIEItem{IECriticality: d.Criticality, IEID: d.ID, TypeOfError: kind}
	})
}

func decodeTransfer[T any, PT interface {
	*T
	per.Unmarshaler
}](b []byte,
) (PT, error) {
	return apimpl.DecodeTransfer[ProtocolIEID, ProcedureCode, T, PT](profile, "", b)
}

func parseTransfer[T any, PT interface {
	*T
	per.Unmarshaler
}](b []byte,
) (PT, error) {
	return apimpl.ParseTransfer[ProtocolIEID, ProcedureCode, T, PT](profile, "", b)
}

func diagnosticsOf(v any) Diagnostics {
	return Diagnostics{apimpl.DiagnosticsOf[ProtocolIEID](v)}
}
