// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// ENBIDKind selects an ENB-ID CHOICE alternative (TS 36.413 §9.2.1.37).
type ENBIDKind uint8

const (
	ENBIDMacro      ENBIDKind = iota // root 0: BIT STRING (SIZE(20))
	ENBIDHome                        // root 1: BIT STRING (SIZE(28))
	ENBIDShortMacro                  // extension: BIT STRING (SIZE(18))
	ENBIDLongMacro                   // extension: BIT STRING (SIZE(21))
)

var enbIDBits = map[ENBIDKind]int{
	ENBIDMacro:      20,
	ENBIDHome:       28,
	ENBIDShortMacro: 18,
	ENBIDLongMacro:  21,
}

// ENB-ID ::= CHOICE { macroENB-ID, homeENB-ID, ..., short-macroENB-ID,
// long-macroENB-ID }: two root alternatives, two extension alternatives.
type ENBID struct {
	Kind  ENBIDKind
	Value uint32
}

func (e ENBID) encode(w *aper.Writer) error {
	nb, ok := enbIDBits[e.Kind]
	if !ok {
		return fmt.Errorf("s1ap: invalid ENB-ID kind %d", e.Kind)
	}

	switch e.Kind {
	case ENBIDMacro, ENBIDHome:
		if err := w.WriteChoiceIndex(int(e.Kind), 2, true, false); err != nil {
			return err
		}

		return w.WriteBitString(uintToBits(uint64(e.Value), nb), nb, nb, nb, false)
	default: // extension alternatives: index among extensions, value as open type
		extIdx := int(e.Kind - ENBIDShortMacro)
		if err := w.WriteChoiceIndex(extIdx, 2, true, true); err != nil {
			return err
		}

		var vw aper.Writer
		if err := vw.WriteBitString(uintToBits(uint64(e.Value), nb), nb, nb, nb, false); err != nil {
			return err
		}

		return w.WriteOpenType(vw.Bytes())
	}
}

func decodeENBID(r *aper.Reader) (ENBID, error) {
	idx, isExt, err := r.ReadChoiceIndex(2, true)
	if err != nil {
		return ENBID{}, err
	}

	if !isExt {
		kind := ENBIDMacro
		if idx == 1 {
			kind = ENBIDHome
		}

		nb := enbIDBits[kind]

		b, _, err := r.ReadBitString(nb, nb, false)
		if err != nil {
			return ENBID{}, err
		}

		return ENBID{Kind: kind, Value: uint32(bitsToUint(b, nb))}, nil
	}

	kind := ENBIDShortMacro
	if idx == 1 {
		kind = ENBIDLongMacro
	}

	raw, err := r.ReadOpenType()
	if err != nil {
		return ENBID{}, err
	}

	nb := enbIDBits[kind]

	b, _, err := aper.NewReader(raw).ReadBitString(nb, nb, false)
	if err != nil {
		return ENBID{}, err
	}

	return ENBID{Kind: kind, Value: uint32(bitsToUint(b, nb))}, nil
}

// GlobalENBID ::= SEQUENCE { pLMNidentity, eNB-ID, iE-Extensions OPTIONAL }
// (extensible).
type GlobalENBID struct {
	PLMNIdentity PLMNIdentity
	ENBID        ENBID
}

func (g GlobalENBID) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := g.PLMNIdentity.encode(w); err != nil {
		return err
	}

	return g.ENBID.encode(w)
}

func decodeGlobalENBID(r *aper.Reader) (GlobalENBID, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return GlobalENBID{}, err
	}

	plmn, err := decodePLMNIdentity(r)
	if err != nil {
		return GlobalENBID{}, err
	}

	enb, err := decodeENBID(r)
	if err != nil {
		return GlobalENBID{}, err
	}

	if opt[0] {
		if err := skipExtensionContainer(r); err != nil {
			return GlobalENBID{}, err
		}
	}

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return GlobalENBID{}, err
		}
	}

	return GlobalENBID{PLMNIdentity: plmn, ENBID: enb}, nil
}

// TAC ::= OCTET STRING (SIZE(2)).
type TAC uint16

func (t TAC) encode(w *aper.Writer) error {
	return w.WriteOctetString([]byte{byte(t >> 8), byte(t)}, 2, 2, false)
}

func decodeTAC(r *aper.Reader) (TAC, error) {
	b, err := r.ReadOctetString(2, 2, false)
	if err != nil {
		return 0, err
	}

	return TAC(uint16(b[0])<<8 | uint16(b[1])), nil
}

// BPLMNs ::= SEQUENCE (SIZE(1..maxnoofBPLMNs)) OF PLMNidentity.
type BPLMNs []PLMNIdentity

func (b BPLMNs) encode(w *aper.Writer) error {
	if err := w.WriteConstrainedLength(len(b), 1, maxnoofBPLMNs); err != nil {
		return err
	}

	for _, p := range b {
		if err := p.encode(w); err != nil {
			return err
		}
	}

	return nil
}

func decodeBPLMNs(r *aper.Reader) (BPLMNs, error) {
	n, err := r.ReadConstrainedLength(1, maxnoofBPLMNs)
	if err != nil {
		return nil, err
	}

	out := make(BPLMNs, 0, n)

	for i := 0; i < n; i++ {
		p, err := decodePLMNIdentity(r)
		if err != nil {
			return nil, err
		}

		out = append(out, p)
	}

	return out, nil
}

// SupportedTAItem ::= SEQUENCE { tAC, broadcastPLMNs, iE-Extensions OPTIONAL }
// (extensible).
type SupportedTAItem struct {
	TAC            TAC
	BroadcastPLMNs BPLMNs
}

func (it SupportedTAItem) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := it.TAC.encode(w); err != nil {
		return err
	}

	return it.BroadcastPLMNs.encode(w)
}

func decodeSupportedTAItem(r *aper.Reader) (SupportedTAItem, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return SupportedTAItem{}, err
	}

	tac, err := decodeTAC(r)
	if err != nil {
		return SupportedTAItem{}, err
	}

	plmns, err := decodeBPLMNs(r)
	if err != nil {
		return SupportedTAItem{}, err
	}

	if opt[0] {
		if err := skipExtensionContainer(r); err != nil {
			return SupportedTAItem{}, err
		}
	}

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return SupportedTAItem{}, err
		}
	}

	return SupportedTAItem{TAC: tac, BroadcastPLMNs: plmns}, nil
}

// SupportedTAs ::= SEQUENCE (SIZE(1..maxnoofTACs)) OF SupportedTAs-Item.
type SupportedTAs []SupportedTAItem

func (s SupportedTAs) encode(w *aper.Writer) error {
	if err := w.WriteConstrainedLength(len(s), 1, maxnoofTACs); err != nil {
		return err
	}

	for _, it := range s {
		if err := it.encode(w); err != nil {
			return err
		}
	}

	return nil
}

func decodeSupportedTAs(r *aper.Reader) (SupportedTAs, error) {
	n, err := r.ReadConstrainedLength(1, maxnoofTACs)
	if err != nil {
		return nil, err
	}

	out := make(SupportedTAs, 0, minInt(n, 16))

	for i := 0; i < n; i++ {
		it, err := decodeSupportedTAItem(r)
		if err != nil {
			return nil, err
		}

		out = append(out, it)
	}

	return out, nil
}

// PagingDRX ::= ENUMERATED { v32, v64, v128, v256, ... } (extensible).
type PagingDRX uint8

const (
	PagingDRXv32 PagingDRX = iota
	PagingDRXv64
	PagingDRXv128
	PagingDRXv256

	pagingDRXRootCount = 4
)

func (p PagingDRX) encode(w *aper.Writer) error {
	return w.WriteEnum(int(p), pagingDRXRootCount, true, false)
}

func decodePagingDRX(r *aper.Reader) (PagingDRX, error) {
	idx, _, err := r.ReadEnum(pagingDRXRootCount, true)
	if err != nil {
		return 0, err
	}

	return PagingDRX(idx), nil
}

// maxnoof constants for S1 Setup IEs (TS 36.413 §9.3, S1AP-Constants).
const (
	maxnoofTACs   = 256
	maxnoofBPLMNs = 6
)
