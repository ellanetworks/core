// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package decode converts free5gc NGAP message types into validated Go
// value types so handlers cannot deref nil mandatory IEs. Handlers in
// internal/amf/ngap must not walk ProtocolIEs.List themselves.
//
// Per-message decoder functions return a value plus a *Report. Callers
// must pass a non-nil Report; the mutating methods on *Report assume a
// non-nil receiver.
//
// Duplicate IE policy: when an IE id appears multiple times in a single
// message, the last well-formed occurrence wins. TS 38.413 forbids
// duplicates, but rejecting them would drop messages that real-world
// gNBs send.
package decode

import (
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

// Report accumulates structural problems found while decoding a PDU and
// maps 1:1 onto NGAP CriticalityDiagnostics. A report is fatal iff it
// carries a reject-criticality item or ProcedureRejected is set.
type Report struct {
	ProcedureCode        int64
	TriggeringMessage    aper.Enumerated
	ProcedureCriticality aper.Enumerated
	Items                []ErrorItem

	// ProcedureRejected marks the whole procedure unprocessable without
	// naming an IE. Set when the PDU body is nil; do not pair with per-IE
	// Items.
	ProcedureRejected bool
}

type ErrorItem struct {
	IEID        int64
	Criticality aper.Enumerated
	TypeOfError aper.Enumerated
}

func (r *Report) MissingMandatory(id int64, criticality aper.Enumerated) {
	r.Items = append(r.Items, ErrorItem{
		IEID:        id,
		Criticality: criticality,
		TypeOfError: ngapType.TypeOfErrorPresentMissing,
	})
}

func (r *Report) Malformed(id int64, criticality aper.Enumerated) {
	r.Items = append(r.Items, ErrorItem{
		IEID:        id,
		Criticality: criticality,
		TypeOfError: ngapType.TypeOfErrorPresentNotUnderstood,
	})
}

func (r *Report) Fatal() bool {
	if r == nil {
		return false
	}

	if r.ProcedureRejected {
		return true
	}

	for _, item := range r.Items {
		if item.Criticality == ngapType.CriticalityPresentReject {
			return true
		}
	}

	return false
}

func (r *Report) HasItems() bool {
	if r == nil {
		return false
	}

	return r.ProcedureRejected || len(r.Items) > 0
}

// FromInitiatingMessage reports whether the decoded message was an initiating
// message. Per TS 38.413 §10.3.4.2, §10.3.5, a fatal decode of an initiating
// message is answered with an Error Indication, while a fatal decode of a
// response is left to local error handling.
func (r *Report) FromInitiatingMessage() bool {
	return r != nil && r.TriggeringMessage == ngapType.TriggeringMessagePresentInitiatingMessage
}

func (r *Report) ToCriticalityDiagnostics() ngapType.CriticalityDiagnostics {
	cd := ngapType.CriticalityDiagnostics{
		ProcedureCode: &ngapType.ProcedureCode{
			Value: r.ProcedureCode,
		},
		TriggeringMessage: &ngapType.TriggeringMessage{
			Value: r.TriggeringMessage,
		},
		ProcedureCriticality: &ngapType.Criticality{
			Value: r.ProcedureCriticality,
		},
	}

	if len(r.Items) == 0 {
		return cd
	}

	list := &ngapType.CriticalityDiagnosticsIEList{}
	for _, item := range r.Items {
		list.List = append(list.List, ngapType.CriticalityDiagnosticsIEItem{
			IECriticality: ngapType.Criticality{Value: item.Criticality},
			IEID:          ngapType.ProtocolIEID{Value: item.IEID},
			TypeOfError:   ngapType.TypeOfError{Value: item.TypeOfError},
		})
	}

	cd.IEsCriticalityDiagnostics = list

	return cd
}
