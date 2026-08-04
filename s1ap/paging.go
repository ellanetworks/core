// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

const (
	maxnoofTAIs = 256

	// CNDomain ::= ENUMERATED { ps, cs } (non-extensible, TS 36.413).
	cnDomainRootCount = 2
	// UEPagingID ::= CHOICE { s-TMSI, iMSI, ... } (extensible).
	uePagingIDRootCount   = 2
	uePagingIDChoiceSTMSI = 0
)

// Ella Core pages the PS domain.
type CNDomain uint8

const (
	CNDomainPS CNDomain = 0
	CNDomainCS CNDomain = 1
)

// PagingPriority ::= ENUMERATED { priolevel1..priolevel8, ... } — §9.2.1.78.
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

// TS 36.413 §9.1.6.
type Paging struct {
	UEIdentityIndexValue *uint16
	STMSI                *STMSI
	PagingDRX            *PagingDRX
	CNDomain             *CNDomain
	TAIList              []TAI
	PagingPriority       *PagingPriority
	// The eNB-reported paging-specific capability; when set, the eNB may use it
	// to apply specific paging schemes.
	UERadioCapabilityForPaging []byte

	messageMeta
}

// taiItem ::= SEQUENCE { tAI TAI, iE-Extensions OPTIONAL, ... }.
type taiItem struct {
	_   [0]struct{} `per:"extseq"`
	TAI TAI
	_   ieExtensions `per:",skip"`
}

// UE-Identity-Index-value ::= BIT STRING (SIZE(10)) — §9.2.3.10.
const ueIdentityIndexValueBits = 10

var pagingIEs = []ieSpec[Paging]{
	{
		id: idUEIdentityIndexValue, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *Paging, raw []byte, enc per.Encoding) error {
			n, err := decodeBitStringUint(per.NewReader(raw), enc, ueIdentityIndexValueBits)
			if err != nil {
				return err
			}

			v := uint16(n)
			m.UEIdentityIndexValue = &v

			return nil
		},
		encode: func(m *Paging) (per.Marshaler, bool) {
			if m.UEIdentityIndexValue == nil {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeBitStringUint(w, enc, uint64(*m.UEIdentityIndexValue), ueIdentityIndexValueBits)
			}), true
		},
	},
	{
		id: idUEPagingID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *Paging, raw []byte, enc per.Encoding) error {
			sub := per.NewReader(raw)

			isExt, err := sub.ReadBit()
			if err != nil {
				return err
			}

			if isExt {
				return fmt.Errorf("%w: UE paging identity extension alternative", errNotComprehended)
			}

			index, err := per.DecodeConstrainedWholeNumber(sub, enc, 0, uePagingIDRootCount-1)
			if err != nil {
				return err
			}

			if index != uePagingIDChoiceSTMSI {
				return fmt.Errorf("%w: UE paging identity choice %d", errNotComprehended, index)
			}

			var v STMSI
			if err := v.UnmarshalPER(sub, enc); err != nil {
				return err
			}

			m.STMSI = &v

			return nil
		},
		encode: func(m *Paging) (per.Marshaler, bool) {
			if m.STMSI == nil {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				w.WriteBit(false)

				if err := per.EncodeConstrainedWholeNumber(w, enc, 0, uePagingIDRootCount-1, uePagingIDChoiceSTMSI); err != nil {
					return err
				}

				return m.STMSI.MarshalPER(w, enc)
			}), true
		},
	},
	{
		id: idPagingDRX, presence: presenceOptional, crit: CriticalityIgnore,
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
		id: idCNDomain, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *Paging, raw []byte, enc per.Encoding) error {
			index, err := per.DecodeEnumerated(per.NewReader(raw), enc, cnDomainRootCount, false)
			if err != nil {
				return err
			}

			v := CNDomain(index)
			m.CNDomain = &v

			return nil
		},
		encode: func(m *Paging) (per.Marshaler, bool) {
			if m.CNDomain == nil {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return per.EncodeEnumerated(w, enc, cnDomainRootCount, false, int64(*m.CNDomain))
			}), true
		},
	},
	{
		id: idTAIList, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *Paging, raw []byte, enc per.Encoding) error {
			items, err := decodeItemList[taiItem](per.NewReader(raw), enc, maxnoofTAIs)
			if err != nil {
				return err
			}

			m.TAIList = make([]TAI, len(items))
			for i, it := range items {
				m.TAIList[i] = it.TAI
			}

			return nil
		},
		encode: func(m *Paging) (per.Marshaler, bool) {
			if len(m.TAIList) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				items := make([]taiItem, len(m.TAIList))
				for i, tai := range m.TAIList {
					items[i] = taiItem{TAI: tai}
				}

				return encodeSingleContainerList(w, enc, maxnoofTAIs, idTAIItem, CriticalityIgnore, items)
			}), true
		},
	},
	{
		id: idPagingPriority, presence: presenceOptional, crit: CriticalityIgnore,
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
					return fmt.Errorf("s1ap: PagingPriority %d outside the root values", *m.PagingPriority)
				}

				return encodeRootEnumerated(w, enc, pagingPriorityRootCount, int64(*m.PagingPriority), "PagingPriority")
			}), true
		},
	},
	{
		id: idUERadioCapabilityForPaging, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *Paging, raw []byte, enc per.Encoding) error {
			var err error

			m.UERadioCapabilityForPaging, err = per.DecodeOctetString(per.NewReader(raw), enc, 0, 0, true, false, false)

			return err
		},
		encode: func(m *Paging) (per.Marshaler, bool) {
			if len(m.UERadioCapabilityForPaging) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return per.EncodeOctetString(w, enc, 0, 0, true, false, false, m.UERadioCapabilityForPaging)
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
