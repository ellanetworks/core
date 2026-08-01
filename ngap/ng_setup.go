// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// TS 38.413 §9.2.6.1.
type NGSetupRequest struct {
	GlobalRANNodeID        GlobalRANNodeID
	RANNodeName            *string
	SupportedTAList        SupportedTAList
	DefaultPagingDRX       *PagingDRX
	UERetentionInformation *UERetentionInformation

	messageMeta
}

var nGSetupRequestIEs = []ieSpec[NGSetupRequest]{
	{
		id: idGlobalRANNodeID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *NGSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.GlobalRANNodeID)
		},
		encode: func(m *NGSetupRequest) (per.Marshaler, bool) { return &m.GlobalRANNodeID, true },
	},
	{
		id: idRANNodeName, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *NGSetupRequest, raw []byte, enc per.Encoding) error {
			var (
				err error
				n   Name
			)
			if err = perIEDecode(raw, &n); err == nil {
				name := string(n)
				m.RANNodeName = &name
			}

			return err
		},
		encode: func(m *NGSetupRequest) (per.Marshaler, bool) {
			if m.RANNodeName == nil {
				return nil, false
			}

			return Name(*m.RANNodeName), true
		},
	},
	{
		id: idSupportedTAList, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *NGSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SupportedTAList)
		},
		encode: func(m *NGSetupRequest) (per.Marshaler, bool) { return m.SupportedTAList, true },
	},
	{
		id: idDefaultPagingDRX, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *NGSetupRequest, raw []byte, enc per.Encoding) error {
			var v PagingDRX

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.DefaultPagingDRX = &v

			return nil
		},
		encode: func(m *NGSetupRequest) (per.Marshaler, bool) {
			if m.DefaultPagingDRX == nil {
				return nil, false
			}

			return m.DefaultPagingDRX, true
		},
	},
	{
		id: idUERetentionInformation, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *NGSetupRequest, raw []byte, enc per.Encoding) error {
			var (
				err error
				v   UERetentionInformation
			)

			err = perIEDecode(raw, &v)
			m.UERetentionInformation = &v

			return err
		},
		encode: func(m *NGSetupRequest) (per.Marshaler, bool) {
			if m.UERetentionInformation == nil {
				return nil, false
			}

			return m.UERetentionInformation, true
		},
	},
}

func (m *NGSetupRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcNGSetup, nGSetupRequestIEs, m)
}

func (m *NGSetupRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcNGSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseNGSetupRequest(value []byte) (*NGSetupRequest, error) {
	return parseMessageBody[NGSetupRequest](ProcNGSetup, TriggeringInitiatingMessage, nGSetupRequestIEs, value)
}
