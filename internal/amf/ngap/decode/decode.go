// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

// Package decode converts free5gc NGAP message types into validated Go
// value types so handlers cannot deref nil mandatory IEs. Handlers in
// internal/amf/ngap must not walk ProtocolIEs.List themselves.
package decode

import (
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

// DecodeReport maps 1:1 onto an NGAP ErrorIndication with
// CriticalityDiagnostics (3GPP TS 38.413 §10.3). A report is fatal iff
// at least one item carries reject criticality.
type DecodeReport struct {
	ProcedureCode        int64
	TriggeringMessage    aper.Enumerated
	ProcedureCriticality aper.Enumerated
	Items                []DecodeErrorItem
}

type DecodeErrorItem struct {
	IEID        int64
	Criticality aper.Enumerated
	TypeOfError aper.Enumerated
}

func (r *DecodeReport) MissingMandatory(id int64, criticality aper.Enumerated) {
	r.Items = append(r.Items, DecodeErrorItem{
		IEID:        id,
		Criticality: criticality,
		TypeOfError: ngapType.TypeOfErrorPresentMissing,
	})
}

func (r *DecodeReport) Malformed(id int64, criticality aper.Enumerated) {
	r.Items = append(r.Items, DecodeErrorItem{
		IEID:        id,
		Criticality: criticality,
		TypeOfError: ngapType.TypeOfErrorPresentNotUnderstood,
	})
}

func (r *DecodeReport) Fatal() bool {
	if r == nil {
		return false
	}

	for _, item := range r.Items {
		if item.Criticality == ngapType.CriticalityPresentReject {
			return true
		}
	}

	return false
}

func (r *DecodeReport) HasItems() bool {
	return r != nil && len(r.Items) > 0
}

func (r *DecodeReport) ToCriticalityDiagnostics() ngapType.CriticalityDiagnostics {
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
