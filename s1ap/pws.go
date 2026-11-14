// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

func decodeOptionalIE[T any, PT interface {
	*T
	per.Unmarshaler
}](raw []byte, dst **T) error {
	v := PT(new(T))
	if err := perIEDecode(raw, v); err != nil {
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

func decodeOptionalInteger[T ~uint8 | ~uint16 | ~uint32](raw []byte, b per.Bounds, dst **T) error {
	n, err := per.DecodeInteger(per.NewReader(raw), per.Aligned, b)
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
		id: IDMessageIdentifier, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MessageIdentifier)
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return &m.MessageIdentifier, true },
	},
	{
		id: IDSerialNumber, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SerialNumber)
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return &m.SerialNumber, true },
	},
	{
		id: IDWarningAreaList, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			return decodeOptionalIE(raw, &m.WarningAreaList)
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return optionalIE(m.WarningAreaList) },
	},
	{
		id: IDRepetitionPeriod, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			n, err := per.DecodeInteger(per.NewReader(raw), enc, repetitionPeriodBounds)
			m.RepetitionPeriod = uint16(n)

			return err
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			return integerIE(repetitionPeriodBounds, int64(m.RepetitionPeriod)), true
		},
	},
	{
		id: IDExtendedRepetitionPeriod, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			return decodeOptionalInteger(raw, extendedRepetitionPeriodBounds, &m.ExtendedRepetitionPeriod)
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			if m.ExtendedRepetitionPeriod == nil {
				return nil, false
			}

			return integerIE(extendedRepetitionPeriodBounds, int64(*m.ExtendedRepetitionPeriod)), true
		},
	},
	{
		id: IDNumberofBroadcastRequest, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			n, err := per.DecodeInteger(per.NewReader(raw), enc, numberofBroadcastRequestBounds)
			m.NumberofBroadcastRequest = uint16(n)

			return err
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			return integerIE(numberofBroadcastRequestBounds, int64(m.NumberofBroadcastRequest)), true
		},
	},
	{
		id: IDWarningType, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			return decodeOptionalIE(raw, &m.WarningType)
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return optionalIE(m.WarningType) },
	},
	{
		id: IDWarningSecurityInfo, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			return decodeOptionalIE(raw, &m.WarningSecurityInfo)
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return optionalIE(m.WarningSecurityInfo) },
	},
	{
		id: IDDataCodingScheme, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			return decodeOptionalIE(raw, &m.DataCodingScheme)
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) { return optionalIE(m.DataCodingScheme) },
	},
	{
		id: IDWarningMessageContents, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			return decodeOptionalIE(raw, &m.WarningMessageContents)
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			return optionalIE(m.WarningMessageContents)
		},
	},
	{
		id: IDConcurrentWarningMessageIndicator, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			return decodeOptionalIE(raw, &m.ConcurrentWarningMessageIndicator)
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
			return optionalIE(m.ConcurrentWarningMessageIndicator)
		},
	},
	{
		id: IDWarningAreaCoordinates, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *WriteReplaceWarningRequest, raw []byte, enc per.Encoding) error {
			return decodeOptionalIE(raw, &m.WarningAreaCoordinates)
		},
		encode: func(m *WriteReplaceWarningRequest) (per.Marshaler, bool) {
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
		encode: func(m *WriteReplaceWarningResponse) (per.Marshaler, bool) { return &m.MessageIdentifier, true },
	},
	{
		id: IDSerialNumber, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *WriteReplaceWarningResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SerialNumber)
		},
		encode: func(m *WriteReplaceWarningResponse) (per.Marshaler, bool) { return &m.SerialNumber, true },
	},
	{
		id: IDBroadcastCompletedAreaList, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *WriteReplaceWarningResponse, raw []byte, enc per.Encoding) error {
			return decodeOptionalIE(raw, &m.BroadcastCompletedAreaList)
		},
		encode: func(m *WriteReplaceWarningResponse) (per.Marshaler, bool) {
			return optionalIE(m.BroadcastCompletedAreaList)
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *WriteReplaceWarningResponse, raw []byte, enc per.Encoding) error {
			return decodeOptionalIE(raw, &m.CriticalityDiagnostics)
		},
		encode: func(m *WriteReplaceWarningResponse) (per.Marshaler, bool) {
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

type KillRequest struct {
	MessageIdentifier      MessageIdentifier
	SerialNumber           SerialNumber
	WarningAreaList        *WarningAreaList
	KillAllWarningMessages *KillAllWarningMessages

	messageMeta
}

var killRequestIEs = []ieSpec[KillRequest]{
	{
		id: IDMessageIdentifier, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *KillRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MessageIdentifier)
		},
		encode: func(m *KillRequest) (per.Marshaler, bool) { return &m.MessageIdentifier, true },
	},
	{
		id: IDSerialNumber, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *KillRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SerialNumber)
		},
		encode: func(m *KillRequest) (per.Marshaler, bool) { return &m.SerialNumber, true },
	},
	{
		id: IDWarningAreaList, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *KillRequest, raw []byte, enc per.Encoding) error {
			return decodeOptionalIE(raw, &m.WarningAreaList)
		},
		encode: func(m *KillRequest) (per.Marshaler, bool) { return optionalIE(m.WarningAreaList) },
	},
	{
		id: IDKillAllWarningMessages, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *KillRequest, raw []byte, enc per.Encoding) error {
			return decodeOptionalIE(raw, &m.KillAllWarningMessages)
		},
		encode: func(m *KillRequest) (per.Marshaler, bool) { return optionalIE(m.KillAllWarningMessages) },
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

type KillResponse struct {
	MessageIdentifier          MessageIdentifier
	SerialNumber               SerialNumber
	BroadcastCancelledAreaList *BroadcastCancelledAreaList
	CriticalityDiagnostics     *CriticalityDiagnostics

	messageMeta
}

var killResponseIEs = []ieSpec[KillResponse]{
	{
		id: IDMessageIdentifier, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *KillResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MessageIdentifier)
		},
		encode: func(m *KillResponse) (per.Marshaler, bool) { return &m.MessageIdentifier, true },
	},
	{
		id: IDSerialNumber, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *KillResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SerialNumber)
		},
		encode: func(m *KillResponse) (per.Marshaler, bool) { return &m.SerialNumber, true },
	},
	{
		id: IDBroadcastCancelledAreaList, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *KillResponse, raw []byte, enc per.Encoding) error {
			return decodeOptionalIE(raw, &m.BroadcastCancelledAreaList)
		},
		encode: func(m *KillResponse) (per.Marshaler, bool) { return optionalIE(m.BroadcastCancelledAreaList) },
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *KillResponse, raw []byte, enc per.Encoding) error {
			return decodeOptionalIE(raw, &m.CriticalityDiagnostics)
		},
		encode: func(m *KillResponse) (per.Marshaler, bool) { return optionalIE(m.CriticalityDiagnostics) },
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

type PWSRestartIndication struct {
	ECGIListForRestart            []EUTRANCGI
	GlobalENBID                   GlobalENBID
	TAIListForRestart             []TAI
	EmergencyAreaIDListForRestart []EmergencyAreaID

	messageMeta
}

var pWSRestartIndicationIEs = []ieSpec[PWSRestartIndication]{
	{
		id: IDECGIListForRestart, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PWSRestartIndication, raw []byte, enc per.Encoding) error {
			var err error

			m.ECGIListForRestart, err = unmarshalSeqOf[EUTRANCGI](per.NewReader(raw), enc, 1, maxnoofCellsforRestart)

			return err
		},
		encode: func(m *PWSRestartIndication) (per.Marshaler, bool) {
			if len(m.ECGIListForRestart) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return marshalSeqOf(w, enc, 1, maxnoofCellsforRestart, m.ECGIListForRestart)
			}), true
		},
	},
	{
		id: IDGlobalENBID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PWSRestartIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.GlobalENBID)
		},
		encode: func(m *PWSRestartIndication) (per.Marshaler, bool) { return &m.GlobalENBID, true },
	},
	{
		id: IDTAIListForRestart, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PWSRestartIndication, raw []byte, enc per.Encoding) error {
			var err error

			m.TAIListForRestart, err = unmarshalSeqOf[TAI](per.NewReader(raw), enc, 1, maxnoofRestartTAIs)

			return err
		},
		encode: func(m *PWSRestartIndication) (per.Marshaler, bool) {
			if len(m.TAIListForRestart) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return marshalSeqOf(w, enc, 1, maxnoofRestartTAIs, m.TAIListForRestart)
			}), true
		},
	},
	{
		id: IDEmergencyAreaIDListForRestart, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PWSRestartIndication, raw []byte, enc per.Encoding) error {
			var err error

			m.EmergencyAreaIDListForRestart, err = unmarshalSeqOf[EmergencyAreaID](per.NewReader(raw), enc, 1, maxnoofRestartEmergencyAreaIDs)

			return err
		},
		encode: func(m *PWSRestartIndication) (per.Marshaler, bool) {
			if len(m.EmergencyAreaIDListForRestart) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return marshalSeqOf(w, enc, 1, maxnoofRestartEmergencyAreaIDs, m.EmergencyAreaIDListForRestart)
			}), true
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
	PWSFailedECGIList []EUTRANCGI
	GlobalENBID       GlobalENBID

	messageMeta
}

var pWSFailureIndicationIEs = []ieSpec[PWSFailureIndication]{
	{
		id: IDPWSfailedECGIList, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PWSFailureIndication, raw []byte, enc per.Encoding) error {
			var err error

			m.PWSFailedECGIList, err = unmarshalSeqOf[EUTRANCGI](per.NewReader(raw), enc, 1, maxnoofCellsineNB)

			return err
		},
		encode: func(m *PWSFailureIndication) (per.Marshaler, bool) {
			if len(m.PWSFailedECGIList) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return marshalSeqOf(w, enc, 1, maxnoofCellsineNB, m.PWSFailedECGIList)
			}), true
		},
	},
	{
		id: IDGlobalENBID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PWSFailureIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.GlobalENBID)
		},
		encode: func(m *PWSFailureIndication) (per.Marshaler, bool) { return &m.GlobalENBID, true },
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
