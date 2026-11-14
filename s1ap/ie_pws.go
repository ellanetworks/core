// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

const (
	maxnoofCellID                  = 65535
	maxnoofTAIforWarning           = 65535
	maxnoofEmergencyAreaID         = 65535
	maxnoofCellsineNB              = 256
	maxnoofCellsforRestart         = 256
	maxnoofRestartTAIs             = 2048
	maxnoofRestartEmergencyAreaIDs = 256
	maxWarningMessageContents      = 9600
	maxWarningAreaCoordinates      = 1024
	warningSecurityInfoOctets      = 50
)

var (
	repetitionPeriodBounds         = per.Bounds{LB: 0, UB: 4095, HasLB: true, HasUB: true}
	extendedRepetitionPeriodBounds = per.Bounds{LB: 4096, UB: 131071, HasLB: true, HasUB: true}
	numberofBroadcastRequestBounds = per.Bounds{LB: 0, UB: 65535, HasLB: true, HasUB: true}
)

type MessageIdentifier uint16

func (m MessageIdentifier) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(m), 16)
}

func (m *MessageIdentifier) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, 16)
	if err != nil {
		return err
	}

	*m = MessageIdentifier(v)

	return nil
}

type SerialNumber uint16

func (s SerialNumber) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(s), 16)
}

func (s *SerialNumber) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, 16)
	if err != nil {
		return err
	}

	*s = SerialNumber(v)

	return nil
}

type DataCodingScheme uint8

func (d DataCodingScheme) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(d), 8)
}

func (d *DataCodingScheme) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, 8)
	if err != nil {
		return err
	}

	*d = DataCodingScheme(v)

	return nil
}

type EmergencyAreaID [3]byte

func (e EmergencyAreaID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 3, 3, true, true, false, e[:])
}

func (e *EmergencyAreaID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 3, 3, true, true, false)
	if err != nil {
		return err
	}

	copy(e[:], b)

	return nil
}

type WarningType [2]byte

func (t WarningType) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 2, 2, true, true, false, t[:])
}

func (t *WarningType) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 2, 2, true, true, false)
	if err != nil {
		return err
	}

	copy(t[:], b)

	return nil
}

type WarningSecurityInfo [warningSecurityInfoOctets]byte

func (s WarningSecurityInfo) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, warningSecurityInfoOctets, warningSecurityInfoOctets, true, true, false, s[:])
}

func (s *WarningSecurityInfo) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, warningSecurityInfoOctets, warningSecurityInfoOctets, true, true, false)
	if err != nil {
		return err
	}

	copy(s[:], b)

	return nil
}

type WarningMessageContents []byte

func (c WarningMessageContents) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 1, maxWarningMessageContents, true, true, false, c)
}

func (c *WarningMessageContents) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 1, maxWarningMessageContents, true, true, false)
	if err != nil {
		return err
	}

	*c = b

	return nil
}

type WarningAreaCoordinates []byte

func (c WarningAreaCoordinates) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 1, maxWarningAreaCoordinates, true, true, false, c)
}

func (c *WarningAreaCoordinates) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 1, maxWarningAreaCoordinates, true, true, false)
	if err != nil {
		return err
	}

	*c = b

	return nil
}

type ConcurrentWarningMessageIndicator uint8

const ConcurrentWarningMessageIndicatorTrue ConcurrentWarningMessageIndicator = 0

func (c ConcurrentWarningMessageIndicator) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeEnumerated(w, enc, 1, false, int64(c))
}

func (c *ConcurrentWarningMessageIndicator) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeEnumerated(r, enc, 1, false)
	if err != nil {
		return err
	}

	*c = ConcurrentWarningMessageIndicator(v)

	return nil
}

type KillAllWarningMessages uint8

const KillAllWarningMessagesTrue KillAllWarningMessages = 0

func (k KillAllWarningMessages) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeEnumerated(w, enc, 1, false, int64(k))
}

func (k *KillAllWarningMessages) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeEnumerated(r, enc, 1, false)
	if err != nil {
		return err
	}

	*k = KillAllWarningMessages(v)

	return nil
}

type CellIDBroadcastItem struct {
	_    [0]struct{} `per:"extseq"`
	ECGI EUTRANCGI
	_    ieExtensions `per:",skip"`
}

type CompletedCellInTAIItem struct {
	_    [0]struct{} `per:"extseq"`
	ECGI EUTRANCGI
	_    ieExtensions `per:",skip"`
}

type TAIBroadcastItem struct {
	_                  [0]struct{} `per:"extseq"`
	TAI                TAI
	CompletedCellInTAI []CompletedCellInTAIItem `per:"SEQUENCE-OF,size:1..65535"`
	_                  ieExtensions             `per:",skip"`
}

type CompletedCellInEAIItem struct {
	_    [0]struct{} `per:"extseq"`
	ECGI EUTRANCGI
	_    ieExtensions `per:",skip"`
}

type EmergencyAreaIDBroadcastItem struct {
	_                  [0]struct{} `per:"extseq"`
	EmergencyAreaID    EmergencyAreaID
	CompletedCellInEAI []CompletedCellInEAIItem `per:"SEQUENCE-OF,size:1..65535"`
	_                  ieExtensions             `per:",skip"`
}

type CellIDCancelledItem struct {
	_                  [0]struct{} `per:"extseq"`
	ECGI               EUTRANCGI
	NumberOfBroadcasts uint16       `per:",range:0..65535"`
	_                  ieExtensions `per:",skip"`
}

type CancelledCellInTAIItem struct {
	_                  [0]struct{} `per:"extseq"`
	ECGI               EUTRANCGI
	NumberOfBroadcasts uint16       `per:",range:0..65535"`
	_                  ieExtensions `per:",skip"`
}

type TAICancelledItem struct {
	_                  [0]struct{} `per:"extseq"`
	TAI                TAI
	CancelledCellInTAI []CancelledCellInTAIItem `per:"SEQUENCE-OF,size:1..65535"`
	_                  ieExtensions             `per:",skip"`
}

type CancelledCellInEAIItem struct {
	_                  [0]struct{} `per:"extseq"`
	ECGI               EUTRANCGI
	NumberOfBroadcasts uint16       `per:",range:0..65535"`
	_                  ieExtensions `per:",skip"`
}

type EmergencyAreaIDCancelledItem struct {
	_                  [0]struct{} `per:"extseq"`
	EmergencyAreaID    EmergencyAreaID
	CancelledCellInEAI []CancelledCellInEAIItem `per:"SEQUENCE-OF,size:1..65535"`
	_                  ieExtensions             `per:",skip"`
}

func encodeExtensibleChoice(w *per.Writer, enc per.Encoding, nRoot, idx int64) error {
	w.WriteBit(false)

	return per.EncodeConstrainedWholeNumber(w, enc, 0, nRoot-1, idx)
}

func decodeExtensibleChoice(r *per.Reader, enc per.Encoding, nRoot int64, name string) (int64, error) {
	isExt, err := r.ReadBit()
	if err != nil {
		return 0, fmt.Errorf("s1ap: %s choice: %w", name, err)
	}

	if isExt {
		return 0, fmt.Errorf("%w: %s extension alternative", errNotComprehended, name)
	}

	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, nRoot-1)
	if err != nil {
		return 0, fmt.Errorf("s1ap: %s choice: %w", name, err)
	}

	return idx, nil
}

func choiceAlternative(name string, set ...bool) (int64, error) {
	idx := int64(-1)

	for i, s := range set {
		if !s {
			continue
		}

		if idx >= 0 {
			return 0, fmt.Errorf("s1ap: %s has more than one alternative set", name)
		}

		idx = int64(i)
	}

	if idx < 0 {
		return 0, fmt.Errorf("s1ap: %s has no alternative set", name)
	}

	return idx, nil
}

type WarningAreaList struct {
	CellIDList          []EUTRANCGI
	TAIList             []TAI
	EmergencyAreaIDList []EmergencyAreaID
}

func (l WarningAreaList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	idx, err := choiceAlternative("WarningAreaList", len(l.CellIDList) > 0, len(l.TAIList) > 0, len(l.EmergencyAreaIDList) > 0)
	if err != nil {
		return err
	}

	if err := encodeExtensibleChoice(w, enc, 3, idx); err != nil {
		return err
	}

	switch idx {
	case 0:
		return marshalSeqOf(w, enc, 1, maxnoofCellID, l.CellIDList)
	case 1:
		return marshalSeqOf(w, enc, 1, maxnoofTAIforWarning, l.TAIList)
	default:
		return marshalSeqOf(w, enc, 1, maxnoofEmergencyAreaID, l.EmergencyAreaIDList)
	}
}

func (l *WarningAreaList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeExtensibleChoice(r, enc, 3, "WarningAreaList")
	if err != nil {
		return err
	}

	*l = WarningAreaList{}

	switch idx {
	case 0:
		l.CellIDList, err = unmarshalSeqOf[EUTRANCGI](r, enc, 1, maxnoofCellID)
	case 1:
		l.TAIList, err = unmarshalSeqOf[TAI](r, enc, 1, maxnoofTAIforWarning)
	default:
		l.EmergencyAreaIDList, err = unmarshalSeqOf[EmergencyAreaID](r, enc, 1, maxnoofEmergencyAreaID)
	}

	return err
}

type BroadcastCompletedAreaList struct {
	CellIDBroadcast          []CellIDBroadcastItem
	TAIBroadcast             []TAIBroadcastItem
	EmergencyAreaIDBroadcast []EmergencyAreaIDBroadcastItem
}

func (l BroadcastCompletedAreaList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	idx, err := choiceAlternative("BroadcastCompletedAreaList", len(l.CellIDBroadcast) > 0, len(l.TAIBroadcast) > 0, len(l.EmergencyAreaIDBroadcast) > 0)
	if err != nil {
		return err
	}

	if err := encodeExtensibleChoice(w, enc, 3, idx); err != nil {
		return err
	}

	switch idx {
	case 0:
		return marshalSeqOf(w, enc, 1, maxnoofCellID, l.CellIDBroadcast)
	case 1:
		return marshalSeqOf(w, enc, 1, maxnoofTAIforWarning, l.TAIBroadcast)
	default:
		return marshalSeqOf(w, enc, 1, maxnoofEmergencyAreaID, l.EmergencyAreaIDBroadcast)
	}
}

func (l *BroadcastCompletedAreaList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeExtensibleChoice(r, enc, 3, "BroadcastCompletedAreaList")
	if err != nil {
		return err
	}

	*l = BroadcastCompletedAreaList{}

	switch idx {
	case 0:
		l.CellIDBroadcast, err = unmarshalSeqOf[CellIDBroadcastItem](r, enc, 1, maxnoofCellID)
	case 1:
		l.TAIBroadcast, err = unmarshalSeqOf[TAIBroadcastItem](r, enc, 1, maxnoofTAIforWarning)
	default:
		l.EmergencyAreaIDBroadcast, err = unmarshalSeqOf[EmergencyAreaIDBroadcastItem](r, enc, 1, maxnoofEmergencyAreaID)
	}

	return err
}

type BroadcastCancelledAreaList struct {
	CellIDCancelled          []CellIDCancelledItem
	TAICancelled             []TAICancelledItem
	EmergencyAreaIDCancelled []EmergencyAreaIDCancelledItem
}

func (l BroadcastCancelledAreaList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	idx, err := choiceAlternative("BroadcastCancelledAreaList", len(l.CellIDCancelled) > 0, len(l.TAICancelled) > 0, len(l.EmergencyAreaIDCancelled) > 0)
	if err != nil {
		return err
	}

	if err := encodeExtensibleChoice(w, enc, 3, idx); err != nil {
		return err
	}

	switch idx {
	case 0:
		return marshalSeqOf(w, enc, 1, maxnoofCellID, l.CellIDCancelled)
	case 1:
		return marshalSeqOf(w, enc, 1, maxnoofTAIforWarning, l.TAICancelled)
	default:
		return marshalSeqOf(w, enc, 1, maxnoofEmergencyAreaID, l.EmergencyAreaIDCancelled)
	}
}

func (l *BroadcastCancelledAreaList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeExtensibleChoice(r, enc, 3, "BroadcastCancelledAreaList")
	if err != nil {
		return err
	}

	*l = BroadcastCancelledAreaList{}

	switch idx {
	case 0:
		l.CellIDCancelled, err = unmarshalSeqOf[CellIDCancelledItem](r, enc, 1, maxnoofCellID)
	case 1:
		l.TAICancelled, err = unmarshalSeqOf[TAICancelledItem](r, enc, 1, maxnoofTAIforWarning)
	default:
		l.EmergencyAreaIDCancelled, err = unmarshalSeqOf[EmergencyAreaIDCancelledItem](r, enc, 1, maxnoofEmergencyAreaID)
	}

	return err
}
