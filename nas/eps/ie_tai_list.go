// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// TAI is an EPS tracking area identity: a PLMN and a 2-octet (16-bit) TAC
// (TS 24.301 §9.9.3.32). The narrower TAC is a 3GPP-mandated EPS/5GS
// difference — fgs.TAI carries a 3-octet TAC.
type TAI struct {
	PLMN nas.PLMN
	TAC  uint16
}

func (t TAI) String() string { return fmt.Sprintf("%s-%04x", t.PLMN, t.TAC) }

// PartialTAIListType selects how a partial tracking area identity list encodes
// the identities it holds (TS 24.301 §9.9.3.33, table 9.9.3.33.1).
type PartialTAIListType uint8

const (
	// PartialTAIListNonConsecutive is type "00": one PLMN, then one TAC per
	// identity.
	PartialTAIListNonConsecutive PartialTAIListType = 0
	// PartialTAIListConsecutive is type "01": one PLMN and one TAC, with the
	// remaining identities implied by counting up from it.
	PartialTAIListConsecutive PartialTAIListType = 1
	// PartialTAIListPerPLMN is type "10": a PLMN alongside each TAC.
	PartialTAIListPerPLMN PartialTAIListType = 2
)

func (t PartialTAIListType) String() string {
	switch t {
	case PartialTAIListNonConsecutive:
		return "non-consecutive TACs"
	case PartialTAIListConsecutive:
		return "consecutive TACs"
	case PartialTAIListPerPLMN:
		return "TAIs with per-entry PLMN"
	default:
		return fmt.Sprintf("reserved (%d)", uint8(t))
	}
}

// PartialTAIList is one partial tracking area identity list: between 1 and 16
// tracking area identities sharing one encoding.
type PartialTAIList struct {
	// Type is the encoding, which constrains what TAIs may hold.
	Type PartialTAIListType
	// TAIs holds every identity the partial list denotes, expanded. A
	// PartialTAIListConsecutive run holds each identity in the run even though
	// only the first reaches the wire.
	TAIs []TAI
}

// TAIList is a tracking area identity list IE value: one or more partial lists
// (TS 24.301 §9.9.3.33).
type TAIList []PartialTAIList

// maxTAIsPerPartialList is the largest number of elements the count field of a
// partial list can express.
const maxTAIsPerPartialList = 16

// maxTAIsTotal and maxTAIListOctets are the whole element's limits: TS 24.301 §9.9.3.33
// caps the list at 16 identities and the element at 98 octets, two of which are the
// IEI and length.
const (
	maxTAIsTotal     = 16
	maxTAIListOctets = 96
)

// maxTAC is the largest value the 2-octet EPS TAC can hold. The 5GS counterpart
// is three octets wide (TS 24.501 §9.11.3.9).
const maxTAC = 0xFFFF

// NewTAIList builds a list denoting exactly tais, grouping runs that share a
// PLMN into partial lists of at most 16 identities.
func NewTAIList(tais ...TAI) (TAIList, error) {
	if len(tais) == 0 {
		return nil, fmt.Errorf("nas/eps: TAI list needs at least one identity")
	}

	if len(tais) > maxTAIsTotal {
		return nil, fmt.Errorf("nas/eps: TAI list holds %d identities, the element carries %d", len(tais), maxTAIsTotal)
	}

	var out TAIList

	for _, t := range tais {
		if n := len(out); n > 0 {
			last := &out[n-1]
			if len(last.TAIs) < maxTAIsPerPartialList && last.TAIs[0].PLMN == t.PLMN {
				last.TAIs = append(last.TAIs, t)
				continue
			}
		}

		out = append(out, PartialTAIList{Type: PartialTAIListNonConsecutive, TAIs: []TAI{t}})
	}

	return out, nil
}

// TAIs returns every identity the list denotes, across all its partial lists.
func (l TAIList) TAIs() []TAI {
	var out []TAI
	for _, p := range l {
		out = append(out, p.TAIs...)
	}

	return out
}

// Contains reports whether the list denotes tai.
func (l TAIList) Contains(tai TAI) bool {
	for _, p := range l {
		for _, t := range p.TAIs {
			if t == tai {
				return true
			}
		}
	}

	return false
}

// ParseTAIList decodes a TAI list IE value.
func ParseTAIList(b []byte) (TAIList, error) {
	r := nas.NewReader(b)

	var out TAIList

	for r.Remaining() > 0 {
		p, err := parsePartialTAIList(r)
		if err != nil {
			return nil, err
		}

		out = append(out, p)
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("nas/eps: TAI list is empty")
	}

	if n := out.count(); n > maxTAIsTotal {
		return nil, fmt.Errorf("nas/eps: TAI list holds %d identities, the element carries %d", n, maxTAIsTotal)
	}

	if len(b) > maxTAIListOctets {
		return nil, fmt.Errorf("nas/eps: TAI list is %d octets, the element carries %d", len(b), maxTAIListOctets)
	}

	return out, nil
}

func parsePartialTAIList(r *nas.Reader) (PartialTAIList, error) {
	hdr, err := r.U8()
	if err != nil {
		return PartialTAIList{}, err
	}

	listType := PartialTAIListType(hdr >> 5 & 0x03)

	// The 5-bit count field spans 1-32, but only 1-16 is assigned; the rest is
	// reserved (TS 24.301 table 9.9.3.33.1).
	n := int(hdr&0x1F) + 1
	if n > maxTAIsPerPartialList {
		return PartialTAIList{}, fmt.Errorf("nas/eps: partial TAI list declares %d identities, want 1-%d", n, maxTAIsPerPartialList)
	}

	p := PartialTAIList{Type: listType, TAIs: make([]TAI, 0, n)}

	switch listType {
	case PartialTAIListNonConsecutive, PartialTAIListConsecutive:
		plmn, err := readPLMN(r)
		if err != nil {
			return PartialTAIList{}, err
		}

		if listType == PartialTAIListConsecutive {
			tac, err := r.U16()
			if err != nil {
				return PartialTAIList{}, err
			}

			// A run that would count past the 16-bit TAC is malformed: the
			// identities it claims do not exist.
			if uint32(tac)+uint32(n)-1 > maxTAC {
				return PartialTAIList{}, fmt.Errorf("nas/eps: consecutive TAC run from %#x for %d identities exceeds 16 bits", tac, n)
			}

			for i := range n {
				p.TAIs = append(p.TAIs, TAI{PLMN: plmn, TAC: tac + uint16(i)})
			}

			return p, nil
		}

		for range n {
			tac, err := r.U16()
			if err != nil {
				return PartialTAIList{}, err
			}

			p.TAIs = append(p.TAIs, TAI{PLMN: plmn, TAC: tac})
		}

		return p, nil
	case PartialTAIListPerPLMN:
		for range n {
			plmn, err := readPLMN(r)
			if err != nil {
				return PartialTAIList{}, err
			}

			tac, err := r.U16()
			if err != nil {
				return PartialTAIList{}, err
			}

			p.TAIs = append(p.TAIs, TAI{PLMN: plmn, TAC: tac})
		}

		return p, nil
	default:
		return PartialTAIList{}, fmt.Errorf("nas/eps: reserved partial TAI list type %d", listType)
	}
}

// AppendBinary encodes the TAI list IE value.
// The encoding is appended to b.
func (l TAIList) AppendBinary(b []byte) ([]byte, error) {
	if len(l) == 0 {
		return b, fmt.Errorf("nas/eps: TAI list needs at least one partial list")
	}

	if n := l.count(); n > maxTAIsTotal {
		return b, fmt.Errorf("nas/eps: TAI list holds %d identities, the element carries %d", n, maxTAIsTotal)
	}

	w := nas.NewWriter(b)

	for _, p := range l {
		if err := p.write(w); err != nil {
			return b, err
		}
	}

	out, err := w.Result(b)
	if err != nil {
		return b, err
	}

	if n := len(out) - len(b); n > maxTAIListOctets {
		return b, fmt.Errorf("nas/eps: TAI list encodes to %d octets, the element carries %d", n, maxTAIListOctets)
	}

	return out, nil
}

// count is the number of identities across every partial list.
func (l TAIList) count() int {
	var n int
	for _, p := range l {
		n += len(p.TAIs)
	}

	return n
}

// MarshalBinary encodes the TAI list IE value.
func (l TAIList) MarshalBinary() ([]byte, error) { return l.AppendBinary(nil) }

func (p PartialTAIList) write(w *nas.Writer) error {
	if len(p.TAIs) < 1 || len(p.TAIs) > maxTAIsPerPartialList {
		return fmt.Errorf("nas/eps: partial TAI list holds %d identities, want 1-%d", len(p.TAIs), maxTAIsPerPartialList)
	}

	w.U8(uint8(p.Type)<<5 | uint8(len(p.TAIs)-1))

	switch p.Type {
	case PartialTAIListNonConsecutive, PartialTAIListConsecutive:
		if err := requireSharedPLMN(p.TAIs); err != nil {
			return err
		}

		plmn, err := p.TAIs[0].PLMN.Octets()
		if err != nil {
			return err
		}

		w.Raw(plmn[:])

		if p.Type == PartialTAIListConsecutive {
			if err := requireConsecutiveTACs(p.TAIs); err != nil {
				return err
			}

			w.U16(p.TAIs[0].TAC)

			return nil
		}

		for _, t := range p.TAIs {
			w.U16(t.TAC)
		}

		return nil
	case PartialTAIListPerPLMN:
		for _, t := range p.TAIs {
			plmn, err := t.PLMN.Octets()
			if err != nil {
				return err
			}

			w.Raw(plmn[:])
			w.U16(t.TAC)
		}

		return nil
	default:
		return fmt.Errorf("nas/eps: reserved partial TAI list type %d", p.Type)
	}
}

func requireSharedPLMN(tais []TAI) error {
	for _, t := range tais[1:] {
		if t.PLMN != tais[0].PLMN {
			return fmt.Errorf("nas/eps: partial TAI list type %d needs one PLMN, got %s and %s",
				PartialTAIListNonConsecutive, tais[0], t)
		}
	}

	return nil
}

func requireConsecutiveTACs(tais []TAI) error {
	if uint32(tais[0].TAC)+uint32(len(tais))-1 > maxTAC {
		return fmt.Errorf("nas/eps: consecutive TAC run from %#x for %d identities exceeds 16 bits",
			tais[0].TAC, len(tais))
	}

	for i, t := range tais[1:] {
		if t.TAC != tais[0].TAC+uint16(i)+1 {
			return fmt.Errorf("nas/eps: partial TAI list type %d needs consecutive TACs, got %s after %s",
				PartialTAIListConsecutive, t, tais[i])
		}
	}

	return nil
}

func readPLMN(r *nas.Reader) (nas.PLMN, error) {
	b, err := r.Bytes(3)
	if err != nil {
		return nas.PLMN{}, err
	}

	return nas.ParsePLMN([3]byte{b[0], b[1], b[2]})
}
