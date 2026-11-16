// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

//go:generate go run github.com/ellanetworks/core/per/cmd/pergen -profile profile -ieid ProtocolIEID

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/per"
)

// Hand-written codecs for leaf types whose Go shape differs from their wire
// shape; pergen generates the SEQUENCEs built from them.

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

func (t FiveGSTAC) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if t > fiveGSTACMax {
		return fmt.Errorf("s1ap: FiveGSTAC %d exceeds three octets", uint32(t))
	}

	return per.EncodeOctetString(w, enc, 3, 3, true, true, false, []byte{byte(t >> 16), byte(t >> 8), byte(t)})
}

func (t *FiveGSTAC) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 3, 3, true, true, false)
	if err != nil {
		return err
	}

	*t = FiveGSTAC(uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2]))

	return nil
}

func (g GNBID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if g.Bits < gnbIDMinBits || g.Bits > gnbIDMaxBits {
		return fmt.Errorf("s1ap: gNB-ID of %d bits, want %d..%d", g.Bits, gnbIDMinBits, gnbIDMaxBits)
	}

	if g.Bits < 32 && uint64(g.Value) >= 1<<uint(g.Bits) {
		return fmt.Errorf("s1ap: gNB id %d exceeds its own %d bits", g.Value, g.Bits)
	}

	return per.EncodeBitString(w, enc, gnbIDMinBits, gnbIDMaxBits, true, true, false,
		uintToBits(uint64(g.Value), g.Bits), g.Bits)
}

func (g *GNBID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, nbits, err := per.DecodeBitString(r, enc, gnbIDMinBits, gnbIDMaxBits, true, true, false)
	if err != nil {
		return err
	}

	*g = GNBID{Value: uint32(bitsToUint(b, nbits)), Bits: nbits}

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

func (e MacroENBID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(e), macroENBIDBits)
}

func (e *MacroENBID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, macroENBIDBits)
	*e = MacroENBID(v)

	return err
}

func (e HomeENBID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(e), homeENBIDBits)
}

func (e *HomeENBID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, homeENBIDBits)
	*e = HomeENBID(v)

	return err
}

func (e ShortMacroENBID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(e), shortMacroENBIDBits)
}

func (e *ShortMacroENBID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, shortMacroENBIDBits)
	*e = ShortMacroENBID(v)

	return err
}

func (e LongMacroENBID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(e), longMacroENBIDBits)
}

func (e *LongMacroENBID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, longMacroENBIDBits)
	*e = LongMacroENBID(v)

	return err
}

func encodeBitStringUint(w *per.Writer, enc per.Encoding, v uint64, nbits int) error {
	if nbits < 64 && v >= 1<<uint(nbits) {
		return fmt.Errorf("s1ap: value %d exceeds %d-bit field", v, nbits)
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

// The container holds at least one extension (SIZE(1..maxProtocolExtensions)),
// so a caller with nothing to send leaves the field nil.
func (c RAC) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 1, 1, true, true, false, []byte{byte(c)})
}

func (c *RAC) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 1, 1, true, true, false)
	if err != nil {
		return err
	}

	*c = RAC(b[0])

	return nil
}

func (c CI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 2, 2, true, true, false, []byte{byte(c >> 8), byte(c)})
}

func (c *CI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 2, 2, true, true, false)
	if err != nil {
		return err
	}

	*c = CI(uint16(b[0])<<8 | uint16(b[1]))

	return nil
}

func (c CellIdentity) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(c), cellIDBits)
}

func (c *CellIdentity) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, cellIDBits)
	*c = CellIdentity(v)

	return err
}

func (a SecurityAlgorithms) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if a.extended {
		return per.EncodeBitString(w, enc, securityAlgorithmsBits, securityAlgorithmsBits, true, true, true, []byte(a.octets), a.bits)
	}

	return per.EncodeBitString(w, enc, securityAlgorithmsBits, securityAlgorithmsBits, true, true, true,
		uintToBits(uint64(a.Value), securityAlgorithmsBits), securityAlgorithmsBits)
}

func (a *SecurityAlgorithms) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, nbits, err := per.DecodeBitString(r, enc, securityAlgorithmsBits, securityAlgorithmsBits, true, true, true)
	if err != nil {
		return err
	}

	if nbits == securityAlgorithmsBits {
		*a = SecurityAlgorithms{Value: uint16(bitsToUint(b, nbits))}

		return nil
	}

	head := make([]byte, 2)
	copy(head, b)

	*a = SecurityAlgorithms{
		Value:    uint16(bitsToUint(head, securityAlgorithmsBits)),
		extended: true,
		octets:   string(b),
		bits:     nbits,
	}

	return nil
}

func (l LAC) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 2, 2, true, true, false, []byte{byte(l >> 8), byte(l)})
}

func (l *LAC) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 2, 2, true, true, false)
	if err != nil {
		return err
	}

	*l = LAC(uint16(b[0])<<8 | uint16(b[1]))

	return nil
}

func (e EPLMNs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEPLMNs, []PLMNIdentity(e))
}

func (e *EPLMNs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PLMNIdentity](r, enc, 1, maxnoofEPLMNs)
	if err != nil {
		return err
	}

	*e = items

	return nil
}

func (f ForbiddenTACs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofForbTACs, []TAC(f))
}

func (f *ForbiddenTACs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAC](r, enc, 1, maxnoofForbTACs)
	if err != nil {
		return err
	}

	*f = items

	return nil
}

func (f ForbiddenLACs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofForbLACs, []LAC(f))
}

func (f *ForbiddenLACs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[LAC](r, enc, 1, maxnoofForbLACs)
	if err != nil {
		return err
	}

	*f = items

	return nil
}

func (f ForbiddenTAs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEPLMNsPlusOne, []ForbiddenTAsItem(f))
}

func (f *ForbiddenTAs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[ForbiddenTAsItem](r, enc, 1, maxnoofEPLMNsPlusOne)
	if err != nil {
		return err
	}

	*f = items

	return nil
}

func (f ForbiddenLAs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEPLMNsPlusOne, []ForbiddenLAsItem(f))
}

func (f *ForbiddenLAs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[ForbiddenLAsItem](r, enc, 1, maxnoofEPLMNsPlusOne)
	if err != nil {
		return err
	}

	*f = items

	return nil
}

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
