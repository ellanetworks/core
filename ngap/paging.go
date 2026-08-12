// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// maxnoofTAIforPaging bounds TAIListForPaging (TS 38.413, NGAP-Constants).
const maxnoofTAIforPaging = 16

// UEPagingIdentity alternatives (TS 38.413 §9.3.3.18). One identity, closed by
// a choice-Extensions alternative rather than an extension marker.
const (
	uePagingIdentityFiveGSTMSI = iota
	uePagingIdentityChoiceExtensions

	uePagingIdentityAlternatives = 2
)

// PagingPriority ::= ENUMERATED { priolevel1..priolevel8, ... } — §9.3.1.78.
type PagingPriority uint8

const (
	PagingPriorityLevel1 PagingPriority = iota
	PagingPriorityLevel2
	PagingPriorityLevel3
	PagingPriorityLevel4
	PagingPriorityLevel5
	PagingPriorityLevel6
	PagingPriorityLevel7
	PagingPriorityLevel8

	pagingPriorityRootCount = 8
)

// PagingOrigin ::= ENUMERATED { non-3gpp, ... } — §9.3.1.79. Present only for
// paging triggered from a non-3GPP access, so its presence is the message.
type PagingOrigin uint8

const (
	PagingOriginNon3GPP PagingOrigin = iota

	pagingOriginRootCount = 1
)

// taiListForPagingItem ::= SEQUENCE { tAI, iE-Extensions OPTIONAL }
// (extensible). The TAI is TS 38.413 §9.3.3.11.
type taiListForPagingItem struct {
	_   [0]struct{} `per:"extseq"`
	TAI TAI
	_   ieExtensions `per:",skip"`
}

// UERadioCapabilityForPaging ::= SEQUENCE {
// uERadioCapabilityForPagingOfNR OPTIONAL, uERadioCapabilityForPagingOfEUTRA
// OPTIONAL, iE-Extensions OPTIONAL } (extensible) — §9.3.1.68.
type UERadioCapabilityForPaging struct {
	_     [0]struct{}                        `per:"extseq"`
	NR    *UERadioCapabilityForPagingOfNR    `per:",optional"`
	EUTRA *UERadioCapabilityForPagingOfEUTRA `per:",optional"`
	_     ieExtensions                       `per:",skip"`
}

// UERadioCapabilityForPagingOfNR ::= OCTET STRING (unconstrained).
type UERadioCapabilityForPagingOfNR []byte

func (c UERadioCapabilityForPagingOfNR) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *UERadioCapabilityForPagingOfNR) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = b

	return nil
}

// UERadioCapabilityForPagingOfEUTRA ::= OCTET STRING (unconstrained).
type UERadioCapabilityForPagingOfEUTRA []byte

func (c UERadioCapabilityForPagingOfEUTRA) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *UERadioCapabilityForPagingOfEUTRA) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = b

	return nil
}

// TS 38.413 §9.2.4.1. Every IE is ignore criticality, so none is a value type.
type Paging struct {
	FiveGSTMSI                 *FiveGSTMSI
	PagingDRX                  *PagingDRX
	TAIListForPaging           []TAI
	PagingPriority             *PagingPriority
	UERadioCapabilityForPaging *UERadioCapabilityForPaging
	PagingOrigin               *PagingOrigin

	messageMeta
}

var pagingIEs = []ieSpec[Paging]{
	{
		id: IDUEPagingIdentity, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *Paging, raw []byte, enc per.Encoding) error {
			sub := per.NewReader(raw)

			index, err := per.DecodeConstrainedWholeNumber(sub, enc, 0, uePagingIdentityAlternatives-1)
			if err != nil {
				return err
			}

			if index != uePagingIdentityFiveGSTMSI {
				return decodeChoiceExtension(sub, enc, "UEPagingIdentity")
			}

			var v FiveGSTMSI
			if err := v.UnmarshalPER(sub, enc); err != nil {
				return err
			}

			m.FiveGSTMSI = &v

			return nil
		},
		encode: func(m *Paging) (per.Marshaler, bool) {
			if m.FiveGSTMSI == nil {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				if err := per.EncodeConstrainedWholeNumber(w, enc, 0,
					uePagingIdentityAlternatives-1, uePagingIdentityFiveGSTMSI); err != nil {
					return err
				}

				return m.FiveGSTMSI.MarshalPER(w, enc)
			}), true
		},
	},
	{
		id: IDPagingDRX, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *Paging, raw []byte, enc per.Encoding) error {
			var drx PagingDRX

			if err := perIEDecode(raw, &drx); err != nil {
				return err
			}

			m.PagingDRX = &drx

			return nil
		},
		encode: func(m *Paging) (per.Marshaler, bool) {
			if m.PagingDRX == nil {
				return nil, false
			}

			return m.PagingDRX, true
		},
	},
	{
		id: IDTAIListForPaging, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *Paging, raw []byte, enc per.Encoding) error {
			items, err := unmarshalSeqOf[taiListForPagingItem](per.NewReader(raw), enc, 1, maxnoofTAIforPaging)
			if err != nil {
				return err
			}

			m.TAIListForPaging = make([]TAI, len(items))
			for i, it := range items {
				m.TAIListForPaging[i] = it.TAI
			}

			return nil
		},
		encode: func(m *Paging) (per.Marshaler, bool) {
			if len(m.TAIListForPaging) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				items := make([]taiListForPagingItem, len(m.TAIListForPaging))
				for i, tai := range m.TAIListForPaging {
					items[i] = taiListForPagingItem{TAI: tai}
				}

				return marshalSeqOf(w, enc, 1, maxnoofTAIforPaging, items)
			}), true
		},
	},
	{
		id: IDPagingPriority, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *Paging, raw []byte, enc per.Encoding) error {
			v, err := decodeRootEnumerated(per.NewReader(raw), enc, pagingPriorityRootCount, "PagingPriority")
			if err != nil {
				return err
			}

			p := PagingPriority(v)
			m.PagingPriority = &p

			return nil
		},
		encode: func(m *Paging) (per.Marshaler, bool) {
			if m.PagingPriority == nil {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				if int(*m.PagingPriority) >= pagingPriorityRootCount {
					return fmt.Errorf("ngap: PagingPriority %d outside the root values", *m.PagingPriority)
				}

				return encodeRootEnumerated(w, enc, pagingPriorityRootCount, int64(*m.PagingPriority), "PagingPriority")
			}), true
		},
	},
	{
		id: IDUERadioCapabilityForPaging, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *Paging, raw []byte, enc per.Encoding) error {
			var v UERadioCapabilityForPaging

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.UERadioCapabilityForPaging = &v

			return nil
		},
		encode: func(m *Paging) (per.Marshaler, bool) {
			if m.UERadioCapabilityForPaging == nil {
				return nil, false
			}

			return m.UERadioCapabilityForPaging, true
		},
	},
	{
		id: IDPagingOrigin, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *Paging, raw []byte, enc per.Encoding) error {
			v, err := decodeRootEnumerated(per.NewReader(raw), enc, pagingOriginRootCount, "PagingOrigin")
			if err != nil {
				return err
			}

			o := PagingOrigin(v)
			m.PagingOrigin = &o

			return nil
		},
		encode: func(m *Paging) (per.Marshaler, bool) {
			if m.PagingOrigin == nil {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				if int(*m.PagingOrigin) >= pagingOriginRootCount {
					return fmt.Errorf("ngap: PagingOrigin %d outside the root values", *m.PagingOrigin)
				}

				return encodeRootEnumerated(w, enc, pagingOriginRootCount, int64(*m.PagingOrigin), "PagingOrigin")
			}), true
		},
	},
}

func (m *Paging) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPaging, pagingIEs, m)
}

func (m *Paging) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcPaging,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParsePaging(value []byte) (*Paging, error) {
	return parseMessageBody[Paging](ProcPaging, TriggeringInitiatingMessage, pagingIEs, value)
}
