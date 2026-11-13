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

	*s = b

	return nil
}

type AdditionalGUTI struct {
	_      [0]struct{} `per:"extseq"`
	GUMMEI GUMMEI
	MTMSI  MTMSI
	_      ieExtensions `per:",skip"`
}

// TS 36.413 §9.1.7.5.
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
		id: IDENBUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *RerouteNASRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *RerouteNASRequest) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: IDMMEUES1APID, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *RerouteNASRequest, raw []byte, enc per.Encoding) error {
			return decodeOptionalIE(raw, &m.MMEUES1APID)
		},
		encode: func(m *RerouteNASRequest) (per.Marshaler, bool) { return optionalIE(m.MMEUES1APID) },
	},
	{
		id: IDS1Message, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *RerouteNASRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.S1Message)
		},
		encode: func(m *RerouteNASRequest) (per.Marshaler, bool) {
			if m.S1Message == nil {
				return nil, false
			}

			return m.S1Message, true
		},
	},
	{
		id: IDMMEGroupID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *RerouteNASRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEGroupID)
		},
		encode: func(m *RerouteNASRequest) (per.Marshaler, bool) { return &m.MMEGroupID, true },
	},
	{
		id: IDAdditionalGUTI, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *RerouteNASRequest, raw []byte, enc per.Encoding) error {
			var v AdditionalGUTI

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AdditionalGUTI = &v

			return nil
		},
		encode: func(m *RerouteNASRequest) (per.Marshaler, bool) {
			if m.AdditionalGUTI == nil {
				return nil, false
			}

			return m.AdditionalGUTI, true
		},
	},
	{
		id: IDUEUsageType, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *RerouteNASRequest, raw []byte, enc per.Encoding) error {
			return decodeOptionalInteger(raw, uEUsageTypeBounds, &m.UEUsageType)
		},
		encode: func(m *RerouteNASRequest) (per.Marshaler, bool) {
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
