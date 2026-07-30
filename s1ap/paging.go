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

// Paging is the PAGING message (TS 36.413): a non-UE-associated message
// the MME sends to eNB(s) to reach an ECM-IDLE UE. Ella Core uses the S-TMSI
// paging identity and pages the operator's tracking area(s).
//
//	Paging ::= SEQUENCE {
//	    UEIdentityIndexValue   BIT STRING (SIZE(10)),  -- IMSI mod 1024
//	    UEPagingID             CHOICE { s-TMSI, iMSI, ... },
//	    CNDomain               ENUMERATED { ps, cs },
//	    TAIList                SEQUENCE (SIZE(1..256)) OF TAIItem,
//	    ... }
type Paging struct {
	UEIdentityIndexValue uint16
	STMSI                STMSI // UE Paging Identity (s-TMSI alternative)
	CNDomain             CNDomain
	TAIList              []TAI
	// UERadioCapabilityForPaging is the eNB-reported paging-specific capability
	// (TS 36.413 §9.1.6.1, optional-ignore); when set, the eNB may use it to apply
	// specific paging schemes. Empty means the IE is omitted.
	UERadioCapabilityForPaging []byte

	unmodeledIEs
}

// taiItem ::= SEQUENCE { tAI TAI, iE-Extensions OPTIONAL, ... }.
type taiItem struct {
	_   [0]struct{} `per:"extseq"`
	TAI TAI
	_   ieExtensions `per:",skip"`
}

func (m *Paging) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	taiItems := make([]taiItem, len(m.TAIList))
	for i, tai := range m.TAIList {
		taiItems[i] = taiItem{TAI: tai}
	}

	fields := []ieField{
		{id: idUEIdentityIndexValue, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			b := []byte{byte(m.UEIdentityIndexValue >> 2), byte(m.UEIdentityIndexValue << 6)}

			return per.EncodeBitString(w, enc, 10, 10, true, true, false, b, 10)
		})},
		{id: idUEPagingID, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			w.WriteBit(false)

			if err := per.EncodeConstrainedWholeNumber(w, enc, 0, uePagingIDRootCount-1, uePagingIDChoiceSTMSI); err != nil {
				return err
			}

			return m.STMSI.MarshalPER(w, enc)
		})},
		{id: idCNDomain, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return per.EncodeEnumerated(w, enc, cnDomainRootCount, false, int64(m.CNDomain))
		})},
		{id: idTAIList, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofTAIs, idTAIItem, CriticalityIgnore, taiItems)
		})},
	}

	// UE Radio Capability for Paging follows the List of TAIs in the message order
	// (TS 36.413 §9.1.6.1); included only when the eNB reported one.
	if len(m.UERadioCapabilityForPaging) > 0 {
		fields = append(fields, ieField{id: idUERadioCapabilityForPaging, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return per.EncodeOctetString(w, enc, 0, 0, true, false, false, m.UERadioCapabilityForPaging)
		})})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParsePaging decodes a Paging from the open-type payload of an initiatingMessage.
func ParsePaging(value []byte) (*Paging, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: Paging preamble: %w", err)
	}

	fields, err := decodeIEContainer(r, enc)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := skipSequenceExtensionsPER(r, enc, false, true); err != nil {
			return nil, err
		}
	}

	m := &Paging{}

	var seenIndex, seenPagingID, seenCNDomain, seenTAIList bool

	for _, f := range fields {
		switch f.id {
		case idUEIdentityIndexValue:
			var (
				b     []byte
				nbits int
			)

			b, nbits, err = per.DecodeBitString(per.NewReader(f.value), enc, 10, 10, true, true, false)
			if err == nil && nbits == 10 && len(b) >= 2 {
				m.UEIdentityIndexValue = uint16(b[0])<<2 | uint16(b[1])>>6
			}

			seenIndex = true
		case idUEPagingID:
			sub := per.NewReader(f.value)

			var isExt bool

			isExt, err = sub.ReadBit()
			if err == nil && isExt {
				err = fmt.Errorf("s1ap: unsupported UE paging identity extension alternative")
			}

			if err == nil {
				var index int64

				index, err = per.DecodeConstrainedWholeNumber(sub, enc, 0, uePagingIDRootCount-1)
				if err == nil && index == uePagingIDChoiceSTMSI {
					err = m.STMSI.UnmarshalPER(sub, enc)
				} else if err == nil {
					err = fmt.Errorf("s1ap: unsupported UE paging identity choice %d", index)
				}
			}

			seenPagingID = true
		case idCNDomain:
			var index int64

			index, err = per.DecodeEnumerated(per.NewReader(f.value), enc, cnDomainRootCount, false)
			m.CNDomain = CNDomain(index)
			seenCNDomain = true
		case idTAIList:
			var items []taiItem

			items, err = decodeItemList[taiItem](per.NewReader(f.value), enc, maxnoofTAIs)
			if err == nil {
				m.TAIList = make([]TAI, len(items))
				for i, it := range items {
					m.TAIList[i] = it.TAI
				}
			}

			seenTAIList = true
		case idUERadioCapabilityForPaging:
			m.UERadioCapabilityForPaging, err = per.DecodeOctetString(per.NewReader(f.value), enc, 0, 0, true, false, false)
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: Paging IE %d: %w", f.id, err)
		}
	}

	if !seenIndex || !seenPagingID || !seenCNDomain || !seenTAIList {
		return nil, fmt.Errorf("s1ap: Paging missing mandatory IE")
	}

	return m, nil
}
