// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

var uEUsageTypeBounds = per.Bounds{LB: 0, UB: 255, HasLB: true, HasUB: true}

type S1Message []byte

func (s S1Message) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, s)
}

func (s *S1Message) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*s = append(S1Message{}, b...)

	return nil
}

type AdditionalGUTI struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	GUMMEI       GUMMEI
	MTMSI        MTMSI
	iEExtensions extensionMeta `per:",exttable"`
}

type RerouteNASRequest struct {
	ENBUES1APID    ENBUES1APID
	MMEUES1APID    *MMEUES1APID
	S1Message      S1Message
	MMEGroupID     MMEGroupID
	AdditionalGUTI *AdditionalGUTI
	UEUsageType    *uint8

	messageMeta
}

var rerouteNASRequestIEs = []ieSpec[RerouteNASRequest]{
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *RerouteNASRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *RerouteNASRequest) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDMMEUES1APID, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *RerouteNASRequest, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *RerouteNASRequest) (per.Marshaler, bool) { return optionalIE(m.MMEUES1APID) },
	},
	{
		ID: IDS1Message, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *RerouteNASRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.S1Message)
		},
		Encode: func(m *RerouteNASRequest) (per.Marshaler, bool) {
			if m.S1Message == nil {
				return nil, false
			}

			return m.S1Message, true
		},
	},
	{
		ID: IDMMEGroupID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *RerouteNASRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEGroupID)
		},
		Encode: func(m *RerouteNASRequest) (per.Marshaler, bool) { return &m.MMEGroupID, true },
	},
	{
		ID: IDAdditionalGUTI, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *RerouteNASRequest, r *per.Reader, enc per.Encoding) error {
			var v AdditionalGUTI

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AdditionalGUTI = &v

			return nil
		},
		Encode: func(m *RerouteNASRequest) (per.Marshaler, bool) {
			if m.AdditionalGUTI == nil {
				return nil, false
			}

			return m.AdditionalGUTI, true
		},
	},
	{
		ID: IDUEUsageType, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *RerouteNASRequest, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalInteger(r, uEUsageTypeBounds, &m.UEUsageType)
		},
		Encode: func(m *RerouteNASRequest) (per.Marshaler, bool) {
			if m.UEUsageType == nil {
				return nil, false
			}

			return integerIE(uEUsageTypeBounds, int64(*m.UEUsageType)), true
		},
	},
}

func (m *RerouteNASRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcRerouteNASRequest, rerouteNASRequestIEs, m)
}

func (m *RerouteNASRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcRerouteNASRequest,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseRerouteNASRequest(value []byte) (*RerouteNASRequest, error) {
	return parseMessageBody[RerouteNASRequest](ProcRerouteNASRequest, TriggeringInitiatingMessage, rerouteNASRequestIEs, value)
}

func DecodeRerouteNASRequest(value []byte) (*RerouteNASRequest, error) {
	return decodeMessageBody[RerouteNASRequest](ProcRerouteNASRequest, rerouteNASRequestIEs, value)
}
