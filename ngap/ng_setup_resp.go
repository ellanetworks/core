// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// TS 38.413 §9.2.6.2.
type NGSetupResponse struct {
	AMFName                string
	ServedGUAMIList        ServedGUAMIList
	RelativeAMFCapacity    *uint8
	PLMNSupportList        PLMNSupportList
	CriticalityDiagnostics *CriticalityDiagnostics
	UERetentionInformation *UERetentionInformation
	ExtendedAMFName        *ExtendedAMFName
	IABSupported           *IABSupported
	MobileIABSupported     *MobileIABSupported
	AIOTFIdentifier        *AIOTFIdentifier
	AIOTFName              *AIOTFName

	messageMeta
}

// relativeAMFCapacityBounds is RelativeAMFCapacity ::= INTEGER (0..255).
var relativeAMFCapacityBounds = per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true}

var nGSetupResponseIEs = []ieSpec[NGSetupResponse]{
	{
		ID: IDAMFName, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *NGSetupResponse, r *per.Reader, enc per.Encoding) error {
			var (
				err error
				n   Name
			)
			if err = decodeIE(r, &n); err == nil {
				m.AMFName = string(n)
			}

			return err
		},
		Encode: func(m *NGSetupResponse) (per.Marshaler, bool) { return Name(m.AMFName), true },
	},
	{
		ID: IDServedGUAMIList, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *NGSetupResponse, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ServedGUAMIList)
		},
		Encode: func(m *NGSetupResponse) (per.Marshaler, bool) { return m.ServedGUAMIList, true },
	},
	{
		ID: IDRelativeAMFCapacity, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *NGSetupResponse, r *per.Reader, enc per.Encoding) error {
			v, err := per.DecodeInteger(r, enc, relativeAMFCapacityBounds)
			if err != nil {
				return err
			}

			c := uint8(v)
			m.RelativeAMFCapacity = &c

			return nil
		},
		Encode: func(m *NGSetupResponse) (per.Marshaler, bool) {
			if m.RelativeAMFCapacity == nil {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return per.EncodeInteger(w, enc, relativeAMFCapacityBounds, int64(*m.RelativeAMFCapacity))
			}), true
		},
	},
	{
		ID: IDPLMNSupportList, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *NGSetupResponse, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PLMNSupportList)
		},
		Encode: func(m *NGSetupResponse) (per.Marshaler, bool) { return m.PLMNSupportList, true },
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *NGSetupResponse, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *NGSetupResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		ID: IDUERetentionInformation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *NGSetupResponse, r *per.Reader, enc per.Encoding) error {
			var v UERetentionInformation

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.UERetentionInformation = &v

			return nil
		},
		Encode: func(m *NGSetupResponse) (per.Marshaler, bool) {
			if m.UERetentionInformation == nil {
				return nil, false
			}

			return m.UERetentionInformation, true
		},
	},
	apimpl.OptionalIE(IDIABSupported, CriticalityIgnore, func(m *NGSetupResponse) **IABSupported { return &m.IABSupported }),
	{
		ID: IDExtendedAMFName, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *NGSetupResponse, r *per.Reader, enc per.Encoding) error {
			var v ExtendedAMFName

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.ExtendedAMFName = &v

			return nil
		},
		Encode: func(m *NGSetupResponse) (per.Marshaler, bool) {
			if m.ExtendedAMFName == nil {
				return nil, false
			}

			return m.ExtendedAMFName, true
		},
	},
	apimpl.OptionalIE(IDMobileIABSupported, CriticalityIgnore, func(m *NGSetupResponse) **MobileIABSupported { return &m.MobileIABSupported }),
	apimpl.OptionalIE(IDAIOTFIdentifier, CriticalityReject, func(m *NGSetupResponse) **AIOTFIdentifier { return &m.AIOTFIdentifier }),
	apimpl.OptionalIE(IDAIOTFName, CriticalityReject, func(m *NGSetupResponse) **AIOTFName { return &m.AIOTFName }),
}

func (m *NGSetupResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcNGSetup, nGSetupResponseIEs, m)
}

func (m *NGSetupResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcNGSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseNGSetupResponse(value []byte) (*NGSetupResponse, error) {
	return parseMessageBody[NGSetupResponse](ProcNGSetup, TriggeringSuccessfulOutcome, nGSetupResponseIEs, value)
}

func DecodeNGSetupResponse(value []byte) (*NGSetupResponse, error) {
	return decodeMessageBody[NGSetupResponse](ProcNGSetup, nGSetupResponseIEs, value)
}
