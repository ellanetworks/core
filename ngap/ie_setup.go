// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/per"
)

// maxnoof constants for NG Setup IEs (TS 38.413, NGAP-Constants).
const (
	maxnoofTACs   = 256
	maxnoofBPLMNs = 12
	maxnoofPLMNs  = 12
)

const maxnoofExtSliceItems = 65535

// TAC ::= OCTET STRING (SIZE(3)). NR tracking area codes are three octets,
// where S1AP's are two.
type TAC uint32

// BroadcastPLMNItem ::= SEQUENCE { pLMNIdentity, tAISliceSupportList,
// iE-Extensions OPTIONAL } (extensible).
type BroadcastPLMNItem struct {
	_                   [0]struct{} `per:"extseq"`
	PLMNIdentity        PLMNIdentity
	TAISliceSupportList SliceSupportList
	_                   ieExtensions `per:",skip"`
}

// BroadcastPLMNList ::= SEQUENCE (SIZE(1..maxnoofBPLMNs)) OF
// BroadcastPLMNItem.
type BroadcastPLMNList []BroadcastPLMNItem

// SupportedTAItem ::= SEQUENCE { tAC, broadcastPLMNList, iE-Extensions
// OPTIONAL } (extensible).
type SupportedTAItem struct {
	_                 [0]struct{} `per:"extseq"`
	TAC               TAC
	BroadcastPLMNList BroadcastPLMNList
	_                 ieExtensions `per:",skip"`
}

// SupportedTAList ::= SEQUENCE (SIZE(1..maxnoofTACs)) OF SupportedTAItem.
type SupportedTAList []SupportedTAItem

// PLMNSupportItem ::= SEQUENCE { pLMNIdentity, sliceSupportList,
// iE-Extensions OPTIONAL } (extensible).
type PLMNSupportItem struct {
	PLMNIdentity             PLMNIdentity
	SliceSupportList         SliceSupportList
	NPNSupport               *NPNSupport
	ExtendedSliceSupportList ExtendedSliceSupportList
	OnboardingSupport        *OnboardingSupport

	iEExtensions messageMeta
	extensions   per.ExtensionAdditions
}

var pLMNSupportItemExtIEs = []ieSpec[PLMNSupportItem]{
	{
		id: IDNPNSupport, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PLMNSupportItem, raw []byte, enc per.Encoding) error {
			var v NPNSupport

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.NPNSupport = &v

			return nil
		},
		encode: func(m *PLMNSupportItem) (per.Marshaler, bool) {
			if m.NPNSupport == nil {
				return nil, false
			}

			return m.NPNSupport, true
		},
	},
	{
		id: IDExtendedSliceSupportList, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PLMNSupportItem, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ExtendedSliceSupportList)
		},
		encode: func(m *PLMNSupportItem) (per.Marshaler, bool) {
			if len(m.ExtendedSliceSupportList) == 0 {
				return nil, false
			}

			return m.ExtendedSliceSupportList, true
		},
	},
	{
		id: IDOnboardingSupport, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PLMNSupportItem, raw []byte, enc per.Encoding) error {
			var v OnboardingSupport

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.OnboardingSupport = &v

			return nil
		},
		encode: func(m *PLMNSupportItem) (per.Marshaler, bool) {
			if m.OnboardingSupport == nil {
				return nil, false
			}

			return m.OnboardingSupport, true
		},
	},
}

func (p *PLMNSupportItem) MarshalPER(w *per.Writer, enc per.Encoding) error {
	ext := extensionFields(pLMNSupportItemExtIEs, p, &p.iEExtensions)

	w.WriteBit(p.extensions.Present())
	w.WriteBit(len(ext) > 0)

	if err := p.PLMNIdentity.MarshalPER(w, enc); err != nil {
		return err
	}

	if err := p.SliceSupportList.MarshalPER(w, enc); err != nil {
		return err
	}

	if len(ext) > 0 {
		if err := encodeExtensionContainer(w, enc, ext); err != nil {
			return err
		}
	}

	if p.extensions.Present() {
		return p.extensions.MarshalPER(w, enc)
	}

	return nil
}

func (p *PLMNSupportItem) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	extBit, err := r.ReadBit()
	if err != nil {
		return err
	}

	extContainer, err := r.ReadBit()
	if err != nil {
		return err
	}

	if err := p.PLMNIdentity.UnmarshalPER(r, enc); err != nil {
		return err
	}

	if err := p.SliceSupportList.UnmarshalPER(r, enc); err != nil {
		return err
	}

	if extContainer {
		if err := decodeExtensionContainer(r, enc, pLMNSupportItemExtIEs, p, &p.iEExtensions); err != nil {
			return err
		}
	}

	if extBit {
		return p.extensions.UnmarshalPER(r, enc)
	}

	return nil
}

type NID uint64

const nidBits = 44

func (n NID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(n), nidBits)
}

func (n *NID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, nidBits)
	if err != nil {
		return err
	}

	*n = NID(v)

	return nil
}

const (
	npnSupportSNPN = iota
	npnSupportChoiceExtensions

	npnSupportAlternatives = 2
)

type NPNSupport struct {
	SNPN NID
}

func (n NPNSupport) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, npnSupportAlternatives-1, npnSupportSNPN); err != nil {
		return err
	}

	return n.SNPN.MarshalPER(w, enc)
}

func (n *NPNSupport) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, npnSupportAlternatives-1)
	if err != nil {
		return fmt.Errorf("ngap: NPN-Support choice: %w", err)
	}

	if idx == npnSupportChoiceExtensions {
		return decodeChoiceExtension(r, enc, "NPN-Support")
	}

	return n.SNPN.UnmarshalPER(r, enc)
}

type ExtendedSliceSupportList []SliceSupportItem

func (l ExtendedSliceSupportList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofExtSliceItems, []SliceSupportItem(l))
}

func (l *ExtendedSliceSupportList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[SliceSupportItem](r, enc, 1, maxnoofExtSliceItems)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type OnboardingSupport uint8

const (
	OnboardingSupportTrue OnboardingSupport = iota

	onboardingSupportRootCount = 1
)

func (o OnboardingSupport) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, onboardingSupportRootCount, int64(o), "OnboardingSupport")
}

func (o *OnboardingSupport) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, onboardingSupportRootCount, "OnboardingSupport")
	if err != nil {
		return err
	}

	*o = OnboardingSupport(idx)

	return nil
}

func extensionFields[M any](table []ieSpec[M], m *M, meta *messageMeta) []ieField {
	typed := make([]*ieField, len(table))

	for i, spec := range table {
		if val, ok := spec.encode(m); ok {
			typed[i] = &ieField{id: spec.id, crit: spec.crit, val: val}
		}
	}

	return orderFields(table, typed, meta)
}

func encodeExtensionContainer(w *per.Writer, enc per.Encoding, fields []ieField) error {
	if len(fields) > maxProtocolExtensions {
		return fmt.Errorf("ngap: %d extensions exceed maxProtocolExtensions", len(fields))
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 1, maxProtocolExtensions, int64(len(fields))); err != nil {
		return err
	}

	for _, f := range fields {
		if err := encodeContainerField(w, enc, f); err != nil {
			return err
		}
	}

	return nil
}

func decodeExtensionContainer[M any](r *per.Reader, enc per.Encoding, table []ieSpec[M], m *M, meta *messageMeta) error {
	n, err := per.DecodeConstrainedWholeNumber(r, enc, 1, maxProtocolExtensions)
	if err != nil {
		return err
	}

	if maxBits := int64(r.Bits()); n > maxBits/minIEFieldBits {
		return fmt.Errorf("ngap: extension container declares %d fields in %d bits", n, maxBits)
	}

	var rejected *notComprehendedIE

	lastIdx := -1

	for i := int64(0); i < n; i++ {
		id, err := per.DecodeConstrainedWholeNumber(r, enc, 0, maxProtocolIEs)
		if err != nil {
			return err
		}

		crit, err := per.DecodeEnumerated(r, enc, criticalityRootCount, false)
		if err != nil {
			return err
		}

		val, err := per.DecodeOpenTypeBytes(r, enc)
		if err != nil {
			return err
		}

		f := rawIE{id: ProtocolIEID(id), crit: Criticality(crit), value: val}

		if idx, spec, ok := lookupIESpec(table, f.id); ok {
			if idx <= lastIdx {
				return fmt.Errorf("ngap: extension %s out of order or repeated", f.id)
			}

			lastIdx = idx

			err := spec.decode(m, f.value, enc)
			if err == nil {
				meta.wire = append(meta.wire, wireSlot{id: f.id, crit: f.crit, unknown: -1})

				continue
			}

			if !errors.Is(err, errNotComprehended) {
				return err
			}

			var inner *notComprehendedIE
			if errors.As(err, &inner) && inner.Crit == CriticalityReject && rejected == nil {
				rejected = inner
			}
		}

		if f.crit == CriticalityReject {
			if rejected == nil {
				rejected = &notComprehendedIE{ID: f.id, Crit: CriticalityReject, What: "iE-Extensions"}
			}

			continue
		}

		meta.preserve(f)
	}

	if rejected != nil {
		return rejected
	}

	if wireMatchesTable(table, meta.wire) {
		meta.wire = nil
	}

	return nil
}

// PLMNSupportList ::= SEQUENCE (SIZE(1..maxnoofPLMNs)) OF PLMNSupportItem.
type PLMNSupportList []PLMNSupportItem

// UERetentionInformation ::= ENUMERATED { ues-retained, ... } (extensible) —
// TS 38.413 §9.3.1.117. Modeled but never accepted: Ella Core retains no UE
// context across NG Setup.
type UERetentionInformation uint8

const (
	UERetentionUesRetained UERetentionInformation = iota

	ueRetentionInformationRootCount = 1
)

// PagingDRX ::= ENUMERATED { v32, v64, v128, v256, ... } (extensible).
type PagingDRX uint8

const (
	PagingDRXv32 PagingDRX = iota
	PagingDRXv64
	PagingDRXv128
	PagingDRXv256

	pagingDRXRootCount = 4
)
