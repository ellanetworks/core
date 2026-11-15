// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

// Container size bounds (TS 36.413, S1AP-Constants).
const (
	maxProtocolIEs        = 65535
	maxProtocolExtensions = 65535
)

var profile = &ap.Profile{
	Name:                  "s1ap",
	MaxProtocolIEs:        maxProtocolIEs,
	MaxProtocolExtensions: maxProtocolExtensions,
	OpaqueProcedures:      []uint8{uint8(ProcPrivateMessage)},
	ProcedureName:         func(pc uint8) string { return ProcedureCode(pc).String() },
	IEName:                func(id uint16) string { return ProtocolIEID(id).String() },
}

type ieField = ap.FieldValue[ProtocolIEID]

type rawIE = ap.Field[ProtocolIEID]

// A ProtocolIE-Field the message type does not model; Value is open-type bytes.
type RawIE = ap.Field[ProtocolIEID]

func encodeContainerField(w *per.Writer, enc per.Encoding, f ieField) error {
	return ap.EncodeField(profile, w, enc, f)
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

// ub is each list's own ASN.1 SIZE bound; that today's lists share 256 is
// coincidental.
//
//nolint:unparam
type containerCrit struct {
	crit ap.WireCriticality
}

func (c *containerCrit) wireCriticality() *containerCrit { return c }

type containerCritical interface {
	wireCriticality() *containerCrit
}

func encodeSingleContainerList[T any](w *per.Writer, enc per.Encoding, ub int64, id ProtocolIEID, crit Criticality, items []T) error {
	if len(items) < 1 || int64(len(items)) > ub {
		return fmt.Errorf("s1ap: single-container list length %d outside [1, %d]", len(items), ub)
	}

	off := 0

	return per.EncodeLength(w, enc, 1, ub, true, int64(len(items)), func(count int64) error {
		end := off + int(count)
		for i := off; i < end; i++ {
			m, ok := any(&items[i]).(per.Marshaler)
			if !ok {
				return fmt.Errorf("s1ap: %T does not implement per.Marshaler", items[i])
			}

			c := crit
			if h, ok := any(&items[i]).(containerCritical); ok {
				c = h.wireCriticality().crit.Of(crit)
			}

			if err := encodeContainerField(w, enc, ieField{ID: id, Criticality: c, Value: m}); err != nil {
				return err
			}
		}

		off = end

		return nil
	})
}

// Each item is its own open type, so each gets a fresh reader.
//
//nolint:unparam
func decodeItemList[T any](r *per.Reader, enc per.Encoding, ub int64, itemID ProtocolIEID, crit Criticality) ([]T, error) {
	var items []T

	err := per.DecodeLength(r, enc, 1, ub, true, func(count int64) error {
		for i := int64(0); i < count; i++ {
			id, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProtocolIEs)
			if err != nil {
				return err
			}

			if ProtocolIEID(id) != itemID {
				return fmt.Errorf("s1ap: list item %d has IE id %d, want %d", i, id, itemID)
			}

			c, err := per.DecodeEnumerated(r, enc, criticalityRootCount, false)
			if err != nil {
				return err
			}

			val, err := per.DecodeOpenTypeBytes(r, enc)
			if err != nil {
				return err
			}

			var item T

			u, ok := any(&item).(per.Unmarshaler)
			if !ok {
				return fmt.Errorf("s1ap: %T does not implement per.Unmarshaler", item)
			}

			if err := u.UnmarshalPER(per.NewReader(val), enc); err != nil {
				return err
			}

			if h, ok := any(&item).(containerCritical); ok {
				h.wireCriticality().crit = ap.Wire(Criticality(c), crit)
			}

			items = append(items, item)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return items, nil
}

// Keeps every field, including ids the caller does not model.
//
//nolint:unparam
func decodeIEContainer(r *per.Reader, enc per.Encoding) ([]rawIE, error) {
	return ap.DecodeIEContainer[ProtocolIEID](profile, r, enc)
}

func perIEDecode(b []byte, u per.Unmarshaler) error {
	return u.UnmarshalPER(per.NewReader(b), per.Aligned)
}

type itemList[T any] struct {
	lb, ub int64
	id     ProtocolIEID
	crit   Criticality
	items  []T
	single bool
}

func containerList[T any](ub int64, id ProtocolIEID, crit Criticality, items []T) itemList[T] {
	return itemList[T]{ub: ub, id: id, crit: crit, items: items, single: true}
}

func seqOf[T any](lb, ub int64, items []T) itemList[T] {
	return itemList[T]{lb: lb, ub: ub, items: items}
}

func (l itemList[T]) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if l.single {
		return encodeSingleContainerList(w, enc, l.ub, l.id, l.crit, l.items)
	}

	return marshalSeqOf(w, enc, l.lb, l.ub, l.items)
}

func (l itemList[T]) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool { return diagnoseItems(d, l.items) }
