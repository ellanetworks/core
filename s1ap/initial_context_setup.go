// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// InitialContextSetupRequest is the INITIAL CONTEXT SETUP REQUEST message
// (TS 36.413), sent by the MME to set up the UE context and default
// E-RAB(s). Unmodeled IEs are preserved.
type InitialContextSetupRequest struct {
	MMEUES1APID               MMEUES1APID
	ENBUES1APID               ENBUES1APID
	UEAggregateMaximumBitRate UEAggregateMaximumBitRate
	ERABToBeSetup             []ERABToBeSetupItemCtxtSUReq
	UESecurityCapabilities    UESecurityCapabilities
	SecurityKey               SecurityKey
	// UERadioCapability is the optional UE Radio Capability IE (TS 36.413);
	// when set, the eNB reuses it and skips re-fetching it from the
	// UE over the air (TS 23.401).
	UERadioCapability []byte

	unmodeledIEs
}

// initialContextSetupRequestIEs is the InitialContextSetupRequest IE table (TS 36.413).
var initialContextSetupRequestIEs = []ieSpec[InitialContextSetupRequest]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *InitialContextSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *InitialContextSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idUEAggregateMaximumBitrate, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *InitialContextSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UEAggregateMaximumBitRate)
		},
		encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return &m.UEAggregateMaximumBitRate, true },
	},
	{
		id: idERABToBeSetupListCtxtSUReq, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *InitialContextSetupRequest, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABToBeSetup, err = decodeItemList[ERABToBeSetupItemCtxtSUReq](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABToBeSetupItemCtxtSUReq, CriticalityReject, m.ERABToBeSetup)
			}), true
		},
	},
	{
		id: idUESecurityCapabilities, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *InitialContextSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UESecurityCapabilities)
		},
		encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return &m.UESecurityCapabilities, true },
	},
	{
		id: idSecurityKey, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *InitialContextSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SecurityKey)
		},
		encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return &m.SecurityKey, true },
	},
	{
		id: idUERadioCapability, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupRequest, raw []byte, enc per.Encoding) error {
			var err error

			m.UERadioCapability, err = per.DecodeOctetString(per.NewReader(raw), enc, 0, 0, true, false, false)

			return err
		},
		encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			if len(m.UERadioCapability) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return per.EncodeOctetString(w, enc, 0, 0, true, false, false, m.UERadioCapability)
			}), true
		},
	},
}

func (m *InitialContextSetupRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, initialContextSetupRequestIEs, m)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *InitialContextSetupRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcInitialContextSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseInitialContextSetupRequest decodes the message from an initiatingMessage
// open-type payload.
func ParseInitialContextSetupRequest(value []byte) (*InitialContextSetupRequest, error) {
	return parseMessageBody[InitialContextSetupRequest](ProcInitialContextSetup, initialContextSetupRequestIEs, value)
}

// InitialContextSetupResponse is the INITIAL CONTEXT SETUP RESPONSE message
// (TS 36.413), sent by the eNB once the E-RAB(s) are set up.
type InitialContextSetupResponse struct {
	MMEUES1APID            MMEUES1APID
	ENBUES1APID            ENBUES1APID
	ERABSetup              []ERABSetupItemCtxtSURes
	ERABFailedToSetup      []ERABItem
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

// initialContextSetupResponseIEs is the InitialContextSetupResponse IE table (TS 36.413).
var initialContextSetupResponseIEs = []ieSpec[InitialContextSetupResponse]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idERABSetupListCtxtSURes, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupResponse, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABSetup, err = decodeItemList[ERABSetupItemCtxtSURes](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) {
			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABSetupItemCtxtSURes, CriticalityIgnore, m.ERABSetup)
			}), true
		},
	},
	{
		id: idERABFailedToSetupListCtxtSU, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupResponse, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABFailedToSetup, err = decodeItemList[ERABItem](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) {
			if len(m.ERABFailedToSetup) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABItem, CriticalityIgnore, m.ERABFailedToSetup)
			}), true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupResponse, raw []byte, enc per.Encoding) error {
			var (
				err error
				cd  CriticalityDiagnostics
			)

			err = perIEDecode(raw, &cd)
			m.CriticalityDiagnostics = &cd

			return err
		},
		encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *InitialContextSetupResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, initialContextSetupResponseIEs, m)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *InitialContextSetupResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcInitialContextSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseInitialContextSetupResponse decodes the message from a successfulOutcome
// open-type payload.
func ParseInitialContextSetupResponse(value []byte) (*InitialContextSetupResponse, error) {
	return parseMessageBody[InitialContextSetupResponse](ProcInitialContextSetup, initialContextSetupResponseIEs, value)
}

// InitialContextSetupFailure is the INITIAL CONTEXT SETUP FAILURE message
// (TS 36.413).
type InitialContextSetupFailure struct {
	MMEUES1APID            MMEUES1APID
	ENBUES1APID            ENBUES1APID
	Cause                  Cause
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

// initialContextSetupFailureIEs is the InitialContextSetupFailure IE table (TS 36.413).
var initialContextSetupFailureIEs = []ieSpec[InitialContextSetupFailure]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupFailure, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *InitialContextSetupFailure) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupFailure, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *InitialContextSetupFailure) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idCause, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupFailure, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.Cause)
		},
		encode: func(m *InitialContextSetupFailure) (per.Marshaler, bool) { return &m.Cause, true },
	},
	{
		id: idCriticalityDiagnostics, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupFailure, raw []byte, enc per.Encoding) error {
			var (
				err error
				cd  CriticalityDiagnostics
			)

			err = perIEDecode(raw, &cd)
			m.CriticalityDiagnostics = &cd

			return err
		},
		encode: func(m *InitialContextSetupFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *InitialContextSetupFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, initialContextSetupFailureIEs, m)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *InitialContextSetupFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcInitialContextSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseInitialContextSetupFailure decodes the message from an
// unsuccessfulOutcome open-type payload.
func ParseInitialContextSetupFailure(value []byte) (*InitialContextSetupFailure, error) {
	return parseMessageBody[InitialContextSetupFailure](ProcInitialContextSetup, initialContextSetupFailureIEs, value)
}
