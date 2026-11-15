// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// TS 36.413 §9.1.8.4.
type S1SetupRequest struct {
	GlobalENBID      GlobalENBID
	ENBName          *string
	SupportedTAs     SupportedTAs
	DefaultPagingDRX *PagingDRX
	// What the eNB offers (§8.7.3.2); the MME never accepts it.
	UERetentionInformation *UERetentionInformation
	CSGIdList              CSGIdList
	NBIoTDefaultPagingDRX  *NBIoTDefaultPagingDRX
	ConnectedengNBList     ConnectedengNBList

	messageMeta
}

var s1SetupRequestIEs = []ieSpec[S1SetupRequest]{
	{
		ID: IDGlobalENBID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *S1SetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.GlobalENBID)
		},
		Encode: func(m *S1SetupRequest) (per.Marshaler, bool) { return &m.GlobalENBID, true },
	},
	{
		ID: IDENBname, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *S1SetupRequest, r *per.Reader, enc per.Encoding) error {
			var (
				err error
				n   Name
			)
			if err = decodeIE(r, &n); err == nil {
				name := string(n)
				m.ENBName = &name
			}

			return err
		},
		Encode: func(m *S1SetupRequest) (per.Marshaler, bool) {
			if m.ENBName == nil {
				return nil, false
			}

			return Name(*m.ENBName), true
		},
	},
	{
		ID: IDSupportedTAs, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *S1SetupRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SupportedTAs)
		},
		Encode: func(m *S1SetupRequest) (per.Marshaler, bool) { return m.SupportedTAs, true },
	},
	{
		ID: IDDefaultPagingDRX, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *S1SetupRequest, r *per.Reader, enc per.Encoding) error {
			var v PagingDRX

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.DefaultPagingDRX = &v

			return nil
		},
		Encode: func(m *S1SetupRequest) (per.Marshaler, bool) {
			if m.DefaultPagingDRX == nil {
				return nil, false
			}

			return m.DefaultPagingDRX, true
		},
	},
	apimpl.ListIE(IDCSGIdList, CriticalityReject, func(m *S1SetupRequest) *CSGIdList { return &m.CSGIdList }),
	{
		ID: IDUERetentionInformation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *S1SetupRequest, r *per.Reader, enc per.Encoding) error {
			var v UERetentionInformation

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.UERetentionInformation = &v

			return nil
		},
		Encode: func(m *S1SetupRequest) (per.Marshaler, bool) {
			if m.UERetentionInformation == nil {
				return nil, false
			}

			return m.UERetentionInformation, true
		},
	},
	apimpl.OptionalIE(IDNBIoTDefaultPagingDRX, CriticalityIgnore, func(m *S1SetupRequest) **NBIoTDefaultPagingDRX { return &m.NBIoTDefaultPagingDRX }),
	apimpl.ListIE(IDConnectedengNBList, CriticalityIgnore, func(m *S1SetupRequest) *ConnectedengNBList { return &m.ConnectedengNBList }),
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

func DecodeS1SetupRequest(value []byte) (*S1SetupRequest, error) {
	return decodeMessageBody[S1SetupRequest](ProcS1Setup, s1SetupRequestIEs, value)
}
