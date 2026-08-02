// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

//go:generate sh -c "cd .. && go run ./cmd/pergen -o s1ap/per_gen.go github.com/ellanetworks/core/s1ap"

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

func (t MTMSI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	var b [4]byte

	binary.BigEndian.PutUint32(b[:], uint32(t))

	return per.EncodeOctetString(w, enc, 4, 4, true, true, false, b[:])
}

func (t *MTMSI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 4, 4, true, true, false)
	if err != nil {
		return err
	}

	*t = MTMSI(binary.BigEndian.Uint32(b))

	return nil
}

func (t TAC) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 2, 2, true, true, false, []byte{byte(t >> 8), byte(t)})
}

func (t *TAC) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 2, 2, true, true, false)
	if err != nil {
		return err
	}

	*t = TAC(uint16(b[0])<<8 | uint16(b[1]))

	return nil
}

func (e ENBID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	nb, ok := enbIDBits[e.Kind]
	if !ok {
		return fmt.Errorf("s1ap: invalid ENB-ID kind %d", e.Kind)
	}

	switch e.Kind {
	case ENBIDMacro, ENBIDHome:
		w.WriteBit(false)

		if err := per.EncodeConstrainedWholeNumber(w, enc, 0, 1, int64(e.Kind)); err != nil {
			return err
		}

		return per.EncodeBitString(w, enc, int64(nb), int64(nb), true, true, false, uintToBits(uint64(e.Value), nb), nb)
	default:
		w.WriteBit(true)

		if err := per.EncodeNormallySmall(w, enc, int64(e.Kind-ENBIDShortMacro)); err != nil {
			return err
		}

		inner := per.NewWriter()
		if err := per.EncodeBitString(inner, enc, int64(nb), int64(nb), true, true, false, uintToBits(uint64(e.Value), nb), nb); err != nil {
			return err
		}

		inner.AlignToByte()

		return per.EncodeOpenTypeBytes(w, enc, inner.Bytes())
	}
}

func (e *ENBID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	isExt, err := per.DecodeBoolean(r, enc)
	if err != nil {
		return err
	}

	if !isExt {
		idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, 1)
		if err != nil {
			return err
		}

		kind := ENBIDMacro
		if idx == 1 {
			kind = ENBIDHome
		}

		nb := enbIDBits[kind]

		b, _, err := per.DecodeBitString(r, enc, int64(nb), int64(nb), true, true, false)
		if err != nil {
			return err
		}

		*e = ENBID{Kind: kind, Value: uint32(bitsToUint(b, nb))}

		return nil
	}

	extIdx, err := per.DecodeNormallySmall(r, enc)
	if err != nil {
		return err
	}

	kind := ENBIDShortMacro
	if extIdx == 1 {
		kind = ENBIDLongMacro
	}

	raw, err := per.DecodeOpenTypeBytes(r, enc)
	if err != nil {
		return err
	}

	nb := enbIDBits[kind]

	b, _, err := per.DecodeBitString(per.NewReader(raw), enc, int64(nb), int64(nb), true, true, false)
	if err != nil {
		return err
	}

	*e = ENBID{Kind: kind, Value: uint32(bitsToUint(b, nb))}

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

func (b BPLMNs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	off := 0

	return per.EncodeLength(w, enc, 1, maxnoofBPLMNs, true, int64(len(b)), func(count int64) error {
		end := off + int(count)
		for i := off; i < end; i++ {
			if err := b[i].MarshalPER(w, enc); err != nil {
				return err
			}
		}

		off = end

		return nil
	})
}

func (b *BPLMNs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	*b = nil

	return per.DecodeLength(r, enc, 1, maxnoofBPLMNs, true, func(count int64) error {
		start := len(*b)
		*b = append(*b, make(BPLMNs, count)...)

		for i := int64(0); i < count; i++ {
			if err := (*b)[start+int(i)].UnmarshalPER(r, enc); err != nil {
				return err
			}
		}

		return nil
	})
}

func (s SupportedTAs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	off := 0

	return per.EncodeLength(w, enc, 1, maxnoofTACs, true, int64(len(s)), func(count int64) error {
		end := off + int(count)
		for i := off; i < end; i++ {
			if err := s[i].MarshalPER(w, enc); err != nil {
				return err
			}
		}

		off = end

		return nil
	})
}

func (s *SupportedTAs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	*s = nil

	return per.DecodeLength(r, enc, 1, maxnoofTACs, true, func(count int64) error {
		start := len(*s)
		*s = append(*s, make(SupportedTAs, count)...)

		for i := int64(0); i < count; i++ {
			if err := (*s)[start+int(i)].UnmarshalPER(r, enc); err != nil {
				return err
			}
		}

		return nil
	})
}

// ieExtensions is a ProtocolExtensionContainer: never encoded, discarded on decode.
type ieExtensions struct{}

func (ieExtensions) MarshalPER(*per.Writer, per.Encoding) error {
	return fmt.Errorf("s1ap: unmodeled iE-Extensions cannot be encoded")
}

func (*ieExtensions) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeConstrainedWholeNumber(r, enc, 1, maxProtocolExtensions)
	if err != nil {
		return err
	}

	var rejected ProtocolIEID

	comprehended := true

	for i := int64(0); i < n; i++ {
		id, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProtocolIEs)
		if err != nil {
			return err
		}

		crit, err := per.DecodeEnumerated(r, enc, criticalityRootCount, false)
		if err != nil {
			return err
		}

		if err := per.SkipOpenType(r, enc); err != nil {
			return err
		}

		// TS 36.413 §10.3.4.2: an extension this version does not model is a
		// not-comprehended IE, and one marked reject stops the procedure. The
		// whole container is still consumed first so the reader stays aligned
		// for a caller whose criticality lets it carry on.
		if Criticality(crit) == CriticalityReject && comprehended {
			comprehended, rejected = false, ProtocolIEID(id)
		}
	}

	if !comprehended {
		return fmt.Errorf("%w: iE-Extensions %s", errNotComprehended, rejected)
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
				return fmt.Errorf("s1ap: %T does not implement per.Marshaler", items[i])
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
				return fmt.Errorf("s1ap: %T does not implement per.Unmarshaler", items[start+int(i)])
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
