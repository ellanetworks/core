// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// maxnoofTAIforPaging bounds TAIListForPaging (TS 38.413, NGAP-Constants).
const maxnoofTAIforPaging = 16

type UEPagingIdentity struct {
	FiveGSTMSI      *FiveGSTMSI      `per:",choice:0"`
	ChoiceExtension *SingleContainer `per:",choice:1,extie"`
}

// PagingPriority ::= ENUMERATED { priolevel1..priolevel8, ... } — §9.3.1.78.
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

// PagingOrigin ::= ENUMERATED { non-3gpp, ... } — §9.3.1.79. Present only for
// paging triggered from a non-3GPP access, so its presence is the message.
type PagingOrigin uint8

const (
	PagingOriginNon3GPP PagingOrigin = iota
)

// TAIListForPagingItem ::= SEQUENCE { tAI, iE-Extensions OPTIONAL }
// (extensible). The TAI is TS 38.413 §9.3.3.11.
type TAIListForPagingItem struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	TAI          TAI
	iEExtensions extensionMeta `per:",exttable"`
}

type TAIListForPaging []TAIListForPagingItem

func (l TAIListForPaging) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTAIforPaging, []TAIListForPagingItem(l))
}

func (l *TAIListForPaging) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAIListForPagingItem](r, enc, 1, maxnoofTAIforPaging)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

// UERadioCapabilityForPaging ::= SEQUENCE {
// uERadioCapabilityForPagingOfNR OPTIONAL, uERadioCapabilityForPagingOfEUTRA
// OPTIONAL, iE-Extensions OPTIONAL } (extensible) — §9.3.1.68.
type UERadioCapabilityForPaging struct {
	extensions   per.ExtensionAdditions             `per:",extraw"`
	NR           *UERadioCapabilityForPagingOfNR    `per:",optional"`
	EUTRA        *UERadioCapabilityForPagingOfEUTRA `per:",optional"`
	NBIoT        *UERadioCapabilityForPagingOfNBIoT `per:",extie:IDUERadioCapabilityForPagingOfNBIoT,crit:ignore"`
	iEExtensions extensionMeta                      `per:",exttable"`
}

// UERadioCapabilityForPagingOfNR ::= OCTET STRING (unconstrained).
type UERadioCapabilityForPagingOfNR []byte

func (c UERadioCapabilityForPagingOfNR) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *UERadioCapabilityForPagingOfNR) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = b

	return nil
}

// UERadioCapabilityForPagingOfEUTRA ::= OCTET STRING (unconstrained).
type UERadioCapabilityForPagingOfEUTRA []byte

func (c UERadioCapabilityForPagingOfEUTRA) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *UERadioCapabilityForPagingOfEUTRA) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = b

	return nil
}

type UERadioCapabilityForPagingOfNBIoT []byte

func (c UERadioCapabilityForPagingOfNBIoT) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *UERadioCapabilityForPagingOfNBIoT) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	*c = b

	return err
}

// TS 38.413 §9.2.4.1. Every IE is ignore criticality, so none is a value type.
type Paging struct {
	UEPagingIdentity             *UEPagingIdentity
	PagingDRX                    *PagingDRX
	TAIListForPaging             TAIListForPaging
	PagingPriority               *PagingPriority
	UERadioCapabilityForPaging   *UERadioCapabilityForPaging
	PagingOrigin                 *PagingOrigin
	AssistanceDataForPaging      *AssistanceDataForPaging
	NBIoTPagingeDRXInfo          *NBIoTPagingeDRXInfo
	NBIoTPagingDRX               *NBIoTPagingDRX
	EnhancedCoverageRestriction  *EnhancedCoverageRestriction
	WUSAssistanceInformation     *WUSAssistanceInformation
	EUTRAPagingeDRXInformation   *EUTRAPagingeDRXInformation
	CEmodeBrestricted            *CEmodeBrestricted
	NRPagingeDRXInformation      *NRPagingeDRXInformation
	PagingCause                  *PagingCause
	PEIPSassistanceInformation   *PEIPSassistanceInformation
	LPWUSPSAssistanceInformation *LPWUSPSAssistanceInformation
	LPWUSDisableIndication       *LPWUSDisableIndication

	messageMeta
}

var pagingIEs = []ieSpec[Paging]{
	{
		ID: IDUEPagingIdentity, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *Paging, r *per.Reader, enc per.Encoding) error {
			var v UEPagingIdentity
			if err := v.UnmarshalPER(r, enc); err != nil {
				return err
			}

			m.UEPagingIdentity = &v

			return nil
		},
		Encode: func(m *Paging) (per.Marshaler, bool) {
			if m.UEPagingIdentity == nil {
				return nil, false
			}

			return m.UEPagingIdentity, true
		},
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
		ID: IDTAIListForPaging, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *Paging, r *per.Reader, enc per.Encoding) error {
			return m.TAIListForPaging.UnmarshalPER(r, enc)
		},
		Encode: func(m *Paging) (per.Marshaler, bool) {
			if len(m.TAIListForPaging) == 0 {
				return nil, false
			}

			return m.TAIListForPaging, true
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
			var v UERadioCapabilityForPaging

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.UERadioCapabilityForPaging = &v

			return nil
		},
		Encode: func(m *Paging) (per.Marshaler, bool) {
			if m.UERadioCapabilityForPaging == nil {
				return nil, false
			}

			return m.UERadioCapabilityForPaging, true
		},
	},
	{
		ID: IDPagingOrigin, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *Paging, r *per.Reader, enc per.Encoding) error {
			var v PagingOrigin
			if err := v.UnmarshalPER(r, enc); err != nil {
				return err
			}

			m.PagingOrigin = &v

			return nil
		},
		Encode: func(m *Paging) (per.Marshaler, bool) {
			if m.PagingOrigin == nil {
				return nil, false
			}

			return m.PagingOrigin, true
		},
	},
	optionalExtension(IDAssistanceDataForPaging, CriticalityIgnore, func(m *Paging) **AssistanceDataForPaging { return &m.AssistanceDataForPaging }),
	optionalExtension(IDNBIoTPagingeDRXInfo, CriticalityIgnore, func(m *Paging) **NBIoTPagingeDRXInfo { return &m.NBIoTPagingeDRXInfo }),
	optionalExtension(IDNBIoTPagingDRX, CriticalityIgnore, func(m *Paging) **NBIoTPagingDRX { return &m.NBIoTPagingDRX }),
	optionalExtension(IDEnhancedCoverageRestriction, CriticalityIgnore, func(m *Paging) **EnhancedCoverageRestriction { return &m.EnhancedCoverageRestriction }),
	optionalExtension(IDWUSAssistanceInformation, CriticalityIgnore, func(m *Paging) **WUSAssistanceInformation { return &m.WUSAssistanceInformation }),
	optionalExtension(IDEUTRAPagingeDRXInformation, CriticalityIgnore, func(m *Paging) **EUTRAPagingeDRXInformation { return &m.EUTRAPagingeDRXInformation }),
	optionalExtension(IDCEmodeBrestricted, CriticalityIgnore, func(m *Paging) **CEmodeBrestricted { return &m.CEmodeBrestricted }),
	optionalExtension(IDNRPagingeDRXInformation, CriticalityIgnore, func(m *Paging) **NRPagingeDRXInformation { return &m.NRPagingeDRXInformation }),
	optionalExtension(IDPagingCause, CriticalityIgnore, func(m *Paging) **PagingCause { return &m.PagingCause }),
	optionalExtension(IDPEIPSassistanceInformation, CriticalityIgnore, func(m *Paging) **PEIPSassistanceInformation { return &m.PEIPSassistanceInformation }),
	optionalExtension(IDLPWUSPSAssistanceInformation, CriticalityIgnore, func(m *Paging) **LPWUSPSAssistanceInformation { return &m.LPWUSPSAssistanceInformation }),
	optionalExtension(IDLPWUSDisableIndication, CriticalityIgnore, func(m *Paging) **LPWUSDisableIndication { return &m.LPWUSDisableIndication }),
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
