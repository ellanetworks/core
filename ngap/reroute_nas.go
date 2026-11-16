// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "github.com/ellanetworks/core/per"

const maxnoofPartiallyAllowedSNSSAIs = 8

const (
	configuredNSSAILen = 128
	rejectedNSSAILen   = 32
)

type NGAPMessage []byte

func (n NGAPMessage) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, n)
}

func (n *NGAPMessage) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*n = b

	return nil
}

type ConfiguredNSSAI [configuredNSSAILen]byte

func (c ConfiguredNSSAI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, configuredNSSAILen, configuredNSSAILen, true, true, false, c[:])
}

func (c *ConfiguredNSSAI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, configuredNSSAILen, configuredNSSAILen, true, true, false)
	if err != nil {
		return err
	}

	copy(c[:], b)

	return nil
}

type RejectedNSSAIinPLMN [rejectedNSSAILen]byte

func (n RejectedNSSAIinPLMN) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, rejectedNSSAILen, rejectedNSSAILen, true, true, false, n[:])
}

func (n *RejectedNSSAIinPLMN) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, rejectedNSSAILen, rejectedNSSAILen, true, true, false)
	if err != nil {
		return err
	}

	copy(n[:], b)

	return nil
}

type RejectedNSSAIinTA [rejectedNSSAILen]byte

func (n RejectedNSSAIinTA) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, rejectedNSSAILen, rejectedNSSAILen, true, true, false, n[:])
}

func (n *RejectedNSSAIinTA) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, rejectedNSSAILen, rejectedNSSAILen, true, true, false)
	if err != nil {
		return err
	}

	copy(n[:], b)

	return nil
}

type SourceToTargetAMFInformationReroute struct {
	extensions          per.ExtensionAdditions `per:",extraw"`
	ConfiguredNSSAI     *ConfiguredNSSAI       `per:",optional"`
	RejectedNSSAIinPLMN *RejectedNSSAIinPLMN   `per:",optional"`
	RejectedNSSAIinTA   *RejectedNSSAIinTA     `per:",optional"`
	iEExtensions        extensionMeta          `per:",exttable"`
}

type PartiallyAllowedNSSAIItem struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	SNSSAI       SNSSAI
	iEExtensions extensionMeta `per:",exttable"`
}

type PartiallyAllowedNSSAI []PartiallyAllowedNSSAIItem

func (p PartiallyAllowedNSSAI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPartiallyAllowedSNSSAIs, []PartiallyAllowedNSSAIItem(p))
}

func (p *PartiallyAllowedNSSAI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PartiallyAllowedNSSAIItem](r, enc, 1, maxnoofPartiallyAllowedSNSSAIs)
	if err != nil {
		return err
	}

	*p = items

	return nil
}

type RerouteNASRequest struct {
	RANUENGAPID                         RANUENGAPID
	AMFUENGAPID                         *AMFUENGAPID
	NGAPMessage                         NGAPMessage
	AMFSetID                            AMFSetID
	AllowedNSSAI                        AllowedNSSAI
	SourceToTargetAMFInformationReroute *SourceToTargetAMFInformationReroute
	PartiallyAllowedNSSAI               PartiallyAllowedNSSAI

	messageMeta
}

var rerouteNASRequestIEs = []ieSpec[RerouteNASRequest]{
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *RerouteNASRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *RerouteNASRequest) (per.Marshaler, bool) { return m.RANUENGAPID, true },
	},
	{
		ID: IDAMFUENGAPID, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *RerouteNASRequest, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		Encode: func(m *RerouteNASRequest) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		ID: IDNGAPMessage, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *RerouteNASRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.NGAPMessage)
		},
		Encode: func(m *RerouteNASRequest) (per.Marshaler, bool) { return m.NGAPMessage, true },
	},
	{
		ID: IDAMFSetID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *RerouteNASRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFSetID)
		},
		Encode: func(m *RerouteNASRequest) (per.Marshaler, bool) { return m.AMFSetID, true },
	},
	{
		ID: IDAllowedNSSAI, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *RerouteNASRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AllowedNSSAI)
		},
		Encode: func(m *RerouteNASRequest) (per.Marshaler, bool) {
			if len(m.AllowedNSSAI) == 0 {
				return nil, false
			}

			return m.AllowedNSSAI, true
		},
	},
	{
		ID: IDSourceToTargetAMFInformationReroute, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *RerouteNASRequest, r *per.Reader, enc per.Encoding) error {
			var v SourceToTargetAMFInformationReroute

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.SourceToTargetAMFInformationReroute = &v

			return nil
		},
		Encode: func(m *RerouteNASRequest) (per.Marshaler, bool) {
			if m.SourceToTargetAMFInformationReroute == nil {
				return nil, false
			}

			return m.SourceToTargetAMFInformationReroute, true
		},
	},
	{
		ID: IDPartiallyAllowedNSSAI, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *RerouteNASRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PartiallyAllowedNSSAI)
		},
		Encode: func(m *RerouteNASRequest) (per.Marshaler, bool) {
			if len(m.PartiallyAllowedNSSAI) == 0 {
				return nil, false
			}

			return m.PartiallyAllowedNSSAI, true
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
