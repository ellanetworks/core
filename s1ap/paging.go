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

// CNDomain selects the core-network domain a Paging targets (TS 36.413).
// Ella Core pages the PS domain.
type CNDomain uint8

const (
	CNDomainPS CNDomain = 0
	CNDomainCS CNDomain = 1
)

// TS 36.413 §9.1.6.
type Paging struct {
	UEIdentityIndexValue *uint16
	STMSI                *STMSI
	CNDomain             *CNDomain
	TAIList              []TAI
	// UERadioCapabilityForPaging is the eNB-reported paging-specific capability
	// (TS 36.413 §9.1.6.1, optional-ignore); when set, the eNB may use it to apply
	// specific paging schemes. Empty means the IE is omitted.
	UERadioCapabilityForPaging []byte

	messageMeta
}

// taiItem ::= SEQUENCE { tAI TAI, iE-Extensions OPTIONAL, ... }.
type taiItem struct {
	_   [0]struct{} `per:"extseq"`
	TAI TAI
	_   ieExtensions `per:",skip"`
}

var pagingIEs = []ieSpec[Paging]{
	{
		id: idUEIdentityIndexValue, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *Paging, raw []byte, enc per.Encoding) error {
			b, nbits, err := per.DecodeBitString(per.NewReader(raw), enc, 10, 10, true, true, false)
			if err != nil {
				return err
			}

			if nbits != 10 || len(b) < 2 {
				return fmt.Errorf("s1ap: UE identity index value is %d bits", nbits)
			}

			v := uint16(b[0])<<2 | uint16(b[1])>>6
			m.UEIdentityIndexValue = &v

			return nil
		},
		encode: func(m *Paging) (per.Marshaler, bool) {
			if m.UEIdentityIndexValue == nil {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				b := []byte{byte(*m.UEIdentityIndexValue >> 2), byte(*m.UEIdentityIndexValue << 6)}

				return per.EncodeBitString(w, enc, 10, 10, true, true, false, b, 10)
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
				return fmt.Errorf("s1ap: unsupported UE paging identity extension alternative")
			}

			index, err := per.DecodeConstrainedWholeNumber(sub, enc, 0, uePagingIDRootCount-1)
			if err != nil {
				return err
			}

			if index != uePagingIDChoiceSTMSI {
				return fmt.Errorf("s1ap: unsupported UE paging identity choice %d", index)
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
	// UE Radio Capability for Paging follows the List of TAIs in the message
	// order (§9.1.6.1); included only when the eNB reported one.
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
