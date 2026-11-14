// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "github.com/ellanetworks/core/per"

type WriteReplaceWarningRequest struct {
	MessageIdentifier           MessageIdentifier
	SerialNumber                SerialNumber
	WarningAreaList             *WarningAreaList
	RepetitionPeriod            RepetitionPeriod
	NumberOfBroadcastsRequested NumberOfBroadcastsRequested
	WarningType                 *WarningType
	WarningSecurityInfo         *WarningSecurityInfo
	DataCodingScheme            *DataCodingScheme
	WarningMessageContents      WarningMessageContents
	ConcurrentWarningMessageInd *ConcurrentWarningMessageInd
	WarningAreaCoordinates      WarningAreaCoordinates

	messageMeta
}

var writeReplaceWarningRequestIEs = []ieSpec[WriteReplaceWarningRequest]{
	{
		id: IDMessageIdentifier, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MessageIdentifier)
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return m.MessageIdentifier, true },
	},
	{
		id: IDSerialNumber, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SerialNumber)
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return m.SerialNumber, true },
	},
	{
		id: IDWarningAreaList, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			var v WarningAreaList

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.WarningAreaList = &v

			return nil
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			if m.WarningAreaList == nil {
				return nil, false
			}

			return m.WarningAreaList, true
		},
	},
	{
		id: IDRepetitionPeriod, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RepetitionPeriod)
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return m.RepetitionPeriod, true },
	},
	{
		id: IDNumberOfBroadcastsRequested, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.NumberOfBroadcastsRequested)
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return m.NumberOfBroadcastsRequested, true },
	},
	{
		id: IDWarningType, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			var v WarningType

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.WarningType = &v

			return nil
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			if m.WarningType == nil {
				return nil, false
			}

			return m.WarningType, true
		},
	},
	{
		id: IDWarningSecurityInfo, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			var v WarningSecurityInfo

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.WarningSecurityInfo = &v

			return nil
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			if m.WarningSecurityInfo == nil {
				return nil, false
			}

			return m.WarningSecurityInfo, true
		},
	},
	{
		id: IDDataCodingScheme, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			var v DataCodingScheme

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.DataCodingScheme = &v

			return nil
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			if m.DataCodingScheme == nil {
				return nil, false
			}

			return m.DataCodingScheme, true
		},
	},
	{
		id: IDWarningMessageContents, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.WarningMessageContents)
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			if len(m.WarningMessageContents) == 0 {
				return nil, false
			}

			return m.WarningMessageContents, true
		},
	},
	{
		id: IDConcurrentWarningMessageInd, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			var v ConcurrentWarningMessageInd

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.ConcurrentWarningMessageInd = &v

			return nil
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			if m.ConcurrentWarningMessageInd == nil {
				return nil, false
			}

			return m.ConcurrentWarningMessageInd, true
		},
	},
	{
		id: IDWarningAreaCoordinates, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.WarningAreaCoordinates)
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			if len(m.WarningAreaCoordinates) == 0 {
				return nil, false
			}

			return m.WarningAreaCoordinates, true
		},
	},
}

func (m *WriteReplaceWarningRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcWriteReplaceWarning, writeReplaceWarningRequestIEs, m)
}

func (m *WriteReplaceWarningRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcWriteReplaceWarning,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseWriteReplaceWarningRequest(value []byte) (*WriteReplaceWarningRequest, error) {
	return parseMessageBody[WriteReplaceWarningRequest](ProcWriteReplaceWarning, TriggeringInitiatingMessage, writeReplaceWarningRequestIEs, value)
}

type WriteReplaceWarningResponse struct {
	MessageIdentifier          MessageIdentifier
	SerialNumber               SerialNumber
	BroadcastCompletedAreaList *BroadcastCompletedAreaList
	CriticalityDiagnostics     *CriticalityDiagnostics

	messageMeta
}

var writeReplaceWarningResponseIEs = []ieSpec[WriteReplaceWarningResponse]{
	{
		id: IDMessageIdentifier, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *WriteReplaceWarningResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MessageIdentifier)
		},
		encode: func(m *WriteReplaceWarningResponse) (per.Marshaler, bool) { return m.MessageIdentifier, true },
	},
	{
		id: IDSerialNumber, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *WriteReplaceWarningResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SerialNumber)
		},
		encode: func(m *WriteReplaceWarningResponse) (per.Marshaler, bool) { return m.SerialNumber, true },
	},
	{
		id: IDBroadcastCompletedAreaList, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *WriteReplaceWarningResponse, raw []byte, enc per.Encoding) error {
			var v BroadcastCompletedAreaList

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.BroadcastCompletedAreaList = &v

			return nil
		},
		encode: func(m *WriteReplaceWarningResponse) (per.Marshaler, bool) {
			if m.BroadcastCompletedAreaList == nil {
				return nil, false
			}

			return m.BroadcastCompletedAreaList, true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *WriteReplaceWarningResponse, raw []byte, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		encode: func(m *WriteReplaceWarningResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *WriteReplaceWarningResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcWriteReplaceWarning, writeReplaceWarningResponseIEs, m)
}

func (m *WriteReplaceWarningResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcWriteReplaceWarning,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseWriteReplaceWarningResponse(value []byte) (*WriteReplaceWarningResponse, error) {
	return parseMessageBody[WriteReplaceWarningResponse](ProcWriteReplaceWarning, TriggeringSuccessfulOutcome, writeReplaceWarningResponseIEs, value)
}

type PWSCancelRequest struct {
	MessageIdentifier        MessageIdentifier
	SerialNumber             SerialNumber
	WarningAreaList          *WarningAreaList
	CancelAllWarningMessages *CancelAllWarningMessages

	messageMeta
}

var pWSCancelRequestIEs = []ieSpec[PWSCancelRequest]{
	{
		id: IDMessageIdentifier, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PWSCancelRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MessageIdentifier)
		},
		encode: func(m *PWSCancelRequest) (per.Marshaler, bool) { return m.MessageIdentifier, true },
	},
	{
		id: IDSerialNumber, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PWSCancelRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SerialNumber)
		},
		encode: func(m *PWSCancelRequest) (per.Marshaler, bool) { return m.SerialNumber, true },
	},
	{
		id: IDWarningAreaList, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PWSCancelRequest, raw []byte, enc per.Encoding) error {
			var v WarningAreaList

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.WarningAreaList = &v

			return nil
		},
		encode: func(m *PWSCancelRequest) (per.Marshaler, bool) {
			if m.WarningAreaList == nil {
				return nil, false
			}

			return m.WarningAreaList, true
		},
	},
	{
		id: IDCancelAllWarningMessages, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PWSCancelRequest, raw []byte, enc per.Encoding) error {
			var v CancelAllWarningMessages

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.CancelAllWarningMessages = &v

			return nil
		},
		encode: func(m *PWSCancelRequest) (per.Marshaler, bool) {
			if m.CancelAllWarningMessages == nil {
				return nil, false
			}

			return m.CancelAllWarningMessages, true
		},
	},
}

func (m *PWSCancelRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPWSCancel, pWSCancelRequestIEs, m)
}

func (m *PWSCancelRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcPWSCancel,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePWSCancelRequest(value []byte) (*PWSCancelRequest, error) {
	return parseMessageBody[PWSCancelRequest](ProcPWSCancel, TriggeringInitiatingMessage, pWSCancelRequestIEs, value)
}

type PWSCancelResponse struct {
	MessageIdentifier          MessageIdentifier
	SerialNumber               SerialNumber
	BroadcastCancelledAreaList *BroadcastCancelledAreaList
	CriticalityDiagnostics     *CriticalityDiagnostics

	messageMeta
}

var pWSCancelResponseIEs = []ieSpec[PWSCancelResponse]{
	{
		id: IDMessageIdentifier, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PWSCancelResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MessageIdentifier)
		},
		encode: func(m *PWSCancelResponse) (per.Marshaler, bool) { return m.MessageIdentifier, true },
	},
	{
		id: IDSerialNumber, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PWSCancelResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SerialNumber)
		},
		encode: func(m *PWSCancelResponse) (per.Marshaler, bool) { return m.SerialNumber, true },
	},
	{
		id: IDBroadcastCancelledAreaList, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PWSCancelResponse, raw []byte, enc per.Encoding) error {
			var v BroadcastCancelledAreaList

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.BroadcastCancelledAreaList = &v

			return nil
		},
		encode: func(m *PWSCancelResponse) (per.Marshaler, bool) {
			if m.BroadcastCancelledAreaList == nil {
				return nil, false
			}

			return m.BroadcastCancelledAreaList, true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PWSCancelResponse, raw []byte, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		encode: func(m *PWSCancelResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *PWSCancelResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPWSCancel, pWSCancelResponseIEs, m)
}

func (m *PWSCancelResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcPWSCancel,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePWSCancelResponse(value []byte) (*PWSCancelResponse, error) {
	return parseMessageBody[PWSCancelResponse](ProcPWSCancel, TriggeringSuccessfulOutcome, pWSCancelResponseIEs, value)
}

type PWSRestartIndication struct {
	CellIDListForRestart          CellIDListForRestart
	GlobalRANNodeID               GlobalRANNodeID
	TAIListForRestart             TAIListForRestart
	EmergencyAreaIDListForRestart EmergencyAreaIDListForRestart

	messageMeta
}

var pWSRestartIndicationIEs = []ieSpec[PWSRestartIndication]{
	{
		id: IDCellIDListForRestart, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PWSRestartIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.CellIDListForRestart)
		},
		encode: func(m *PWSRestartIndication) (per.Marshaler, bool) { return m.CellIDListForRestart, true },
	},
	{
		id: IDGlobalRANNodeID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PWSRestartIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.GlobalRANNodeID)
		},
		encode: func(m *PWSRestartIndication) (per.Marshaler, bool) { return m.GlobalRANNodeID, true },
	},
	{
		id: IDTAIListForRestart, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PWSRestartIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.TAIListForRestart)
		},
		encode: func(m *PWSRestartIndication) (per.Marshaler, bool) { return m.TAIListForRestart, true },
	},
	{
		id: IDEmergencyAreaIDListForRestart, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PWSRestartIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.EmergencyAreaIDListForRestart)
		},
		encode: func(m *PWSRestartIndication) (per.Marshaler, bool) {
			if len(m.EmergencyAreaIDListForRestart) == 0 {
				return nil, false
			}

			return m.EmergencyAreaIDListForRestart, true
		},
	},
}

func (m *PWSRestartIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPWSRestartIndication, pWSRestartIndicationIEs, m)
}

func (m *PWSRestartIndication) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcPWSRestartIndication,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParsePWSRestartIndication(value []byte) (*PWSRestartIndication, error) {
	return parseMessageBody[PWSRestartIndication](ProcPWSRestartIndication, TriggeringInitiatingMessage, pWSRestartIndicationIEs, value)
}

type PWSFailureIndication struct {
	PWSFailedCellIDList PWSFailedCellIDList
	GlobalRANNodeID     GlobalRANNodeID

	messageMeta
}

var pWSFailureIndicationIEs = []ieSpec[PWSFailureIndication]{
	{
		id: IDPWSFailedCellIDList, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PWSFailureIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PWSFailedCellIDList)
		},
		encode: func(m *PWSFailureIndication) (per.Marshaler, bool) { return m.PWSFailedCellIDList, true },
	},
	{
		id: IDGlobalRANNodeID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PWSFailureIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.GlobalRANNodeID)
		},
		encode: func(m *PWSFailureIndication) (per.Marshaler, bool) { return m.GlobalRANNodeID, true },
	},
}

func (m *PWSFailureIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPWSFailureIndication, pWSFailureIndicationIEs, m)
}

func (m *PWSFailureIndication) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcPWSFailureIndication,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParsePWSFailureIndication(value []byte) (*PWSFailureIndication, error) {
	return parseMessageBody[PWSFailureIndication](ProcPWSFailureIndication, TriggeringInitiatingMessage, pWSFailureIndicationIEs, value)
}
