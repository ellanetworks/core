// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// PDUSessionResourceToReleaseItemRelCmd ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceReleaseCommandTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceToReleaseItemRelCmd struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceToReleaseListRelCmd ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceToReleaseItemRelCmd.
type PDUSessionResourceToReleaseListRelCmd []PDUSessionResourceToReleaseItemRelCmd

// PDUSessionResourceReleasedItemRelRes ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceReleaseResponseTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceReleasedItemRelRes struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceReleasedListRelRes ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceReleasedItemRelRes.
type PDUSessionResourceReleasedListRelRes []PDUSessionResourceReleasedItemRelRes

// TS 38.413 §9.2.1.3.
type PDUSessionResourceReleaseCommand struct {
	AMFUENGAPID               AMFUENGAPID
	RANUENGAPID               RANUENGAPID
	NASPDU                    *NASPDU
	PDUSessionResourceRelease PDUSessionResourceToReleaseListRelCmd

	messageMeta
}

var pDUSessionResourceReleaseCommandIEs = []ieSpec[PDUSessionResourceReleaseCommand]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceReleaseCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *PDUSessionResourceReleaseCommand) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceReleaseCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *PDUSessionResourceReleaseCommand) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: idNASPDU, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceReleaseCommand, raw []byte, enc per.Encoding) error {
			var v NASPDU

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.NASPDU = &v

			return nil
		},
		encode: func(m *PDUSessionResourceReleaseCommand) (per.Marshaler, bool) {
			if m.NASPDU == nil {
				return nil, false
			}

			return m.NASPDU, true
		},
	},
	{
		id: idPDUSessionResourceToReleaseListRelCmd, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceReleaseCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceRelease)
		},
		encode: func(m *PDUSessionResourceReleaseCommand) (per.Marshaler, bool) {
			if m.PDUSessionResourceRelease == nil {
				return nil, false
			}

			return m.PDUSessionResourceRelease, true
		},
	},
}

func (m *PDUSessionResourceReleaseCommand) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPDUSessionResourceRelease, pDUSessionResourceReleaseCommandIEs, m)
}

func (m *PDUSessionResourceReleaseCommand) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcPDUSessionResourceRelease,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePDUSessionResourceReleaseCommand(value []byte) (*PDUSessionResourceReleaseCommand, error) {
	return parseMessageBody[PDUSessionResourceReleaseCommand](ProcPDUSessionResourceRelease, TriggeringInitiatingMessage, pDUSessionResourceReleaseCommandIEs, value)
}

// TS 38.413 §9.2.1.4.
type PDUSessionResourceReleaseResponse struct {
	AMFUENGAPID                *AMFUENGAPID
	RANUENGAPID                *RANUENGAPID
	PDUSessionResourceReleased PDUSessionResourceReleasedListRelRes
	UserLocationInformation    *UserLocationInformation
	CriticalityDiagnostics     *CriticalityDiagnostics

	messageMeta
}

var pDUSessionResourceReleaseResponseIEs = []ieSpec[PDUSessionResourceReleaseResponse]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceReleaseResponse, raw []byte, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		encode: func(m *PDUSessionResourceReleaseResponse) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceReleaseResponse, raw []byte, enc per.Encoding) error {
			var v RANUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		encode: func(m *PDUSessionResourceReleaseResponse) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		id: idPDUSessionResourceReleasedListRelRes, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceReleaseResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceReleased)
		},
		encode: func(m *PDUSessionResourceReleaseResponse) (per.Marshaler, bool) {
			if m.PDUSessionResourceReleased == nil {
				return nil, false
			}

			return m.PDUSessionResourceReleased, true
		},
	},
	{
		id: idUserLocationInformation, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceReleaseResponse, raw []byte, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := perIEDecode(raw, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		encode: func(m *PDUSessionResourceReleaseResponse) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceReleaseResponse, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *PDUSessionResourceReleaseResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *PDUSessionResourceReleaseResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPDUSessionResourceRelease, pDUSessionResourceReleaseResponseIEs, m)
}

func (m *PDUSessionResourceReleaseResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcPDUSessionResourceRelease,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePDUSessionResourceReleaseResponse(value []byte) (*PDUSessionResourceReleaseResponse, error) {
	return parseMessageBody[PDUSessionResourceReleaseResponse](ProcPDUSessionResourceRelease, TriggeringSuccessfulOutcome, pDUSessionResourceReleaseResponseIEs, value)
}

// PDUSessionResourceReleaseCommandTransfer ::= SEQUENCE { cause, iE-Extensions
// OPTIONAL } (extensible) — TS 38.413 §9.3.4.13. The SMF builds it; the NG-RAN
// node consumes it.
type PDUSessionResourceReleaseCommandTransfer struct {
	_     [0]struct{} `per:"extseq"`
	Cause Cause
	_     ieExtensions `per:",skip"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *PDUSessionResourceReleaseCommandTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParsePDUSessionResourceReleaseCommandTransfer decodes the transfer the SMF
// sends to release a session.
func ParsePDUSessionResourceReleaseCommandTransfer(b TransferContainer) (*PDUSessionResourceReleaseCommandTransfer, error) {
	var t PDUSessionResourceReleaseCommandTransfer

	if err := t.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, err
	}

	return &t, nil
}
