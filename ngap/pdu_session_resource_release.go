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
	extensions   per.ExtensionAdditions `per:",extraw"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta `per:",exttable"`
}

// PDUSessionResourceToReleaseListRelCmd ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceToReleaseItemRelCmd.
type PDUSessionResourceToReleaseListRelCmd []PDUSessionResourceToReleaseItemRelCmd

// PDUSessionResourceReleasedItemRelRes ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceReleaseResponseTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceReleasedItemRelRes struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	iEExtensions extensionMeta `per:",exttable"`
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
	RANPagingPriority         *RANPagingPriority

	messageMeta
}

var pDUSessionResourceReleaseCommandIEs = []ieSpec[PDUSessionResourceReleaseCommand]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceReleaseCommand, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFUENGAPID)
		},
		Encode: func(m *PDUSessionResourceReleaseCommand) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceReleaseCommand, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *PDUSessionResourceReleaseCommand) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	optionalExtension(IDRANPagingPriority, CriticalityIgnore, func(m *PDUSessionResourceReleaseCommand) **RANPagingPriority { return &m.RANPagingPriority }),
	{
		ID: IDNASPDU, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceReleaseCommand, r *per.Reader, enc per.Encoding) error {
			var v NASPDU

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.NASPDU = &v

			return nil
		},
		Encode: func(m *PDUSessionResourceReleaseCommand) (per.Marshaler, bool) {
			if m.NASPDU == nil {
				return nil, false
			}

			return m.NASPDU, true
		},
	},
	{
		ID: IDPDUSessionResourceToReleaseListRelCmd, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PDUSessionResourceReleaseCommand, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceRelease)
		},
		Encode: func(m *PDUSessionResourceReleaseCommand) (per.Marshaler, bool) {
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

func DecodePDUSessionResourceReleaseCommand(value []byte) (*PDUSessionResourceReleaseCommand, error) {
	return decodeMessageBody[PDUSessionResourceReleaseCommand](ProcPDUSessionResourceRelease, pDUSessionResourceReleaseCommandIEs, value)
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
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceReleaseResponse, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		Encode: func(m *PDUSessionResourceReleaseResponse) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceReleaseResponse, r *per.Reader, enc per.Encoding) error {
			var v RANUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		Encode: func(m *PDUSessionResourceReleaseResponse) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		ID: IDPDUSessionResourceReleasedListRelRes, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceReleaseResponse, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceReleased)
		},
		Encode: func(m *PDUSessionResourceReleaseResponse) (per.Marshaler, bool) {
			if m.PDUSessionResourceReleased == nil {
				return nil, false
			}

			return m.PDUSessionResourceReleased, true
		},
	},
	{
		ID: IDUserLocationInformation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceReleaseResponse, r *per.Reader, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := decodeIE(r, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		Encode: func(m *PDUSessionResourceReleaseResponse) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PDUSessionResourceReleaseResponse, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *PDUSessionResourceReleaseResponse) (per.Marshaler, bool) {
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

func DecodePDUSessionResourceReleaseResponse(value []byte) (*PDUSessionResourceReleaseResponse, error) {
	return decodeMessageBody[PDUSessionResourceReleaseResponse](ProcPDUSessionResourceRelease, pDUSessionResourceReleaseResponseIEs, value)
}

// PDUSessionResourceReleaseCommandTransfer ::= SEQUENCE { cause, iE-Extensions
// OPTIONAL } (extensible) — TS 38.413 §9.3.4.12. The SMF builds it; the NG-RAN
// node consumes it.
type PDUSessionResourceReleaseCommandTransfer struct {
	extensions                per.ExtensionAdditions `per:",extraw"`
	Cause                     Cause
	QosFlowAdditionalInfoList QosFlowAdditionalInfoListRelCom `per:",extie:IDQosFlowAdditionalInfoList,crit:ignore"`
	iEExtensions              extensionMeta                   `per:",exttable"`
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
	return parseTransfer[PDUSessionResourceReleaseCommandTransfer](b)
}

func DecodePDUSessionResourceReleaseCommandTransfer(b TransferContainer) (*PDUSessionResourceReleaseCommandTransfer, error) {
	return decodeTransfer[PDUSessionResourceReleaseCommandTransfer](b)
}

func (t *PDUSessionResourceReleaseCommandTransfer) Diagnostics() Diagnostics { return diagnosticsOf(t) }

// PDUSessionResourceReleaseResponseTransfer ::= SEQUENCE { iE-Extensions
// OPTIONAL, ... } (extensible) — TS 38.413 §9.3.4.21. Every field is optional,
// so the NG-RAN node normally sends it empty; it exists so the AMF can relay a
// session's release confirmation to the SMF.
type PDUSessionResourceReleaseResponseTransfer struct {
	extensions                   per.ExtensionAdditions          `per:",extraw"`
	SecondaryRATUsageInformation *SecondaryRATUsageInformation   `per:",extie:IDSecondaryRATUsageInformation,crit:ignore"`
	QosFlowAdditionalInfoList    QosFlowAdditionalInfoListRelRes `per:",extie:IDQosFlowAdditionalInfoList,crit:ignore"`
	iEExtensions                 extensionMeta                   `per:",exttable"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *PDUSessionResourceReleaseResponseTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParsePDUSessionResourceReleaseResponseTransfer decodes the transfer the
// NG-RAN node returns for a released session.
func ParsePDUSessionResourceReleaseResponseTransfer(b TransferContainer) (*PDUSessionResourceReleaseResponseTransfer, error) {
	return parseTransfer[PDUSessionResourceReleaseResponseTransfer](b)
}

func DecodePDUSessionResourceReleaseResponseTransfer(b TransferContainer) (*PDUSessionResourceReleaseResponseTransfer, error) {
	return decodeTransfer[PDUSessionResourceReleaseResponseTransfer](b)
}

func (t *PDUSessionResourceReleaseResponseTransfer) Diagnostics() Diagnostics {
	return diagnosticsOf(t)
}
