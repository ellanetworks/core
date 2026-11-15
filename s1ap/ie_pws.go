// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
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

type KillAllWarningMessages uint8

const KillAllWarningMessagesTrue KillAllWarningMessages = 0

type CellIDBroadcastItem struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	ECGI         EUTRANCGI
	iEExtensions extensionMeta `per:",exttable"`
}

type CompletedCellInTAIItem struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	ECGI         EUTRANCGI
	iEExtensions extensionMeta `per:",exttable"`
}

type TAIBroadcastItem struct {
	extensions         per.ExtensionAdditions `per:",extraw"`
	TAI                TAI
	CompletedCellInTAI []CompletedCellInTAIItem `per:"SEQUENCE-OF,size:1..65535"`
	iEExtensions       extensionMeta            `per:",exttable"`
}

type CompletedCellInEAIItem struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	ECGI         EUTRANCGI
	iEExtensions extensionMeta `per:",exttable"`
}

type EmergencyAreaIDBroadcastItem struct {
	extensions         per.ExtensionAdditions `per:",extraw"`
	EmergencyAreaID    EmergencyAreaID
	CompletedCellInEAI []CompletedCellInEAIItem `per:"SEQUENCE-OF,size:1..65535"`
	iEExtensions       extensionMeta            `per:",exttable"`
}

type CellIDCancelledItem struct {
	extensions         per.ExtensionAdditions `per:",extraw"`
	ECGI               EUTRANCGI
	NumberOfBroadcasts uint16        `per:",range:0..65535"`
	iEExtensions       extensionMeta `per:",exttable"`
}

type CancelledCellInTAIItem struct {
	extensions         per.ExtensionAdditions `per:",extraw"`
	ECGI               EUTRANCGI
	NumberOfBroadcasts uint16        `per:",range:0..65535"`
	iEExtensions       extensionMeta `per:",exttable"`
}

type TAICancelledItem struct {
	extensions         per.ExtensionAdditions `per:",extraw"`
	TAI                TAI
	CancelledCellInTAI []CancelledCellInTAIItem `per:"SEQUENCE-OF,size:1..65535"`
	iEExtensions       extensionMeta            `per:",exttable"`
}

type CancelledCellInEAIItem struct {
	extensions         per.ExtensionAdditions `per:",extraw"`
	ECGI               EUTRANCGI
	NumberOfBroadcasts uint16        `per:",range:0..65535"`
	iEExtensions       extensionMeta `per:",exttable"`
}

type EmergencyAreaIDCancelledItem struct {
	extensions         per.ExtensionAdditions `per:",extraw"`
	EmergencyAreaID    EmergencyAreaID
	CancelledCellInEAI []CancelledCellInEAIItem `per:"SEQUENCE-OF,size:1..65535"`
	iEExtensions       extensionMeta            `per:",exttable"`
}

type WarningAreaList struct {
	CellIDList                 *ECGIList                 `per:",choice:0"`
	TrackingAreaListforWarning *TAIListforWarning        `per:",choice:1"`
	EmergencyAreaIDList        *EmergencyAreaIDList      `per:",choice:2"`
	Ext                        *per.ExtensionAlternative `per:",extalt"`
}

type BroadcastCompletedAreaList struct {
	CellIDBroadcast          *CellIDBroadcast          `per:",choice:0"`
	TAIBroadcast             *TAIBroadcast             `per:",choice:1"`
	EmergencyAreaIDBroadcast *EmergencyAreaIDBroadcast `per:",choice:2"`
	Ext                      *per.ExtensionAlternative `per:",extalt"`
}

type BroadcastCancelledAreaList struct {
	CellIDCancelled          *CellIDCancelled          `per:",choice:0"`
	TAICancelled             *TAICancelled             `per:",choice:1"`
	EmergencyAreaIDCancelled *EmergencyAreaIDCancelled `per:",choice:2"`
	Ext                      *per.ExtensionAlternative `per:",extalt"`
}

type ECGIList []EUTRANCGI

func (l ECGIList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellID, []EUTRANCGI(l))
}

func (l *ECGIList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[EUTRANCGI](r, enc, 1, maxnoofCellID)
	*l = items

	return err
}

type TAIListforWarning []TAI

func (l TAIListforWarning) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTAIforWarning, []TAI(l))
}

func (l *TAIListforWarning) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAI](r, enc, 1, maxnoofTAIforWarning)
	*l = items

	return err
}

type EmergencyAreaIDList []EmergencyAreaID

func (l EmergencyAreaIDList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEmergencyAreaID, []EmergencyAreaID(l))
}

func (l *EmergencyAreaIDList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[EmergencyAreaID](r, enc, 1, maxnoofEmergencyAreaID)
	*l = items

	return err
}

type CellIDBroadcast []CellIDBroadcastItem

func (l CellIDBroadcast) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellID, []CellIDBroadcastItem(l))
}

func (l *CellIDBroadcast) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[CellIDBroadcastItem](r, enc, 1, maxnoofCellID)
	*l = items

	return err
}

type TAIBroadcast []TAIBroadcastItem

func (l TAIBroadcast) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTAIforWarning, []TAIBroadcastItem(l))
}

func (l *TAIBroadcast) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAIBroadcastItem](r, enc, 1, maxnoofTAIforWarning)
	*l = items

	return err
}

type EmergencyAreaIDBroadcast []EmergencyAreaIDBroadcastItem

func (l EmergencyAreaIDBroadcast) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEmergencyAreaID, []EmergencyAreaIDBroadcastItem(l))
}

func (l *EmergencyAreaIDBroadcast) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[EmergencyAreaIDBroadcastItem](r, enc, 1, maxnoofEmergencyAreaID)
	*l = items

	return err
}

type CellIDCancelled []CellIDCancelledItem

func (l CellIDCancelled) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellID, []CellIDCancelledItem(l))
}

func (l *CellIDCancelled) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[CellIDCancelledItem](r, enc, 1, maxnoofCellID)
	*l = items

	return err
}

type TAICancelled []TAICancelledItem

func (l TAICancelled) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTAIforWarning, []TAICancelledItem(l))
}

func (l *TAICancelled) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAICancelledItem](r, enc, 1, maxnoofTAIforWarning)
	*l = items

	return err
}

type EmergencyAreaIDCancelled []EmergencyAreaIDCancelledItem

func (l EmergencyAreaIDCancelled) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEmergencyAreaID, []EmergencyAreaIDCancelledItem(l))
}

func (l *EmergencyAreaIDCancelled) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[EmergencyAreaIDCancelledItem](r, enc, 1, maxnoofEmergencyAreaID)
	*l = items

	return err
}
