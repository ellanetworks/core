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
		ID: IDMessageIdentifier, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MessageIdentifier)
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return m.MessageIdentifier, true },
	},
	{
		ID: IDSerialNumber, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SerialNumber)
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return m.SerialNumber, true },
	},
	{
		ID: IDWarningAreaList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			var v WarningAreaList

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.WarningAreaList = &v

			return nil
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			if m.WarningAreaList == nil {
				return nil, false
			}

			return m.WarningAreaList, true
		},
	},
	{
		ID: IDRepetitionPeriod, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RepetitionPeriod)
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return m.RepetitionPeriod, true },
	},
	{
		ID: IDNumberOfBroadcastsRequested, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.NumberOfBroadcastsRequested)
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return m.NumberOfBroadcastsRequested, true },
	},
	{
		ID: IDWarningType, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			var v WarningType

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.WarningType = &v

			return nil
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			if m.WarningType == nil {
				return nil, false
			}

			return m.WarningType, true
		},
	},
	{
		ID: IDWarningSecurityInfo, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			var v WarningSecurityInfo

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.WarningSecurityInfo = &v

			return nil
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			if m.WarningSecurityInfo == nil {
				return nil, false
			}

			return m.WarningSecurityInfo, true
		},
	},
	{
		ID: IDDataCodingScheme, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			var v DataCodingScheme

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.DataCodingScheme = &v

			return nil
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			if m.DataCodingScheme == nil {
				return nil, false
			}

			return m.DataCodingScheme, true
		},
	},
	{
		ID: IDWarningMessageContents, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.WarningMessageContents)
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			if len(m.WarningMessageContents) == 0 {
				return nil, false
			}

			return m.WarningMessageContents, true
		},
	},
	{
		ID: IDConcurrentWarningMessageInd, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			var v ConcurrentWarningMessageInd

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.ConcurrentWarningMessageInd = &v

			return nil
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			if m.ConcurrentWarningMessageInd == nil {
				return nil, false
			}

			return m.ConcurrentWarningMessageInd, true
		},
	},
	{
		ID: IDWarningAreaCoordinates, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.WarningAreaCoordinates)
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
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

func DecodeWriteReplaceWarningRequest(value []byte) (*WriteReplaceWarningRequest, error) {
	return decodeMessageBody[WriteReplaceWarningRequest](ProcWriteReplaceWarning, writeReplaceWarningRequestIEs, value)
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
		ID: IDMessageIdentifier, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *WriteReplaceWarningResponse, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MessageIdentifier)
		},
		Encode: func(m *WriteReplaceWarningResponse) (per.Marshaler, bool) { return m.MessageIdentifier, true },
	},
	{
		ID: IDSerialNumber, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *WriteReplaceWarningResponse, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SerialNumber)
		},
		Encode: func(m *WriteReplaceWarningResponse) (per.Marshaler, bool) { return m.SerialNumber, true },
	},
	{
		ID: IDBroadcastCompletedAreaList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *WriteReplaceWarningResponse, r *per.Reader, enc per.Encoding) error {
			var v BroadcastCompletedAreaList

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.BroadcastCompletedAreaList = &v

			return nil
		},
		Encode: func(m *WriteReplaceWarningResponse) (per.Marshaler, bool) {
			if m.BroadcastCompletedAreaList == nil {
				return nil, false
			}

			return m.BroadcastCompletedAreaList, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *WriteReplaceWarningResponse, r *per.Reader, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		Encode: func(m *WriteReplaceWarningResponse) (per.Marshaler, bool) {
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

func DecodeWriteReplaceWarningResponse(value []byte) (*WriteReplaceWarningResponse, error) {
	return decodeMessageBody[WriteReplaceWarningResponse](ProcWriteReplaceWarning, writeReplaceWarningResponseIEs, value)
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
		ID: IDMessageIdentifier, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PWSCancelRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MessageIdentifier)
		},
		Encode: func(m *PWSCancelRequest) (per.Marshaler, bool) { return m.MessageIdentifier, true },
	},
	{
		ID: IDSerialNumber, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PWSCancelRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SerialNumber)
		},
		Encode: func(m *PWSCancelRequest) (per.Marshaler, bool) { return m.SerialNumber, true },
	},
	{
		ID: IDWarningAreaList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PWSCancelRequest, r *per.Reader, enc per.Encoding) error {
			var v WarningAreaList

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.WarningAreaList = &v

			return nil
		},
		Encode: func(m *PWSCancelRequest) (per.Marshaler, bool) {
			if m.WarningAreaList == nil {
				return nil, false
			}

			return m.WarningAreaList, true
		},
	},
	{
		ID: IDCancelAllWarningMessages, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *PWSCancelRequest, r *per.Reader, enc per.Encoding) error {
			var v CancelAllWarningMessages

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.CancelAllWarningMessages = &v

			return nil
		},
		Encode: func(m *PWSCancelRequest) (per.Marshaler, bool) {
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

func DecodePWSCancelRequest(value []byte) (*PWSCancelRequest, error) {
	return decodeMessageBody[PWSCancelRequest](ProcPWSCancel, pWSCancelRequestIEs, value)
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
		ID: IDMessageIdentifier, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PWSCancelResponse, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MessageIdentifier)
		},
		Encode: func(m *PWSCancelResponse) (per.Marshaler, bool) { return m.MessageIdentifier, true },
	},
	{
		ID: IDSerialNumber, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PWSCancelResponse, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SerialNumber)
		},
		Encode: func(m *PWSCancelResponse) (per.Marshaler, bool) { return m.SerialNumber, true },
	},
	{
		ID: IDBroadcastCancelledAreaList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PWSCancelResponse, r *per.Reader, enc per.Encoding) error {
			var v BroadcastCancelledAreaList

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.BroadcastCancelledAreaList = &v

			return nil
		},
		Encode: func(m *PWSCancelResponse) (per.Marshaler, bool) {
			if m.BroadcastCancelledAreaList == nil {
				return nil, false
			}

			return m.BroadcastCancelledAreaList, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *PWSCancelResponse, r *per.Reader, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		Encode: func(m *PWSCancelResponse) (per.Marshaler, bool) {
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

func DecodePWSCancelResponse(value []byte) (*PWSCancelResponse, error) {
	return decodeMessageBody[PWSCancelResponse](ProcPWSCancel, pWSCancelResponseIEs, value)
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
		ID: IDCellIDListForRestart, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PWSRestartIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.CellIDListForRestart)
		},
		Encode: func(m *PWSRestartIndication) (per.Marshaler, bool) { return &m.CellIDListForRestart, true },
	},
	{
		ID: IDGlobalRANNodeID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PWSRestartIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.GlobalRANNodeID)
		},
		Encode: func(m *PWSRestartIndication) (per.Marshaler, bool) { return &m.GlobalRANNodeID, true },
	},
	{
		ID: IDTAIListForRestart, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PWSRestartIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.TAIListForRestart)
		},
		Encode: func(m *PWSRestartIndication) (per.Marshaler, bool) { return m.TAIListForRestart, true },
	},
	{
		ID: IDEmergencyAreaIDListForRestart, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *PWSRestartIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.EmergencyAreaIDListForRestart)
		},
		Encode: func(m *PWSRestartIndication) (per.Marshaler, bool) {
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

func DecodePWSRestartIndication(value []byte) (*PWSRestartIndication, error) {
	return decodeMessageBody[PWSRestartIndication](ProcPWSRestartIndication, pWSRestartIndicationIEs, value)
}

type PWSFailureIndication struct {
	PWSFailedCellIDList PWSFailedCellIDList
	GlobalRANNodeID     GlobalRANNodeID

	messageMeta
}

var pWSFailureIndicationIEs = []ieSpec[PWSFailureIndication]{
	{
		ID: IDPWSFailedCellIDList, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PWSFailureIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PWSFailedCellIDList)
		},
		Encode: func(m *PWSFailureIndication) (per.Marshaler, bool) { return &m.PWSFailedCellIDList, true },
	},
	{
		ID: IDGlobalRANNodeID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PWSFailureIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.GlobalRANNodeID)
		},
		Encode: func(m *PWSFailureIndication) (per.Marshaler, bool) { return &m.GlobalRANNodeID, true },
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

func DecodePWSFailureIndication(value []byte) (*PWSFailureIndication, error) {
	return decodeMessageBody[PWSFailureIndication](ProcPWSFailureIndication, pWSFailureIndicationIEs, value)
}
