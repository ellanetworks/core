// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// TS 38.413 §9.2.3.10. The target NG-RAN node asks the AMF to switch the
// downlink user-plane path after an Xn handover. TS 36.413 §9.1.5.8 splits the
// cell and tracking area into separate EUTRAN-CGI and TAI IEs where NGAP
// carries one UserLocationInformation, and orders the session list second where
// NGAP orders it fifth.
type PathSwitchRequest struct {
	RANUENGAPID                          RANUENGAPID
	SourceAMFUENGAPID                    AMFUENGAPID
	UserLocationInformation              *UserLocationInformation
	UESecurityCapabilities               *UESecurityCapabilities
	PDUSessionResourceToBeSwitchedDLList PDUSessionResourceToBeSwitchedDLList
	PDUSessionResourceFailedToSetup      PDUSessionResourceFailedToSetupListPSReq

	messageMeta
}

var pathSwitchRequestIEs = []ieSpec[PathSwitchRequest]{
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PathSwitchRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *PathSwitchRequest) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: idSourceAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PathSwitchRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SourceAMFUENGAPID)
		},
		encode: func(m *PathSwitchRequest) (per.Marshaler, bool) { return &m.SourceAMFUENGAPID, true },
	},
	{
		id: idUserLocationInformation, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequest, raw []byte, enc per.Encoding) error {
			var v UserLocationInformation

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.UserLocationInformation = &v

			return nil
		},
		encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
	{
		id: idUESecurityCapabilities, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequest, raw []byte, enc per.Encoding) error {
			var v UESecurityCapabilities

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.UESecurityCapabilities = &v

			return nil
		},
		encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			if m.UESecurityCapabilities == nil {
				return nil, false
			}

			return m.UESecurityCapabilities, true
		},
	},
	{
		id: idPDUSessionResourceToBeSwitchedDLList, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PathSwitchRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceToBeSwitchedDLList)
		},
		encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			if m.PDUSessionResourceToBeSwitchedDLList == nil {
				return nil, false
			}

			return m.PDUSessionResourceToBeSwitchedDLList, true
		},
	},
	{
		id: idPDUSessionResourceFailedToSetupListPSReq, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceFailedToSetup)
		},
		encode: func(m *PathSwitchRequest) (per.Marshaler, bool) {
			if m.PDUSessionResourceFailedToSetup == nil {
				return nil, false
			}

			return m.PDUSessionResourceFailedToSetup, true
		},
	},
}

func (m *PathSwitchRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPathSwitchRequest, pathSwitchRequestIEs, m)
}

func (m *PathSwitchRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcPathSwitchRequest,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePathSwitchRequest(value []byte) (*PathSwitchRequest, error) {
	return parseMessageBody[PathSwitchRequest](ProcPathSwitchRequest, TriggeringInitiatingMessage, pathSwitchRequestIEs, value)
}

// TS 38.413 §9.2.3.11. The AMF confirms the switch and hands the target the
// {NH, NCC} pair for its next key derivation. TS 36.413 §9.1.5.9 carries a
// UEAggregateMaximumBitRate NGAP does not, and NGAP carries an AllowedNSSAI
// S1AP does not.
type PathSwitchRequestAcknowledge struct {
	AMFUENGAPID                    *AMFUENGAPID
	RANUENGAPID                    *RANUENGAPID
	UESecurityCapabilities         *UESecurityCapabilities
	SecurityContext                SecurityContext
	PDUSessionResourceSwitchedList PDUSessionResourceSwitchedList
	// Sessions the AMF could not switch, whose resources the NG-RAN node
	// releases (§8.4.4.2).
	PDUSessionResourceReleased PDUSessionResourceReleasedListPSAck
	AllowedNSSAI               AllowedNSSAI
	CriticalityDiagnostics     *CriticalityDiagnostics

	messageMeta
}

var pathSwitchRequestAcknowledgeIEs = []ieSpec[PathSwitchRequestAcknowledge]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
			var v RANUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		id: idUESecurityCapabilities, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
			var v UESecurityCapabilities

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.UESecurityCapabilities = &v

			return nil
		},
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.UESecurityCapabilities == nil {
				return nil, false
			}

			return m.UESecurityCapabilities, true
		},
	},
	{
		id: idSecurityContext, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SecurityContext)
		},
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) { return &m.SecurityContext, true },
	},
	{
		id: idPDUSessionResourceSwitchedList, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceSwitchedList)
		},
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.PDUSessionResourceSwitchedList == nil {
				return nil, false
			}

			return m.PDUSessionResourceSwitchedList, true
		},
	},
	{
		id: idPDUSessionResourceReleasedListPSAck, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceReleased)
		},
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.PDUSessionResourceReleased == nil {
				return nil, false
			}

			return m.PDUSessionResourceReleased, true
		},
	},
	{
		id: idAllowedNSSAI, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AllowedNSSAI)
		},
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.AllowedNSSAI == nil {
				return nil, false
			}

			return m.AllowedNSSAI, true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestAcknowledge, raw []byte, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		encode: func(m *PathSwitchRequestAcknowledge) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *PathSwitchRequestAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPathSwitchRequest, pathSwitchRequestAcknowledgeIEs, m)
}

func (m *PathSwitchRequestAcknowledge) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcPathSwitchRequest,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePathSwitchRequestAcknowledge(value []byte) (*PathSwitchRequestAcknowledge, error) {
	return parseMessageBody[PathSwitchRequestAcknowledge](ProcPathSwitchRequest, TriggeringSuccessfulOutcome, pathSwitchRequestAcknowledgeIEs, value)
}

// TS 38.413 §9.2.3.12. The AMF refuses the switch. Where TS 36.413 §9.1.5.10
// names one Cause for the whole message, NGAP reports per session: every
// session in the request is released, each with its own cause inside a
// PathSwitchRequestUnsuccessfulTransfer.
type PathSwitchRequestFailure struct {
	AMFUENGAPID                *AMFUENGAPID
	RANUENGAPID                *RANUENGAPID
	PDUSessionResourceReleased PDUSessionResourceReleasedListPSFail
	CriticalityDiagnostics     *CriticalityDiagnostics

	messageMeta
}

var pathSwitchRequestFailureIEs = []ieSpec[PathSwitchRequestFailure]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestFailure, raw []byte, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestFailure, raw []byte, enc per.Encoding) error {
			var v RANUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		id: idPDUSessionResourceReleasedListPSFail, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestFailure, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceReleased)
		},
		encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) {
			if m.PDUSessionResourceReleased == nil {
				return nil, false
			}

			return m.PDUSessionResourceReleased, true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PathSwitchRequestFailure, raw []byte, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		encode: func(m *PathSwitchRequestFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *PathSwitchRequestFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPathSwitchRequest, pathSwitchRequestFailureIEs, m)
}

func (m *PathSwitchRequestFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcPathSwitchRequest,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePathSwitchRequestFailure(value []byte) (*PathSwitchRequestFailure, error) {
	return parseMessageBody[PathSwitchRequestFailure](ProcPathSwitchRequest, TriggeringUnsuccessfulOutcome, pathSwitchRequestFailureIEs, value)
}
