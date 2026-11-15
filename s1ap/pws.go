// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

func decodeOptionalIE[T any, PT interface {
	*T
	per.Unmarshaler
}](r *per.Reader, dst **T) error {
	v := PT(new(T))
	if err := decodeIE(r, v); err != nil {
		return err
	}

	*dst = (*T)(v)

	return nil
}

func optionalIE[T any, PT interface {
	*T
	per.Marshaler
}](v *T) (per.Marshaler, bool) {
	if v == nil {
		return nil, false
	}

	return PT(v), true
}

func decodeOptionalInteger[T ~uint8 | ~uint16 | ~uint32](r *per.Reader, b per.Bounds, dst **T) error {
	n, err := per.DecodeInteger(r, per.Aligned, b)
	if err != nil {
		return err
	}

	v := T(n)
	*dst = &v

	return nil
}

func integerIE(b per.Bounds, v int64) per.Marshaler {
	return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
		return per.EncodeInteger(w, enc, b, v)
	})
}

type WriteReplaceWarningRequest struct {
	MessageIdentifier                 MessageIdentifier
	SerialNumber                      SerialNumber
	WarningAreaList                   *WarningAreaList
	RepetitionPeriod                  uint16
	ExtendedRepetitionPeriod          *uint32
	NumberofBroadcastRequest          uint16
	WarningType                       *WarningType
	WarningSecurityInfo               *WarningSecurityInfo
	DataCodingScheme                  *DataCodingScheme
	WarningMessageContents            *WarningMessageContents
	ConcurrentWarningMessageIndicator *ConcurrentWarningMessageIndicator
	WarningAreaCoordinates            *WarningAreaCoordinates

	messageMeta
}

var writeReplaceWarningRequestIEs = []ieSpec[WriteReplaceWarningRequest]{
	{
		ID: IDMessageIdentifier, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MessageIdentifier)
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return &m.MessageIdentifier, true },
	},
	{
		ID: IDSerialNumber, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SerialNumber)
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return &m.SerialNumber, true },
	},
	{
		ID: IDWarningAreaList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.WarningAreaList)
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return optionalIE(m.WarningAreaList) },
	},
	{
		ID: IDRepetitionPeriod, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			n, err := per.DecodeInteger(r, enc, repetitionPeriodBounds)
			m.RepetitionPeriod = uint16(n)

			return err
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			return integerIE(repetitionPeriodBounds, int64(m.RepetitionPeriod)), true
		},
	},
	{
		ID: IDExtendedRepetitionPeriod, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalInteger(r, extendedRepetitionPeriodBounds, &m.ExtendedRepetitionPeriod)
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			if m.ExtendedRepetitionPeriod == nil {
				return nil, false
			}

			return integerIE(extendedRepetitionPeriodBounds, int64(*m.ExtendedRepetitionPeriod)), true
		},
	},
	{
		ID: IDNumberofBroadcastRequest, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			n, err := per.DecodeInteger(r, enc, numberofBroadcastRequestBounds)
			m.NumberofBroadcastRequest = uint16(n)

			return err
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			return integerIE(numberofBroadcastRequestBounds, int64(m.NumberofBroadcastRequest)), true
		},
	},
	{
		ID: IDWarningType, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.WarningType)
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return optionalIE(m.WarningType) },
	},
	{
		ID: IDWarningSecurityInfo, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.WarningSecurityInfo)
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return optionalIE(m.WarningSecurityInfo) },
	},
	{
		ID: IDDataCodingScheme, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.DataCodingScheme)
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return optionalIE(m.DataCodingScheme) },
	},
	{
		ID: IDWarningMessageContents, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.WarningMessageContents)
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			return optionalIE(m.WarningMessageContents)
		},
	},
	{
		ID: IDConcurrentWarningMessageIndicator, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.ConcurrentWarningMessageIndicator)
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			return optionalIE(m.ConcurrentWarningMessageIndicator)
		},
	},
	{
		ID: IDWarningAreaCoordinates, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *WriteReplaceWarningRequest, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.WarningAreaCoordinates)
		},
		Encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			return optionalIE(m.WarningAreaCoordinates)
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
		Encode: func(m *WriteReplaceWarningResponse) (per.Marshaler, bool) { return &m.MessageIdentifier, true },
	},
	{
		ID: IDSerialNumber, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *WriteReplaceWarningResponse, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SerialNumber)
		},
		Encode: func(m *WriteReplaceWarningResponse) (per.Marshaler, bool) { return &m.SerialNumber, true },
	},
	{
		ID: IDBroadcastCompletedAreaList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *WriteReplaceWarningResponse, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.BroadcastCompletedAreaList)
		},
		Encode: func(m *WriteReplaceWarningResponse) (per.Marshaler, bool) {
			return optionalIE(m.BroadcastCompletedAreaList)
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *WriteReplaceWarningResponse, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.CriticalityDiagnostics)
		},
		Encode: func(m *WriteReplaceWarningResponse) (per.Marshaler, bool) {
			return optionalIE(m.CriticalityDiagnostics)
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

type KillRequest struct {
	MessageIdentifier      MessageIdentifier
	SerialNumber           SerialNumber
	WarningAreaList        *WarningAreaList
	KillAllWarningMessages *KillAllWarningMessages

	messageMeta
}

var killRequestIEs = []ieSpec[KillRequest]{
	{
		ID: IDMessageIdentifier, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *KillRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MessageIdentifier)
		},
		Encode: func(m *KillRequest) (per.Marshaler, bool) { return &m.MessageIdentifier, true },
	},
	{
		ID: IDSerialNumber, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *KillRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SerialNumber)
		},
		Encode: func(m *KillRequest) (per.Marshaler, bool) { return &m.SerialNumber, true },
	},
	{
		ID: IDWarningAreaList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *KillRequest, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.WarningAreaList)
		},
		Encode: func(m *KillRequest) (per.Marshaler, bool) { return optionalIE(m.WarningAreaList) },
	},
	{
		ID: IDKillAllWarningMessages, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *KillRequest, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.KillAllWarningMessages)
		},
		Encode: func(m *KillRequest) (per.Marshaler, bool) { return optionalIE(m.KillAllWarningMessages) },
	},
}

func (m *KillRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcKill, killRequestIEs, m)
}

func (m *KillRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcKill,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseKillRequest(value []byte) (*KillRequest, error) {
	return parseMessageBody[KillRequest](ProcKill, TriggeringInitiatingMessage, killRequestIEs, value)
}

func DecodeKillRequest(value []byte) (*KillRequest, error) {
	return decodeMessageBody[KillRequest](ProcKill, killRequestIEs, value)
}

type KillResponse struct {
	MessageIdentifier          MessageIdentifier
	SerialNumber               SerialNumber
	BroadcastCancelledAreaList *BroadcastCancelledAreaList
	CriticalityDiagnostics     *CriticalityDiagnostics

	messageMeta
}

var killResponseIEs = []ieSpec[KillResponse]{
	{
		ID: IDMessageIdentifier, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *KillResponse, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MessageIdentifier)
		},
		Encode: func(m *KillResponse) (per.Marshaler, bool) { return &m.MessageIdentifier, true },
	},
	{
		ID: IDSerialNumber, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *KillResponse, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SerialNumber)
		},
		Encode: func(m *KillResponse) (per.Marshaler, bool) { return &m.SerialNumber, true },
	},
	{
		ID: IDBroadcastCancelledAreaList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *KillResponse, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.BroadcastCancelledAreaList)
		},
		Encode: func(m *KillResponse) (per.Marshaler, bool) { return optionalIE(m.BroadcastCancelledAreaList) },
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *KillResponse, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.CriticalityDiagnostics)
		},
		Encode: func(m *KillResponse) (per.Marshaler, bool) { return optionalIE(m.CriticalityDiagnostics) },
	},
}

func (m *KillResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcKill, killResponseIEs, m)
}

func (m *KillResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcKill,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseKillResponse(value []byte) (*KillResponse, error) {
	return parseMessageBody[KillResponse](ProcKill, TriggeringSuccessfulOutcome, killResponseIEs, value)
}

func DecodeKillResponse(value []byte) (*KillResponse, error) {
	return decodeMessageBody[KillResponse](ProcKill, killResponseIEs, value)
}

type PWSRestartIndication struct {
	ECGIListForRestart            []EUTRANCGI
	GlobalENBID                   GlobalENBID
	TAIListForRestart             []TAI
	EmergencyAreaIDListForRestart []EmergencyAreaID

	messageMeta
}

var pWSRestartIndicationIEs = []ieSpec[PWSRestartIndication]{
	{
		ID: IDECGIListForRestart, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PWSRestartIndication, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ECGIListForRestart, err = unmarshalSeqOf[EUTRANCGI](r, enc, 1, maxnoofCellsforRestart)

			return err
		},
		Encode: func(m *PWSRestartIndication) (per.Marshaler, bool) {
			if len(m.ECGIListForRestart) == 0 {
				return nil, false
			}

			return seqOf(1, maxnoofCellsforRestart, &m.ECGIListForRestart), true
		},
	},
	{
		ID: IDGlobalENBID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PWSRestartIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.GlobalENBID)
		},
		Encode: func(m *PWSRestartIndication) (per.Marshaler, bool) { return &m.GlobalENBID, true },
	},
	{
		ID: IDTAIListForRestart, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PWSRestartIndication, r *per.Reader, enc per.Encoding) error {
			var err error

			m.TAIListForRestart, err = unmarshalSeqOf[TAI](r, enc, 1, maxnoofRestartTAIs)

			return err
		},
		Encode: func(m *PWSRestartIndication) (per.Marshaler, bool) {
			if len(m.TAIListForRestart) == 0 {
				return nil, false
			}

			return seqOf(1, maxnoofRestartTAIs, &m.TAIListForRestart), true
		},
	},
	{
		ID: IDEmergencyAreaIDListForRestart, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *PWSRestartIndication, r *per.Reader, enc per.Encoding) error {
			var err error

			m.EmergencyAreaIDListForRestart, err = unmarshalSeqOf[EmergencyAreaID](r, enc, 1, maxnoofRestartEmergencyAreaIDs)

			return err
		},
		Encode: func(m *PWSRestartIndication) (per.Marshaler, bool) {
			if len(m.EmergencyAreaIDListForRestart) == 0 {
				return nil, false
			}

			return seqOf(1, maxnoofRestartEmergencyAreaIDs, &m.EmergencyAreaIDListForRestart), true
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
	PWSFailedECGIList []EUTRANCGI
	GlobalENBID       GlobalENBID

	messageMeta
}

var pWSFailureIndicationIEs = []ieSpec[PWSFailureIndication]{
	{
		ID: IDPWSfailedECGIList, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PWSFailureIndication, r *per.Reader, enc per.Encoding) error {
			var err error

			m.PWSFailedECGIList, err = unmarshalSeqOf[EUTRANCGI](r, enc, 1, maxnoofCellsineNB)

			return err
		},
		Encode: func(m *PWSFailureIndication) (per.Marshaler, bool) {
			if len(m.PWSFailedECGIList) == 0 {
				return nil, false
			}

			return seqOf(1, maxnoofCellsineNB, &m.PWSFailedECGIList), true
		},
	},
	{
		ID: IDGlobalENBID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *PWSFailureIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.GlobalENBID)
		},
		Encode: func(m *PWSFailureIndication) (per.Marshaler, bool) { return &m.GlobalENBID, true },
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
