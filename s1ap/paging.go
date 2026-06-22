// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

const (
	maxnoofTAIs = 256

	// CNDomain ::= ENUMERATED { ps, cs } (non-extensible, TS 36.413 §9.2.3.22).
	cnDomainRootCount = 2
	// UEPagingID ::= CHOICE { s-TMSI, iMSI, ... } (extensible, §9.2.3.13).
	uePagingIDRootCount   = 2
	uePagingIDChoiceSTMSI = 0
)

// CNDomain selects the core-network domain a Paging targets (TS 36.413
// §9.2.3.22). Ella Core pages the PS domain.
type CNDomain uint8

const (
	CNDomainPS CNDomain = 0
	CNDomainCS CNDomain = 1
)

// Paging is the PAGING message (TS 36.413 §9.1.6): a non-UE-associated message
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
	UEIdentityIndexValue uint16 // 10-bit UE identity index (IMSI mod 1024)
	STMSI                STMSI  // UE Paging Identity (s-TMSI alternative)
	CNDomain             CNDomain
	TAIList              []TAI

	unmodeledIEs
}

func (m *Paging) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	taiEncoders := make([]func(*aper.Writer) error, len(m.TAIList))

	for i := range m.TAIList {
		tai := m.TAIList[i]
		taiEncoders[i] = func(w *aper.Writer) error {
			// TAIItem ::= SEQUENCE { tAI TAI, iE-Extensions OPTIONAL, ... }
			w.WriteSequencePreamble(true, false, []bool{false})

			return tai.encode(w)
		}
	}

	fields := []ieField{
		{id: idUEIdentityIndexValue, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			b := []byte{byte(m.UEIdentityIndexValue >> 2), byte(m.UEIdentityIndexValue << 6)}

			return w.WriteBitString(b, 10, 10, 10, false)
		}},
		{id: idUEPagingID, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			if err := w.WriteChoiceIndex(uePagingIDChoiceSTMSI, uePagingIDRootCount, true, false); err != nil {
				return err
			}

			return m.STMSI.encode(w)
		}},
		{id: idCNDomain, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return w.WriteEnum(int(m.CNDomain), cnDomainRootCount, false, false)
		}},
		{id: idTAIList, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofTAIs, idTAIItem, CriticalityIgnore, taiEncoders)
		}},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *Paging) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcPaging,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

// ParsePaging decodes a Paging from the open-type payload of an initiatingMessage.
func ParsePaging(value []byte) (*Paging, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: Paging preamble: %w", err)
	}

	fields, err := decodeIEContainer(r)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return nil, err
		}
	}

	m := &Paging{}

	var seenIndex, seenPagingID, seenCNDomain, seenTAIList bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idUEIdentityIndexValue:
			var (
				b     []byte
				nbits int
			)

			b, nbits, err = sub.ReadBitString(10, 10, false)
			if err == nil && nbits == 10 && len(b) >= 2 {
				m.UEIdentityIndexValue = uint16(b[0])<<2 | uint16(b[1])>>6
			}

			seenIndex = true
		case idUEPagingID:
			var index int

			index, _, err = sub.ReadChoiceIndex(uePagingIDRootCount, true)
			if err == nil && index == uePagingIDChoiceSTMSI {
				m.STMSI, err = decodeSTMSI(sub)
			} else if err == nil {
				err = fmt.Errorf("s1ap: unsupported UE paging identity choice %d", index)
			}

			seenPagingID = true
		case idCNDomain:
			var index int

			index, _, err = sub.ReadEnum(cnDomainRootCount, false)
			m.CNDomain = CNDomain(index)
			seenCNDomain = true
		case idTAIList:
			m.TAIList, err = decodeItemList(sub, maxnoofTAIs, decodeTAIItem)
			seenTAIList = true
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

func decodeTAIItem(b []byte) (TAI, error) {
	r := aper.NewReader(b)

	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return TAI{}, err
	}

	tai, err := decodeTAI(r)
	if err != nil {
		return TAI{}, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return TAI{}, err
	}

	return tai, nil
}
