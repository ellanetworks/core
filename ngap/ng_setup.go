// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// TS 38.413 §9.2.6.1.
type NGSetupRequest struct {
	GlobalRANNodeID        GlobalRANNodeID
	RANNodeName            *string
	SupportedTAList        SupportedTAList
	DefaultPagingDRX       *PagingDRX
	UERetentionInformation *UERetentionInformation
	NBIoTDefaultPagingDRX  *NBIoTDefaultPagingDRX
	ExtendedRANNodeName    *ExtendedRANNodeName
	AIoTSupport            *AIoTSupport
	AdditionalULI          *AdditionalULI

	messageMeta
}

var nGSetupRequestIEs = []ieSpec[NGSetupRequest]{
	{
		ID: IDGlobalRANNodeID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *NGSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.GlobalRANNodeID)
		},
		Encode: func(m *NGSetupRequest) (per.Marshaler, bool) { return &m.GlobalRANNodeID, true },
	},
	{
		ID: IDRANNodeName, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *NGSetupRequest, r *per.Reader, enc per.Encoding) error {
			var (
				err error
				n   Name
			)
			if err = decodeIE(r, &n); err == nil {
				name := string(n)
				m.RANNodeName = &name
			}

			return err
		},
		Encode: func(m *NGSetupRequest) (per.Marshaler, bool) {
			if m.RANNodeName == nil {
				return nil, false
			}

			return Name(*m.RANNodeName), true
		},
	},
	{
		ID: IDSupportedTAList, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *NGSetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SupportedTAList)
		},
		Encode: func(m *NGSetupRequest) (per.Marshaler, bool) { return m.SupportedTAList, true },
	},
	{
		ID: IDDefaultPagingDRX, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *NGSetupRequest, r *per.Reader, enc per.Encoding) error {
			var v PagingDRX

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.DefaultPagingDRX = &v

			return nil
		},
		Encode: func(m *NGSetupRequest) (per.Marshaler, bool) {
			if m.DefaultPagingDRX == nil {
				return nil, false
			}

			return m.DefaultPagingDRX, true
		},
	},
	{
		ID: IDUERetentionInformation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *NGSetupRequest, r *per.Reader, enc per.Encoding) error {
			var v UERetentionInformation

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.UERetentionInformation = &v

			return nil
		},
		Encode: func(m *NGSetupRequest) (per.Marshaler, bool) {
			if m.UERetentionInformation == nil {
				return nil, false
			}

			return m.UERetentionInformation, true
		},
	},
	apimpl.OptionalIE(IDNBIoTDefaultPagingDRX, CriticalityIgnore, func(m *NGSetupRequest) **NBIoTDefaultPagingDRX { return &m.NBIoTDefaultPagingDRX }),
	apimpl.OptionalIE(IDExtendedRANNodeName, CriticalityIgnore, func(m *NGSetupRequest) **ExtendedRANNodeName { return &m.ExtendedRANNodeName }),
	apimpl.OptionalIE(IDAIoTSupport, CriticalityReject, func(m *NGSetupRequest) **AIoTSupport { return &m.AIoTSupport }),
	apimpl.OptionalIE(IDAdditionalULI, CriticalityIgnore, func(m *NGSetupRequest) **AdditionalULI { return &m.AdditionalULI }),
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

func DecodeNGSetupRequest(value []byte) (*NGSetupRequest, error) {
	return decodeMessageBody[NGSetupRequest](ProcNGSetup, nGSetupRequestIEs, value)
}
