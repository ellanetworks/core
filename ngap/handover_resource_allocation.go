// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// TS 38.413 §9.2.3.4. The AMF asks the target NG-RAN node to reserve resources.
// Only the mandatory IEs are modelled, as in TS 36.413 §9.1.5.4; NGAP adds
// AllowedNSSAI and GUAMI, both 5G-only, and orders the security IEs before the
// session list where S1AP orders them after.
type HandoverRequest struct {
	AMFUENGAPID               AMFUENGAPID
	HandoverType              HandoverType
	Cause                     *Cause
	UEAggregateMaximumBitRate UEAggregateMaximumBitRate
	UESecurityCapabilities    UESecurityCapabilities
	SecurityContext           SecurityContext

	// The two elements an EPS to 5GS handover adds. NewSecurityContextInd tells
	// the target the context is one the UE has not used yet, and NASC carries the
	// S1 mode to N1 mode NAS transparent container the target embeds in the
	// Target-to-Source container for the UE (TS 33.501 §8.4.2, TS 24.501
	// §9.11.2.9). Both are optional in the ASN.1 and neither is optional in
	// practice for that direction: without them the UE cannot take the mapped 5G
	// context into use.
	NewSecurityContextInd *NewSecurityContextInd
	NASC                  NASPDU

	PDUSessionResourceSetupListHOReq   PDUSessionResourceSetupListHOReq
	AllowedNSSAI                       AllowedNSSAI
	SourceToTargetTransparentContainer SourceToTargetTransparentContainer

	// MobilityRestrictionList is optional in the ASN.1 and not optional in
	// practice: TS 38.413 §8.4.2.4 has the target NG-RAN node reject the handover
	// when the list is absent and it cannot determine the serving PLMN otherwise.
	MobilityRestrictionList *MobilityRestrictionList

	GUAMI GUAMI

	messageMeta
}

var handoverRequestIEs = []ieSpec[HandoverRequest]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: idHandoverType, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.HandoverType)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.HandoverType, true },
	},
	{
		id: idCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: idUEAggregateMaximumBitRate, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UEAggregateMaximumBitRate)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.UEAggregateMaximumBitRate, true },
	},
	{
		id: idUESecurityCapabilities, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UESecurityCapabilities)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.UESecurityCapabilities, true },
	},
	{
		id: idSecurityContext, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SecurityContext)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.SecurityContext, true },
	},
	{
		id: idNewSecurityContextInd, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			var v NewSecurityContextInd

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.NewSecurityContextInd = &v

			return nil
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.NewSecurityContextInd == nil {
				return nil, false
			}

			return m.NewSecurityContextInd, true
		},
	},
	{
		id: idNASC, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.NASC)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.NASC == nil {
				return nil, false
			}

			return m.NASC, true
		},
	},
	{
		id: idPDUSessionResourceSetupListHOReq, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceSetupListHOReq)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.PDUSessionResourceSetupListHOReq == nil {
				return nil, false
			}

			return m.PDUSessionResourceSetupListHOReq, true
		},
	},
	{
		id: idAllowedNSSAI, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AllowedNSSAI)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.AllowedNSSAI == nil {
				return nil, false
			}

			return m.AllowedNSSAI, true
		},
	},
	{
		id: idSourceToTargetTransparentContainer, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SourceToTargetTransparentContainer)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.SourceToTargetTransparentContainer == nil {
				return nil, false
			}

			return m.SourceToTargetTransparentContainer, true
		},
	},
	{
		id: idMobilityRestrictionList, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			var v MobilityRestrictionList

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.MobilityRestrictionList = &v

			return nil
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.MobilityRestrictionList == nil {
				return nil, false
			}

			return m.MobilityRestrictionList, true
		},
	},
	{
		id: idGUAMI, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.GUAMI)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.GUAMI, true },
	},
}

func (m *HandoverRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverResourceAllocation, handoverRequestIEs, m)
}

func (m *HandoverRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcHandoverResourceAllocation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseHandoverRequest(value []byte) (*HandoverRequest, error) {
	return parseMessageBody[HandoverRequest](ProcHandoverResourceAllocation, TriggeringInitiatingMessage, handoverRequestIEs, value)
}

// TS 38.413 §9.2.3.5. The target NG-RAN node reports what it admitted.
// TS 36.413 §9.1.5.5 carries the same five IEs plus Criticality Diagnostics.
type HandoverRequestAcknowledge struct {
	AMFUENGAPID                        *AMFUENGAPID
	RANUENGAPID                        *RANUENGAPID
	PDUSessionResourceAdmittedList     PDUSessionResourceAdmittedList
	PDUSessionResourceFailedToSetup    PDUSessionResourceFailedToSetupListHOAck
	TargetToSourceTransparentContainer TargetToSourceTransparentContainer
	CriticalityDiagnostics             *CriticalityDiagnostics

	messageMeta
}

var handoverRequestAcknowledgeIEs = []ieSpec[HandoverRequestAcknowledge]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverRequestAcknowledge, raw []byte, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverRequestAcknowledge, raw []byte, enc per.Encoding) error {
			var v RANUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		id: idPDUSessionResourceAdmittedList, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverRequestAcknowledge, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceAdmittedList)
		},
		encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.PDUSessionResourceAdmittedList == nil {
				return nil, false
			}

			return m.PDUSessionResourceAdmittedList, true
		},
	},
	{
		id: idPDUSessionResourceFailedToSetupListHOAck, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *HandoverRequestAcknowledge, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceFailedToSetup)
		},
		encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.PDUSessionResourceFailedToSetup == nil {
				return nil, false
			}

			return m.PDUSessionResourceFailedToSetup, true
		},
	},
	{
		id: idTargetToSourceTransparentContainer, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequestAcknowledge, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.TargetToSourceTransparentContainer)
		},
		encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.TargetToSourceTransparentContainer == nil {
				return nil, false
			}

			return m.TargetToSourceTransparentContainer, true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *HandoverRequestAcknowledge, raw []byte, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *HandoverRequestAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverResourceAllocation, handoverRequestAcknowledgeIEs, m)
}

func (m *HandoverRequestAcknowledge) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcHandoverResourceAllocation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseHandoverRequestAcknowledge(value []byte) (*HandoverRequestAcknowledge, error) {
	return parseMessageBody[HandoverRequestAcknowledge](ProcHandoverResourceAllocation, TriggeringSuccessfulOutcome, handoverRequestAcknowledgeIEs, value)
}

// TS 38.413 §9.2.3.6. The target NG-RAN node admitted nothing and holds no UE
// context. TS 36.413 §9.1.5.6 carries the same three IEs; NGAP adds the failure
// container, which has no S1AP counterpart.
type HandoverFailure struct {
	AMFUENGAPID                               *AMFUENGAPID
	Cause                                     *Cause
	CriticalityDiagnostics                    *CriticalityDiagnostics
	TargettoSourceFailureTransparentContainer TargettoSourceFailureTransparentContainer

	messageMeta
}

var handoverFailureIEs = []ieSpec[HandoverFailure]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverFailure, raw []byte, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		encode: func(m *HandoverFailure) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		id: idCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverFailure, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *HandoverFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *HandoverFailure, raw []byte, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		encode: func(m *HandoverFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		id: idTargettoSourceFailureTransparentContainer, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *HandoverFailure, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.TargettoSourceFailureTransparentContainer)
		},
		encode: func(m *HandoverFailure) (per.Marshaler, bool) {
			if m.TargettoSourceFailureTransparentContainer == nil {
				return nil, false
			}

			return m.TargettoSourceFailureTransparentContainer, true
		},
	},
}

func (m *HandoverFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverResourceAllocation, handoverFailureIEs, m)
}

func (m *HandoverFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcHandoverResourceAllocation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseHandoverFailure(value []byte) (*HandoverFailure, error) {
	return parseMessageBody[HandoverFailure](ProcHandoverResourceAllocation, TriggeringUnsuccessfulOutcome, handoverFailureIEs, value)
}
