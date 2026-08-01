// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

// pergen loads this package through the root module, which therefore requires
// and replaces it. Until a consumer imports ngap, `go mod tidy` at the root
// drops that require as unused and this directive stops resolving; restore the
// require line to regenerate.
//go:generate sh -c "cd .. && go run ./cmd/pergen -o ngap/per_gen.go github.com/ellanetworks/core/ngap"

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/per"
)

// Hand-written Aligned-PER codecs for leaf types whose Go shape differs from
// their wire shape; pergen generates the SEQUENCE types built from them.

func (p PLMNIdentity) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 3, 3, true, true, false, p[:])
}

func (p *PLMNIdentity) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 3, 3, true, true, false)
	if err != nil {
		return err
	}

	copy(p[:], b)

	return nil
}

// tacMax is the largest value a three-octet TAC can hold.
const tacMax = 1<<24 - 1

func (t TAC) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if t > tacMax {
		return fmt.Errorf("ngap: TAC %d exceeds three octets", uint32(t))
	}

	return per.EncodeOctetString(w, enc, 3, 3, true, true, false,
		[]byte{byte(t >> 16), byte(t >> 8), byte(t)})
}

func (t *TAC) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 3, 3, true, true, false)
	if err != nil {
		return err
	}

	*t = TAC(uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2]))

	return nil
}

func (t FiveGTMSI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	var b [4]byte

	binary.BigEndian.PutUint32(b[:], uint32(t))

	return per.EncodeOctetString(w, enc, 4, 4, true, true, false, b[:])
}

func (t *FiveGTMSI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 4, 4, true, true, false)
	if err != nil {
		return err
	}

	*t = FiveGTMSI(binary.BigEndian.Uint32(b))

	return nil
}

func (s SST) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 1, 1, true, true, false, []byte{byte(s)})
}

func (s *SST) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 1, 1, true, true, false)
	if err != nil {
		return err
	}

	*s = SST(b[0])

	return nil
}

func (s SD) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 3, 3, true, true, false, s[:])
}

func (s *SD) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 3, 3, true, true, false)
	if err != nil {
		return err
	}

	copy(s[:], b)

	return nil
}

// The GUAMI fields are BIT STRINGs whose widths are not octet multiples, so
// each is packed from an integer rather than copied from octets.

func (a AMFRegionID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(a), amfRegionIDBits)
}

func (a *AMFRegionID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, amfRegionIDBits)
	if err != nil {
		return err
	}

	*a = AMFRegionID(v)

	return nil
}

func (a AMFSetID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(a), amfSetIDBits)
}

func (a *AMFSetID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, amfSetIDBits)
	if err != nil {
		return err
	}

	*a = AMFSetID(v)

	return nil
}

func (a AMFPointer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(a), amfPointerBits)
}

func (a *AMFPointer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, amfPointerBits)
	if err != nil {
		return err
	}

	*a = AMFPointer(v)

	return nil
}

// encodeBitStringUint writes a fixed-size BIT STRING holding the low nbits of
// v. A value wider than the field is an error rather than a silent truncation.
func encodeBitStringUint(w *per.Writer, enc per.Encoding, v uint64, nbits int) error {
	if nbits < 64 && v >= 1<<uint(nbits) {
		return fmt.Errorf("ngap: value %d exceeds %d-bit field", v, nbits)
	}

	return per.EncodeBitString(w, enc, int64(nbits), int64(nbits), true, true, false, uintToBits(v, nbits), nbits)
}

func decodeBitStringUint(r *per.Reader, enc per.Encoding, nbits int) (uint64, error) {
	b, _, err := per.DecodeBitString(r, enc, int64(nbits), int64(nbits), true, true, false)
	if err != nil {
		return 0, err
	}

	return bitsToUint(b, nbits), nil
}

func (g GlobalRANNodeID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	shape, ok := ranNodeIDShapes[g.Kind]
	if !ok {
		return fmt.Errorf("ngap: invalid GlobalRANNodeID kind %d", g.Kind)
	}

	if g.Bits < shape.lb || g.Bits > shape.ub {
		return fmt.Errorf("ngap: GlobalRANNodeID kind %d: %d bits outside SIZE(%d..%d)",
			g.Kind, g.Bits, shape.lb, shape.ub)
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, globalRANNodeIDAlternatives-1, int64(shape.outer)); err != nil {
		return err
	}

	// Global*-ID ::= SEQUENCE { pLMNIdentity, <node id>, iE-Extensions
	// OPTIONAL, ... }: extension bit, then the OPTIONAL field's presence bit.
	w.WriteBit(false)
	w.WriteBit(false)

	if err := g.PLMNIdentity.MarshalPER(w, enc); err != nil {
		return err
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, int64(nodeIDAlternatives[shape.outer]-1), int64(shape.inner)); err != nil {
		return err
	}

	return per.EncodeBitString(w, enc, int64(shape.lb), int64(shape.ub), true, true, false,
		uintToBits(uint64(g.Value), g.Bits), g.Bits)
}

func (g *GlobalRANNodeID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	outer, err := per.DecodeConstrainedWholeNumber(r, enc, 0, globalRANNodeIDAlternatives-1)
	if err != nil {
		return fmt.Errorf("ngap: GlobalRANNodeID choice: %w", err)
	}

	if outer == globalRANNodeIDChoiceExtensions {
		return decodeChoiceExtension(r, enc, "GlobalRANNodeID")
	}

	extBit, err := r.ReadBit()
	if err != nil {
		return err
	}

	extContainer, err := r.ReadBit()
	if err != nil {
		return err
	}

	var plmn PLMNIdentity
	if err := plmn.UnmarshalPER(r, enc); err != nil {
		return err
	}

	alternatives := nodeIDAlternatives[int(outer)]

	inner, err := per.DecodeConstrainedWholeNumber(r, enc, 0, int64(alternatives-1))
	if err != nil {
		return fmt.Errorf("ngap: %s choice: %w", nodeIDChoiceName[int(outer)], err)
	}

	// The last alternative of each nested CHOICE is its choice-Extensions.
	if int(inner) == alternatives-1 {
		return decodeChoiceExtension(r, enc, nodeIDChoiceName[int(outer)])
	}

	kind, ok := kindForShape[[2]int{int(outer), int(inner)}]
	if !ok {
		return fmt.Errorf("ngap: unreachable GlobalRANNodeID alternative %d/%d", outer, inner)
	}

	shape := ranNodeIDShapes[kind]

	b, nbits, err := per.DecodeBitString(r, enc, int64(shape.lb), int64(shape.ub), true, true, false)
	if err != nil {
		return err
	}

	if err := skipSequenceExtensionsPER(r, enc, extContainer, extBit); err != nil {
		return err
	}

	*g = GlobalRANNodeID{
		Kind:         kind,
		PLMNIdentity: plmn,
		Value:        uint32(bitsToUint(b, nbits)),
		Bits:         nbits,
	}

	return nil
}

func (u UERetentionInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeEnumerated(w, enc, ueRetentionInformationRootCount, true, int64(u))
}

func (u *UERetentionInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := per.DecodeEnumerated(r, enc, ueRetentionInformationRootCount, true)
	if err != nil {
		return err
	}

	*u = UERetentionInformation(idx)

	return nil
}

func (p PagingDRX) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeEnumerated(w, enc, pagingDRXRootCount, true, int64(p))
}

// UnmarshalPER decodes PagingDRX; an extension addition decodes to
// pagingDRXRootCount+k so it cannot collide with a root value.
func (p *PagingDRX) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := per.DecodeEnumerated(r, enc, pagingDRXRootCount, true)
	if err != nil {
		return err
	}

	*p = PagingDRX(idx)

	return nil
}

func (s SliceSupportList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofSliceItems, []SliceSupportItem(s))
}

func (s *SliceSupportList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[SliceSupportItem](r, enc, 1, maxnoofSliceItems)
	if err != nil {
		return err
	}

	*s = items

	return nil
}

func (b BroadcastPLMNList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofBPLMNs, []BroadcastPLMNItem(b))
}

func (b *BroadcastPLMNList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[BroadcastPLMNItem](r, enc, 1, maxnoofBPLMNs)
	if err != nil {
		return err
	}

	*b = items

	return nil
}

func (s SupportedTAList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTACs, []SupportedTAItem(s))
}

func (s *SupportedTAList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[SupportedTAItem](r, enc, 1, maxnoofTACs)
	if err != nil {
		return err
	}

	*s = items

	return nil
}

func (s ServedGUAMIList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofServedGUAMIs, []ServedGUAMIItem(s))
}

func (s *ServedGUAMIList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[ServedGUAMIItem](r, enc, 1, maxnoofServedGUAMIs)
	if err != nil {
		return err
	}

	*s = items

	return nil
}

func (l UEAssociatedLogicalNGConnectionList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofNGConnectionsToReset, []UEAssociatedLogicalNGConnectionItem(l))
}

func (l *UEAssociatedLogicalNGConnectionList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[UEAssociatedLogicalNGConnectionItem](r, enc, 1, maxnoofNGConnectionsToReset)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

func (p PLMNSupportList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPLMNs, []PLMNSupportItem(p))
}

func (p *PLMNSupportList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PLMNSupportItem](r, enc, 1, maxnoofPLMNs)
	if err != nil {
		return err
	}

	*p = items

	return nil
}

// ieExtensions is a ProtocolExtensionContainer: never encoded, discarded on decode.
type ieExtensions struct{}

func (ieExtensions) MarshalPER(*per.Writer, per.Encoding) error {
	return fmt.Errorf("ngap: unmodeled iE-Extensions cannot be encoded")
}

func (*ieExtensions) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeConstrainedWholeNumber(r, enc, 1, maxProtocolExtensions)
	if err != nil {
		return err
	}

	for i := int64(0); i < n; i++ {
		if _, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProtocolIEs); err != nil {
			return err
		}

		if _, err := per.DecodeEnumerated(r, enc, criticalityRootCount, false); err != nil {
			return err
		}

		if err := per.SkipOpenType(r, enc); err != nil {
			return err
		}
	}

	return nil
}

// skipSequenceExtensionsPER steps over a SEQUENCE's unmodeled iE-Extensions
// container and any extension additions.
func skipSequenceExtensionsPER(r *per.Reader, enc per.Encoding, extContainer, extAdditions bool) error {
	if extContainer {
		var e ieExtensions
		if err := e.UnmarshalPER(r, enc); err != nil {
			return err
		}
	}

	if !extAdditions {
		return nil
	}

	var present []bool

	err := per.DecodeNormallySmallLength(r, enc, func(count int64) error {
		present = make([]bool, count)
		for i := range present {
			b, err := r.ReadBit()
			if err != nil {
				return err
			}

			present[i] = b
		}

		return nil
	})
	if err != nil {
		return err
	}

	for _, p := range present {
		if p {
			if err := per.SkipOpenType(r, enc); err != nil {
				return err
			}
		}
	}

	return nil
}

// marshalSeqOf encodes a SEQUENCE (SIZE(lb..ub)) OF items.
func marshalSeqOf[T any](w *per.Writer, enc per.Encoding, lb, ub int64, items []T) error {
	off := 0

	return per.EncodeLength(w, enc, lb, ub, true, int64(len(items)), func(count int64) error {
		end := off + int(count)
		for i := off; i < end; i++ {
			m, ok := any(&items[i]).(per.Marshaler)
			if !ok {
				return fmt.Errorf("ngap: %T does not implement per.Marshaler", items[i])
			}

			if err := m.MarshalPER(w, enc); err != nil {
				return err
			}
		}

		off = end

		return nil
	})
}

// unmarshalSeqOf decodes a SEQUENCE (SIZE(lb..ub)) OF items.
func unmarshalSeqOf[T any](r *per.Reader, enc per.Encoding, lb, ub int64) ([]T, error) {
	var items []T

	err := per.DecodeLength(r, enc, lb, ub, true, func(count int64) error {
		start := len(items)
		items = append(items, make([]T, count)...)

		for i := int64(0); i < count; i++ {
			u, ok := any(&items[start+int(i)]).(per.Unmarshaler)
			if !ok {
				return fmt.Errorf("ngap: %T does not implement per.Unmarshaler", items[start+int(i)])
			}

			if err := u.UnmarshalPER(r, enc); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return items, nil
}

func perIEDecode(b []byte, u per.Unmarshaler) error {
	return u.UnmarshalPER(per.NewReader(b), per.Aligned)
}
