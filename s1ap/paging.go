// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

const (
	maxnoofTAIs = 256
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
)

// TS 36.413 §9.1.6.
type Paging struct {
	UEIdentityIndexValue *uint16
	UEPagingID           *UEPagingID
	PagingDRX            *PagingDRX
	CNDomain             *CNDomain
	TAIList              TAIList
	PagingPriority       *PagingPriority
	// The eNB-reported paging-specific capability; when set, the eNB may use it
	// to apply specific paging schemes.
	UERadioCapabilityForPaging []byte

	messageMeta
}

// TAIItem ::= SEQUENCE { tAI TAI, iE-Extensions OPTIONAL, ... }.
type TAIItem struct {
	extensions    per.ExtensionAdditions `per:",extraw"`
	TAI           TAI
	iEExtensions  extensionMeta `per:",exttable"`
	containerCrit `per:"-"`
}

type TAIList []TAIItem

func (l TAIList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofTAIs, IDTAIItem, CriticalityIgnore, l)
}

func (l *TAIList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[TAIItem](r, enc, maxnoofTAIs, IDTAIItem, CriticalityIgnore)
	*l = items

	return err
}

type UEPagingID struct {
	STMSI *STMSI                    `per:",choice:0"`
	IMSI  *IMSI                     `per:",choice:1"`
	Ext   *per.ExtensionAlternative `per:",extalt"`
}

type IMSI []byte

func (i IMSI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 3, 8, true, true, false, i)
}

func (i *IMSI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 3, 8, true, true, false)
	*i = b

	return err
}

// UE-Identity-Index-value ::= BIT STRING (SIZE(10)) — §9.2.3.10.
const ueIdentityIndexValueBits = 10

var pagingIEs = []ieSpec[Paging]{
	{
		ID: IDUEIdentityIndexValue, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *Paging, r *per.Reader, enc per.Encoding) error {
			n, err := decodeBitStringUint(r, enc, ueIdentityIndexValueBits)
			if err != nil {
				return err
			}

			v := uint16(n)
			m.UEIdentityIndexValue = &v

			return nil
		},
		Encode: func(m *Paging) (per.Marshaler, bool) {
			if m.UEIdentityIndexValue == nil {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeBitStringUint(w, enc, uint64(*m.UEIdentityIndexValue), ueIdentityIndexValueBits)
			}), true
		},
	},
	{
		ID: IDUEPagingID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *Paging, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.UEPagingID)
		},
		Encode: func(m *Paging) (per.Marshaler, bool) { return optionalIE(m.UEPagingID) },
	},
	{
		ID: IDPagingDRX, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *Paging, r *per.Reader, enc per.Encoding) error {
			var drx PagingDRX

			if err := decodeIE(r, &drx); err != nil {
				return err
			}

			m.PagingDRX = &drx

			return nil
		},
		Encode: func(m *Paging) (per.Marshaler, bool) {
			if m.PagingDRX == nil {
				return nil, false
			}

			return m.PagingDRX, true
		},
	},
	{
		ID: IDCNDomain, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *Paging, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.CNDomain)
		},
		Encode: func(m *Paging) (per.Marshaler, bool) { return optionalIE(m.CNDomain) },
	},
	{
		ID: IDTAIList, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *Paging, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.TAIList)
		},
		Encode: func(m *Paging) (per.Marshaler, bool) {
			if len(m.TAIList) == 0 {
				return nil, false
			}

			return &m.TAIList, true
		},
	},
	{
		ID: IDPagingPriority, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *Paging, r *per.Reader, enc per.Encoding) error {
			var v PagingPriority
			if err := v.UnmarshalPER(r, enc); err != nil {
				return err
			}

			m.PagingPriority = &v

			return nil
		},
		Encode: func(m *Paging) (per.Marshaler, bool) {
			if m.PagingPriority == nil {
				return nil, false
			}

			return m.PagingPriority, true
		},
	},
	{
		ID: IDUERadioCapabilityForPaging, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *Paging, r *per.Reader, enc per.Encoding) error {
			var err error

			m.UERadioCapabilityForPaging, err = per.DecodeOctetString(r, enc, 0, 0, true, false, false)

			return err
		},
		Encode: func(m *Paging) (per.Marshaler, bool) {
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

func DecodePaging(value []byte) (*Paging, error) {
	return decodeMessageBody[Paging](ProcPaging, pagingIEs, value)
}
