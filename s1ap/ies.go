// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// Container size bounds (TS 36.413, S1AP-Constants).
const (
	maxProtocolIEs        = 65535
	maxProtocolExtensions = 65535
)

var profile = &apimpl.Profile{
	Name:                  "s1ap",
	MaxProtocolIEs:        maxProtocolIEs,
	MaxProtocolExtensions: maxProtocolExtensions,
	OpaqueProcedures:      []uint8{uint8(ProcPrivateMessage)},
	ProcedureName:         func(pc uint8) string { return ProcedureCode(pc).String() },
	IEName:                func(id uint16) string { return ProtocolIEID(id).String() },
}

type ieField = apimpl.FieldValue[ProtocolIEID]

type rawIE = ap.Field[ProtocolIEID]

// A ProtocolIE-Field the message type does not model; Value is open-type bytes.
type RawIE = ap.Field[ProtocolIEID]

func encodeContainerField(w *per.Writer, enc per.Encoding, f ieField) error {
	return apimpl.EncodeField(profile, w, enc, f)
}

func decodeIE(r *per.Reader, u per.Unmarshaler) error {
	return u.UnmarshalPER(r, per.Aligned)
}

func restOctets(r *per.Reader) []byte {
	b, _ := r.ReadOctets(r.Bits() / 8)

	return b
}

// maxnoofERABs bounds the E-RAB SEQUENCE-OF lists (TS 36.413).
const maxnoofERABs = 256

type containerCrit = apimpl.ListItem[ProtocolIEID]

type containerItem interface {
	ForeignIEs() []RawIE
}

// ub is each list's own ASN.1 SIZE bound; that today's lists share 256 is
// coincidental.
//
//nolint:unparam
func encodeSingleContainerList[T any, PT interface {
	*T
	containerItem
}](w *per.Writer, enc per.Encoding, ub int64, id ProtocolIEID, crit Criticality, items []T,
) error {
	return apimpl.EncodeItemList(profile, w, enc, ub, id, crit, items)
}

// Each item is its own open type, so each gets a fresh reader.
//
//nolint:unparam
func decodeItemList[T any, PT interface {
	*T
	containerItem
}](r *per.Reader, enc per.Encoding, ub int64, itemID ProtocolIEID, crit Criticality,
) ([]T, error) {
	return apimpl.DecodeItemList[ProtocolIEID, T](profile, r, enc, ub, itemID, crit)
}

func diagnoseItemList[L ~[]T, T any](d *ap.Diagnosis[ProtocolIEID], id ProtocolIEID, crit Criticality, l *L) bool {
	items := []T(*l)
	notComprehended := apimpl.DiagnoseItemList(d, id, crit, &items)
	*l = L(items)

	return notComprehended
}

// Keeps every field, including ids the caller does not model.
//
//nolint:unparam
func decodeIEContainer(r *per.Reader, enc per.Encoding) ([]rawIE, error) {
	return apimpl.DecodeIEContainer[ProtocolIEID](profile, r, enc)
}

func perIEDecode(b []byte, u per.Unmarshaler) error {
	return apimpl.DecodeValue(b, u)
}

type itemList[L ~[]T, T any] struct {
	lb, ub int64
	id     ProtocolIEID
	crit   Criticality
	items  *L
	single bool
}

func containerList[L ~[]T, T any](ub int64, id ProtocolIEID, crit Criticality, items *L) itemList[L, T] {
	return itemList[L, T]{ub: ub, id: id, crit: crit, items: items, single: true}
}

func seqOf[L ~[]T, T any](lb, ub int64, items *L) itemList[L, T] {
	return itemList[L, T]{lb: lb, ub: ub, items: items}
}

func (l itemList[L, T]) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if l.single {
		return apimpl.EncodeItemList(profile, w, enc, l.ub, l.id, l.crit, []T(*l.items))
	}

	return marshalSeqOf(w, enc, l.lb, l.ub, []T(*l.items))
}

func (l itemList[L, T]) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	if l.single {
		return diagnoseItemList(d, l.id, l.crit, l.items)
	}

	return diagnoseItems(d, []T(*l.items))
}
