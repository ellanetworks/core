// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

const (
	maxnoofCellIDforWarning = 65535
	maxnoofTAIforWarning    = 65535
	maxnoofEmergencyAreaID  = 65535
	maxnoofCellinTAI        = 65535
	maxnoofCellinEAI        = 65535
	maxnoofCellsinngeNB     = 256
	maxnoofCellsingNB       = 16384
)

const (
	messageIdentifierBits = 16
	serialNumberBits      = 16
	dataCodingSchemeBits  = 8
	repetitionPeriodMax   = 131071
	warningMessageMaxLen  = 9600
	warningCoordinatesMax = 1024
	warningSecurityLen    = 50
)

type NRCellIdentity uint64

func (c NRCellIdentity) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(c), NRCellIdentityBits)
}

func (c *NRCellIdentity) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, NRCellIdentityBits)
	if err != nil {
		return err
	}

	*c = NRCellIdentity(v)

	return nil
}

type EUTRACellIdentity uint32

func (c EUTRACellIdentity) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(c), EUTRACellIdentityBits)
}

func (c *EUTRACellIdentity) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, EUTRACellIdentityBits)
	if err != nil {
		return err
	}

	*c = EUTRACellIdentity(v)

	return nil
}

type NRCGI struct {
	extensions     per.ExtensionAdditions `per:",extraw"`
	PLMNIdentity   PLMNIdentity
	NRCellIdentity NRCellIdentity
	iEExtensions   extensionMeta `per:",exttable"`
}

type EUTRACGI struct {
	extensions        per.ExtensionAdditions `per:",extraw"`
	PLMNIdentity      PLMNIdentity
	EUTRACellIdentity EUTRACellIdentity
	iEExtensions      extensionMeta `per:",exttable"`
}

type MessageIdentifier uint16

func (m MessageIdentifier) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(m), messageIdentifierBits)
}

func (m *MessageIdentifier) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, messageIdentifierBits)
	if err != nil {
		return err
	}

	*m = MessageIdentifier(v)

	return nil
}

type SerialNumber uint16

func (s SerialNumber) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(s), serialNumberBits)
}

func (s *SerialNumber) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, serialNumberBits)
	if err != nil {
		return err
	}

	*s = SerialNumber(v)

	return nil
}

type RepetitionPeriod uint32

func (p RepetitionPeriod) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: repetitionPeriodMax, HasUB: true}, int64(p))
}

func (p *RepetitionPeriod) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: repetitionPeriodMax, HasUB: true})
	if err != nil {
		return err
	}

	*p = RepetitionPeriod(v)

	return nil
}

type NumberOfBroadcastsRequested uint16

func (n NumberOfBroadcastsRequested) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 65535, HasUB: true}, int64(n))
}

func (n *NumberOfBroadcastsRequested) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 65535, HasUB: true})
	if err != nil {
		return err
	}

	*n = NumberOfBroadcastsRequested(v)

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

type WarningSecurityInfo [warningSecurityLen]byte

func (s WarningSecurityInfo) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, warningSecurityLen, warningSecurityLen, true, true, false, s[:])
}

func (s *WarningSecurityInfo) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, warningSecurityLen, warningSecurityLen, true, true, false)
	if err != nil {
		return err
	}

	copy(s[:], b)

	return nil
}

type DataCodingScheme uint8

func (d DataCodingScheme) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(d), dataCodingSchemeBits)
}

func (d *DataCodingScheme) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, dataCodingSchemeBits)
	if err != nil {
		return err
	}

	*d = DataCodingScheme(v)

	return nil
}

type WarningMessageContents []byte

func (c WarningMessageContents) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 1, warningMessageMaxLen, true, true, false, c)
}

func (c *WarningMessageContents) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 1, warningMessageMaxLen, true, true, false)
	if err != nil {
		return err
	}

	*c = b

	return nil
}

type WarningAreaCoordinates []byte

func (c WarningAreaCoordinates) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 1, warningCoordinatesMax, true, true, false, c)
}

func (c *WarningAreaCoordinates) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 1, warningCoordinatesMax, true, true, false)
	if err != nil {
		return err
	}

	*c = b

	return nil
}

type ConcurrentWarningMessageInd uint8

const (
	ConcurrentWarningMessageIndTrue ConcurrentWarningMessageInd = iota
)

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

type WarningAreaList struct {
	EUTRACGIListForWarning *EUTRACGIListForWarning `per:",choice:0"`
	NRCGIListForWarning    *NRCGIListForWarning    `per:",choice:1"`
	TAIListForWarning      *TAIListForWarning      `per:",choice:2"`
	EmergencyAreaIDList    *EmergencyAreaIDList    `per:",choice:3"`
	ChoiceExtension        *SingleContainer        `per:",choice:4,extie"`
}

type EUTRACGIListForWarning []EUTRACGI

func (l EUTRACGIListForWarning) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellIDforWarning, []EUTRACGI(l))
}

func (l *EUTRACGIListForWarning) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[EUTRACGI](r, enc, 1, maxnoofCellIDforWarning)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type NRCGIListForWarning []NRCGI

func (l NRCGIListForWarning) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellIDforWarning, []NRCGI(l))
}

func (l *NRCGIListForWarning) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[NRCGI](r, enc, 1, maxnoofCellIDforWarning)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type TAIListForWarning []TAI

func (l TAIListForWarning) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTAIforWarning, []TAI(l))
}

func (l *TAIListForWarning) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAI](r, enc, 1, maxnoofTAIforWarning)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type EmergencyAreaIDList []EmergencyAreaID

func (l EmergencyAreaIDList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEmergencyAreaID, []EmergencyAreaID(l))
}

func (l *EmergencyAreaIDList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[EmergencyAreaID](r, enc, 1, maxnoofEmergencyAreaID)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type CellIDBroadcastEUTRAItem struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	EUTRACGI     EUTRACGI
	iEExtensions extensionMeta `per:",exttable"`
}

type CompletedCellsInTAIEUTRAItem struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	EUTRACGI     EUTRACGI
	iEExtensions extensionMeta `per:",exttable"`
}

type TAIBroadcastEUTRAItem struct {
	extensions          per.ExtensionAdditions `per:",extraw"`
	TAI                 TAI
	CompletedCellsInTAI []CompletedCellsInTAIEUTRAItem `per:"SEQUENCE-OF,size:1..65535"`
	iEExtensions        extensionMeta                  `per:",exttable"`
}

type CompletedCellsInEAIEUTRAItem struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	EUTRACGI     EUTRACGI
	iEExtensions extensionMeta `per:",exttable"`
}

type EmergencyAreaIDBroadcastEUTRAItem struct {
	extensions          per.ExtensionAdditions `per:",extraw"`
	EmergencyAreaID     EmergencyAreaID
	CompletedCellsInEAI []CompletedCellsInEAIEUTRAItem `per:"SEQUENCE-OF,size:1..65535"`
	iEExtensions        extensionMeta                  `per:",exttable"`
}

type CellIDBroadcastNRItem struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	NRCGI        NRCGI
	iEExtensions extensionMeta `per:",exttable"`
}

type CompletedCellsInTAINRItem struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	NRCGI        NRCGI
	iEExtensions extensionMeta `per:",exttable"`
}

type TAIBroadcastNRItem struct {
	extensions          per.ExtensionAdditions `per:",extraw"`
	TAI                 TAI
	CompletedCellsInTAI []CompletedCellsInTAINRItem `per:"SEQUENCE-OF,size:1..65535"`
	iEExtensions        extensionMeta               `per:",exttable"`
}

type CompletedCellsInEAINRItem struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	NRCGI        NRCGI
	iEExtensions extensionMeta `per:",exttable"`
}

type EmergencyAreaIDBroadcastNRItem struct {
	extensions          per.ExtensionAdditions `per:",extraw"`
	EmergencyAreaID     EmergencyAreaID
	CompletedCellsInEAI []CompletedCellsInEAINRItem `per:"SEQUENCE-OF,size:1..65535"`
	iEExtensions        extensionMeta               `per:",exttable"`
}

type BroadcastCompletedAreaList struct {
	CellIDBroadcastEUTRA          *CellIDBroadcastEUTRA          `per:",choice:0"`
	TAIBroadcastEUTRA             *TAIBroadcastEUTRA             `per:",choice:1"`
	EmergencyAreaIDBroadcastEUTRA *EmergencyAreaIDBroadcastEUTRA `per:",choice:2"`
	CellIDBroadcastNR             *CellIDBroadcastNR             `per:",choice:3"`
	TAIBroadcastNR                *TAIBroadcastNR                `per:",choice:4"`
	EmergencyAreaIDBroadcastNR    *EmergencyAreaIDBroadcastNR    `per:",choice:5"`
	ChoiceExtension               *SingleContainer               `per:",choice:6,extie"`
}

type CellIDBroadcastEUTRA []CellIDBroadcastEUTRAItem

func (l CellIDBroadcastEUTRA) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellIDforWarning, []CellIDBroadcastEUTRAItem(l))
}

func (l *CellIDBroadcastEUTRA) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[CellIDBroadcastEUTRAItem](r, enc, 1, maxnoofCellIDforWarning)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type TAIBroadcastEUTRA []TAIBroadcastEUTRAItem

func (l TAIBroadcastEUTRA) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTAIforWarning, []TAIBroadcastEUTRAItem(l))
}

func (l *TAIBroadcastEUTRA) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAIBroadcastEUTRAItem](r, enc, 1, maxnoofTAIforWarning)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type EmergencyAreaIDBroadcastEUTRA []EmergencyAreaIDBroadcastEUTRAItem

func (l EmergencyAreaIDBroadcastEUTRA) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEmergencyAreaID, []EmergencyAreaIDBroadcastEUTRAItem(l))
}

func (l *EmergencyAreaIDBroadcastEUTRA) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[EmergencyAreaIDBroadcastEUTRAItem](r, enc, 1, maxnoofEmergencyAreaID)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type CellIDBroadcastNR []CellIDBroadcastNRItem

func (l CellIDBroadcastNR) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellIDforWarning, []CellIDBroadcastNRItem(l))
}

func (l *CellIDBroadcastNR) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[CellIDBroadcastNRItem](r, enc, 1, maxnoofCellIDforWarning)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type TAIBroadcastNR []TAIBroadcastNRItem

func (l TAIBroadcastNR) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTAIforWarning, []TAIBroadcastNRItem(l))
}

func (l *TAIBroadcastNR) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAIBroadcastNRItem](r, enc, 1, maxnoofTAIforWarning)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type EmergencyAreaIDBroadcastNR []EmergencyAreaIDBroadcastNRItem

func (l EmergencyAreaIDBroadcastNR) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEmergencyAreaID, []EmergencyAreaIDBroadcastNRItem(l))
}

func (l *EmergencyAreaIDBroadcastNR) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[EmergencyAreaIDBroadcastNRItem](r, enc, 1, maxnoofEmergencyAreaID)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type CancelAllWarningMessages uint8

const (
	CancelAllWarningMessagesTrue CancelAllWarningMessages = iota
)

type CellIDCancelledEUTRAItem struct {
	extensions         per.ExtensionAdditions `per:",extraw"`
	EUTRACGI           EUTRACGI
	NumberOfBroadcasts uint16        `per:",range:0..65535"`
	iEExtensions       extensionMeta `per:",exttable"`
}

type CancelledCellsInTAIEUTRAItem struct {
	extensions         per.ExtensionAdditions `per:",extraw"`
	EUTRACGI           EUTRACGI
	NumberOfBroadcasts uint16        `per:",range:0..65535"`
	iEExtensions       extensionMeta `per:",exttable"`
}

type TAICancelledEUTRAItem struct {
	extensions          per.ExtensionAdditions `per:",extraw"`
	TAI                 TAI
	CancelledCellsInTAI []CancelledCellsInTAIEUTRAItem `per:"SEQUENCE-OF,size:1..65535"`
	iEExtensions        extensionMeta                  `per:",exttable"`
}

type CancelledCellsInEAIEUTRAItem struct {
	extensions         per.ExtensionAdditions `per:",extraw"`
	EUTRACGI           EUTRACGI
	NumberOfBroadcasts uint16        `per:",range:0..65535"`
	iEExtensions       extensionMeta `per:",exttable"`
}

type EmergencyAreaIDCancelledEUTRAItem struct {
	extensions          per.ExtensionAdditions `per:",extraw"`
	EmergencyAreaID     EmergencyAreaID
	CancelledCellsInEAI []CancelledCellsInEAIEUTRAItem `per:"SEQUENCE-OF,size:1..65535"`
	iEExtensions        extensionMeta                  `per:",exttable"`
}

type CellIDCancelledNRItem struct {
	extensions         per.ExtensionAdditions `per:",extraw"`
	NRCGI              NRCGI
	NumberOfBroadcasts uint16        `per:",range:0..65535"`
	iEExtensions       extensionMeta `per:",exttable"`
}

type CancelledCellsInTAINRItem struct {
	extensions         per.ExtensionAdditions `per:",extraw"`
	NRCGI              NRCGI
	NumberOfBroadcasts uint16        `per:",range:0..65535"`
	iEExtensions       extensionMeta `per:",exttable"`
}

type TAICancelledNRItem struct {
	extensions          per.ExtensionAdditions `per:",extraw"`
	TAI                 TAI
	CancelledCellsInTAI []CancelledCellsInTAINRItem `per:"SEQUENCE-OF,size:1..65535"`
	iEExtensions        extensionMeta               `per:",exttable"`
}

type CancelledCellsInEAINRItem struct {
	extensions         per.ExtensionAdditions `per:",extraw"`
	NRCGI              NRCGI
	NumberOfBroadcasts uint16        `per:",range:0..65535"`
	iEExtensions       extensionMeta `per:",exttable"`
}

type EmergencyAreaIDCancelledNRItem struct {
	extensions          per.ExtensionAdditions `per:",extraw"`
	EmergencyAreaID     EmergencyAreaID
	CancelledCellsInEAI []CancelledCellsInEAINRItem `per:"SEQUENCE-OF,size:1..65535"`
	iEExtensions        extensionMeta               `per:",exttable"`
}

type BroadcastCancelledAreaList struct {
	CellIDCancelledEUTRA          *CellIDCancelledEUTRA          `per:",choice:0"`
	TAICancelledEUTRA             *TAICancelledEUTRA             `per:",choice:1"`
	EmergencyAreaIDCancelledEUTRA *EmergencyAreaIDCancelledEUTRA `per:",choice:2"`
	CellIDCancelledNR             *CellIDCancelledNR             `per:",choice:3"`
	TAICancelledNR                *TAICancelledNR                `per:",choice:4"`
	EmergencyAreaIDCancelledNR    *EmergencyAreaIDCancelledNR    `per:",choice:5"`
	ChoiceExtension               *SingleContainer               `per:",choice:6,extie"`
}

type CellIDCancelledEUTRA []CellIDCancelledEUTRAItem

func (l CellIDCancelledEUTRA) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellIDforWarning, []CellIDCancelledEUTRAItem(l))
}

func (l *CellIDCancelledEUTRA) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[CellIDCancelledEUTRAItem](r, enc, 1, maxnoofCellIDforWarning)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type TAICancelledEUTRA []TAICancelledEUTRAItem

func (l TAICancelledEUTRA) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTAIforWarning, []TAICancelledEUTRAItem(l))
}

func (l *TAICancelledEUTRA) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAICancelledEUTRAItem](r, enc, 1, maxnoofTAIforWarning)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type EmergencyAreaIDCancelledEUTRA []EmergencyAreaIDCancelledEUTRAItem

func (l EmergencyAreaIDCancelledEUTRA) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEmergencyAreaID, []EmergencyAreaIDCancelledEUTRAItem(l))
}

func (l *EmergencyAreaIDCancelledEUTRA) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[EmergencyAreaIDCancelledEUTRAItem](r, enc, 1, maxnoofEmergencyAreaID)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type CellIDCancelledNR []CellIDCancelledNRItem

func (l CellIDCancelledNR) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellIDforWarning, []CellIDCancelledNRItem(l))
}

func (l *CellIDCancelledNR) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[CellIDCancelledNRItem](r, enc, 1, maxnoofCellIDforWarning)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type TAICancelledNR []TAICancelledNRItem

func (l TAICancelledNR) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTAIforWarning, []TAICancelledNRItem(l))
}

func (l *TAICancelledNR) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAICancelledNRItem](r, enc, 1, maxnoofTAIforWarning)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type EmergencyAreaIDCancelledNR []EmergencyAreaIDCancelledNRItem

func (l EmergencyAreaIDCancelledNR) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEmergencyAreaID, []EmergencyAreaIDCancelledNRItem(l))
}

func (l *EmergencyAreaIDCancelledNR) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[EmergencyAreaIDCancelledNRItem](r, enc, 1, maxnoofEmergencyAreaID)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

const (
	maxnoofTAIforRestart = 2048
	maxnoofEAIforRestart = 256
)

type CellIDListForRestart struct {
	EUTRACGIListforRestart *EUTRACGIList    `per:",choice:0"`
	NRCGIListforRestart    *NRCGIList       `per:",choice:1"`
	ChoiceExtension        *SingleContainer `per:",choice:2,extie"`
}

type EUTRACGIList []EUTRACGI

func (l EUTRACGIList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellsinngeNB, []EUTRACGI(l))
}

func (l *EUTRACGIList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[EUTRACGI](r, enc, 1, maxnoofCellsinngeNB)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type NRCGIList []NRCGI

func (l NRCGIList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellsingNB, []NRCGI(l))
}

func (l *NRCGIList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[NRCGI](r, enc, 1, maxnoofCellsingNB)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type TAIListForRestart []TAI

func (l TAIListForRestart) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTAIforRestart, []TAI(l))
}

func (l *TAIListForRestart) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAI](r, enc, 1, maxnoofTAIforRestart)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type EmergencyAreaIDListForRestart []EmergencyAreaID

func (l EmergencyAreaIDListForRestart) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEAIforRestart, []EmergencyAreaID(l))
}

func (l *EmergencyAreaIDListForRestart) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[EmergencyAreaID](r, enc, 1, maxnoofEAIforRestart)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type PWSFailedCellIDList struct {
	EUTRACGIPWSFailedList *EUTRACGIList    `per:",choice:0"`
	NRCGIPWSFailedList    *NRCGIList       `per:",choice:1"`
	ChoiceExtension       *SingleContainer `per:",choice:2,extie"`
}
