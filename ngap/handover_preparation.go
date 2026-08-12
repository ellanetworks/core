// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// TS 38.413 §9.2.3.1. The source NG-RAN node asks the AMF to prepare a
// handover. TS 36.413 §9.1.5.1 shares seven of these IEs but adds seven more for
// circuit-switched fallback and legacy femtocells (SRVCC, a secondary source
// container, MSClassmark2/3, CSG-Id, CellAccessMode, PS-ServiceNotAvailable),
// none of which 5G defines; NGAP adds PDUSessionResourceListHORqd in their
// place.
type HandoverRequired struct {
	AMFUENGAPID                        AMFUENGAPID
	RANUENGAPID                        RANUENGAPID
	HandoverType                       HandoverType
	Cause                              *Cause
	TargetID                           TargetID
	DirectForwardingPathAvailability   *DirectForwardingPathAvailability
	PDUSessionResourceListHORqd        PDUSessionResourceListHORqd
	SourceToTargetTransparentContainer SourceToTargetTransparentContainer

	messageMeta
}

var handoverRequiredIEs = []ieSpec[HandoverRequired]{
	{
		id: IDAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: IDRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: IDHandoverType, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.HandoverType)
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) { return &m.HandoverType, true },
	},
	{
		id: IDCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: IDTargetID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.TargetID)
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) { return &m.TargetID, true },
	},
	{
		id: IDDirectForwardingPathAvailability, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			var v DirectForwardingPathAvailability

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.DirectForwardingPathAvailability = &v

			return nil
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) {
			if m.DirectForwardingPathAvailability == nil {
				return nil, false
			}

			return m.DirectForwardingPathAvailability, true
		},
	},
	{
		id: IDPDUSessionResourceListHORqd, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceListHORqd)
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) {
			if m.PDUSessionResourceListHORqd == nil {
				return nil, false
			}

			return m.PDUSessionResourceListHORqd, true
		},
	},
	{
		id: IDSourceToTargetTransparentContainer, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SourceToTargetTransparentContainer)
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) {
			if m.SourceToTargetTransparentContainer == nil {
				return nil, false
			}

			return m.SourceToTargetTransparentContainer, true
		},
	},
}

func (m *HandoverRequired) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverPreparation, handoverRequiredIEs, m)
}

func (m *HandoverRequired) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcHandoverPreparation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseHandoverRequired(value []byte) (*HandoverRequired, error) {
	return parseMessageBody[HandoverRequired](ProcHandoverPreparation, TriggeringInitiatingMessage, handoverRequiredIEs, value)
}

// TS 38.413 §9.2.3.2. The AMF tells the source NG-RAN node that the target has
// resources ready. TS 36.413 §9.1.5.2 carries the same shape: the NAS security
// parameters, the two per-bearer lists and the target container, differing only
// in that S1AP has a second target container for SRVCC, which is not modelled.
type HandoverCommand struct {
	AMFUENGAPID                        AMFUENGAPID
	RANUENGAPID                        RANUENGAPID
	HandoverType                       HandoverType
	NASSecurityParametersFromNGRAN     NASSecurityParametersFromNGRAN
	PDUSessionResourceHandoverList     PDUSessionResourceHandoverList
	PDUSessionResourceToReleaseList    PDUSessionResourceToReleaseListHOCmd
	TargetToSourceTransparentContainer TargetToSourceTransparentContainer
	CriticalityDiagnostics             *CriticalityDiagnostics

	messageMeta
}

// The NAS security parameters travel only when the UE leaves 5GS: TS 38.413
// §9.2.3.2 condition iftoEPSUTRA. HandoverType's fivegs-to-utran is an
// extension value this package does not model, so 5GStoEPS is the whole of it.
func handoverLeavesFiveGS(m *HandoverCommand) bool {
	return m.HandoverType == HandoverTypeFiveGSToEPS
}

var handoverCommandIEs = []ieSpec[HandoverCommand]{
	{
		id: IDAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: IDRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: IDHandoverType, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.HandoverType)
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) { return &m.HandoverType, true },
	},
	{
		id: IDNASSecurityParametersFromNGRAN, presence: presenceConditional, crit: CriticalityReject,
		condition: handoverLeavesFiveGS,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.NASSecurityParametersFromNGRAN)
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) {
			if m.NASSecurityParametersFromNGRAN == nil {
				return nil, false
			}

			return m.NASSecurityParametersFromNGRAN, true
		},
	},
	{
		id: IDPDUSessionResourceHandoverList, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceHandoverList)
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) {
			if m.PDUSessionResourceHandoverList == nil {
				return nil, false
			}

			return m.PDUSessionResourceHandoverList, true
		},
	},
	{
		id: IDPDUSessionResourceToReleaseListHOCmd, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceToReleaseList)
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) {
			if m.PDUSessionResourceToReleaseList == nil {
				return nil, false
			}

			return m.PDUSessionResourceToReleaseList, true
		},
	},
	{
		id: IDTargetToSourceTransparentContainer, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.TargetToSourceTransparentContainer)
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) {
			if m.TargetToSourceTransparentContainer == nil {
				return nil, false
			}

			return m.TargetToSourceTransparentContainer, true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *HandoverCommand) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverPreparation, handoverCommandIEs, m)
}

func (m *HandoverCommand) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcHandoverPreparation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseHandoverCommand(value []byte) (*HandoverCommand, error) {
	return parseMessageBody[HandoverCommand](ProcHandoverPreparation, TriggeringSuccessfulOutcome, handoverCommandIEs, value)
}

// TS 38.413 §9.2.3.3. TS 36.413 §9.1.5.3 carries the same four IEs with the
// same criticality and presence. NGAP adds the target's failure container,
// which reaches the AMF in HANDOVER FAILURE and which §8.4.1.3 has the AMF pass
// on to the source NG-RAN node.
type HandoverPreparationFailure struct {
	AMFUENGAPID                               *AMFUENGAPID
	RANUENGAPID                               *RANUENGAPID
	Cause                                     *Cause
	CriticalityDiagnostics                    *CriticalityDiagnostics
	TargettoSourceFailureTransparentContainer TargettoSourceFailureTransparentContainer

	messageMeta
}

var handoverPreparationFailureIEs = []ieSpec[HandoverPreparationFailure]{
	{
		id: IDAMFUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverPreparationFailure, raw []byte, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		encode: func(m *HandoverPreparationFailure) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		id: IDRANUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverPreparationFailure, raw []byte, enc per.Encoding) error {
			var v RANUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		encode: func(m *HandoverPreparationFailure) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		id: IDCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverPreparationFailure, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *HandoverPreparationFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *HandoverPreparationFailure, raw []byte, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		encode: func(m *HandoverPreparationFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		id: IDTargettoSourceFailureTransparentContainer, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *HandoverPreparationFailure, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.TargettoSourceFailureTransparentContainer)
		},
		encode: func(m *HandoverPreparationFailure) (per.Marshaler, bool) {
			if m.TargettoSourceFailureTransparentContainer == nil {
				return nil, false
			}

			return m.TargettoSourceFailureTransparentContainer, true
		},
	},
}

func (m *HandoverPreparationFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverPreparation, handoverPreparationFailureIEs, m)
}

func (m *HandoverPreparationFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcHandoverPreparation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseHandoverPreparationFailure(value []byte) (*HandoverPreparationFailure, error) {
	return parseMessageBody[HandoverPreparationFailure](ProcHandoverPreparation, TriggeringUnsuccessfulOutcome, handoverPreparationFailureIEs, value)
}
