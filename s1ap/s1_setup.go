// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// TS 36.413 §9.1.8.4.
type S1SetupRequest struct {
	GlobalENBID      GlobalENBID
	ENBName          *string
	SupportedTAs     SupportedTAs
	DefaultPagingDRX *PagingDRX
	// What the eNB offers (§8.7.3.2); the MME never accepts it.
	UERetentionInformation *UERetentionInformation

	messageMeta
}

var s1SetupRequestIEs = []ieSpec[S1SetupRequest]{
	{
		id: idGlobalENBID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *S1SetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.GlobalENBID)
		},
		encode: func(m *S1SetupRequest) (per.Marshaler, bool) { return &m.GlobalENBID, true },
	},
	{
		id: idENBname, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *S1SetupRequest, raw []byte, enc per.Encoding) error {
			var (
				err error
				n   Name
			)
			if err = perIEDecode(raw, &n); err == nil {
				name := string(n)
				m.ENBName = &name
			}

			return err
		},
		encode: func(m *S1SetupRequest) (per.Marshaler, bool) {
			if m.ENBName == nil {
				return nil, false
			}

			return Name(*m.ENBName), true
		},
	},
	{
		id: idSupportedTAs, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *S1SetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SupportedTAs)
		},
		encode: func(m *S1SetupRequest) (per.Marshaler, bool) { return m.SupportedTAs, true },
	},
	{
		id: idDefaultPagingDRX, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *S1SetupRequest, raw []byte, enc per.Encoding) error {
			var v PagingDRX

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.DefaultPagingDRX = &v

			return nil
		},
		encode: func(m *S1SetupRequest) (per.Marshaler, bool) {
			if m.DefaultPagingDRX == nil {
				return nil, false
			}

			return m.DefaultPagingDRX, true
		},
	},
	{
		id: idUERetentionInformation, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *S1SetupRequest, raw []byte, enc per.Encoding) error {
			var v UERetentionInformation

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.UERetentionInformation = &v

			return nil
		},
		encode: func(m *S1SetupRequest) (per.Marshaler, bool) {
			if m.UERetentionInformation == nil {
				return nil, false
			}

			return m.UERetentionInformation, true
		},
	},
}

func (m *S1SetupRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcS1Setup, s1SetupRequestIEs, m)
}

func (m *S1SetupRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcS1Setup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseS1SetupRequest(value []byte) (*S1SetupRequest, error) {
	return parseMessageBody[S1SetupRequest](ProcS1Setup, TriggeringInitiatingMessage, s1SetupRequestIEs, value)
}
