// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// maxnoofTAIforPaging bounds TAIListForPaging (TS 38.413, NGAP-Constants).
const maxnoofTAIforPaging = 16

// UEPagingIdentity alternatives (TS 38.413 §9.3.3.15). Unlike S1AP's UE Paging
// ID, which is an extensible CHOICE of S-TMSI and IMSI, this one carries a
// single identity and is closed by a choice-Extensions alternative.
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

// PagingOrigin ::= ENUMERATED { non-3gpp, ... } — §9.3.1.79. The IE is present
// only when the paging was triggered from a non-3GPP access, so the single root
// value carries the whole meaning.
type PagingOrigin uint8

const (
	PagingOriginNon3GPP PagingOrigin = iota

	pagingOriginRootCount = 1
)

// taiListForPagingItem ::= SEQUENCE { tAI, iE-Extensions OPTIONAL }
// (extensible). NGAP lists items directly where S1AP wraps them in a
// ProtocolIE-SingleContainer (TS 38.413 §9.3.3.24 vs TS 36.413 §9.2.3.16).
type taiListForPagingItem struct {
	_   [0]struct{} `per:"extseq"`
	TAI TAI
	_   ieExtensions `per:",skip"`
}

// UERadioCapabilityForPaging ::= SEQUENCE {
// uERadioCapabilityForPagingOfNR OPTIONAL, uERadioCapabilityForPagingOfEUTRA
// OPTIONAL, iE-Extensions OPTIONAL } (extensible) — §9.3.1.68.
//
// S1AP's IE of the same name is a bare OCTET STRING; NR splits the capability
// per RAT, so the 5G shape carries two.
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

// TS 38.413 §9.2.4.1.
//
// Every IE this message carries is ignore criticality, so none is a value type:
// a receiver takes what arrived and continues (§10.3.5).
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
		id: idUEPagingIdentity, presence: presenceMandatory, crit: CriticalityIgnore,
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
		id: idPagingDRX, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *Paging, raw []byte, enc per.Encoding) error {
			var (
				err error
				drx PagingDRX
			)

			err = perIEDecode(raw, &drx)
			m.PagingDRX = &drx

			return err
		},
		encode: func(m *Paging) (per.Marshaler, bool) {
			if m.PagingDRX == nil {
				return nil, false
			}

			return m.PagingDRX, true
		},
	},
	{
		id: idTAIListForPaging, presence: presenceMandatory, crit: CriticalityIgnore,
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
		id: idPagingPriority, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *Paging, raw []byte, enc per.Encoding) error {
			v, err := per.DecodeEnumerated(per.NewReader(raw), enc, pagingPriorityRootCount, true)
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

				return per.EncodeEnumerated(w, enc, pagingPriorityRootCount, true, int64(*m.PagingPriority))
			}), true
		},
	},
	{
		id: idUERadioCapabilityForPaging, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *Paging, raw []byte, enc per.Encoding) error {
			var (
				err error
				v   UERadioCapabilityForPaging
			)

			err = perIEDecode(raw, &v)
			m.UERadioCapabilityForPaging = &v

			return err
		},
		encode: func(m *Paging) (per.Marshaler, bool) {
			if m.UERadioCapabilityForPaging == nil {
				return nil, false
			}

			return m.UERadioCapabilityForPaging, true
		},
	},
	{
		id: idPagingOrigin, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *Paging, raw []byte, enc per.Encoding) error {
			v, err := per.DecodeEnumerated(per.NewReader(raw), enc, pagingOriginRootCount, true)
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

				return per.EncodeEnumerated(w, enc, pagingOriginRootCount, true, int64(*m.PagingOrigin))
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
