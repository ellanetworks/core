// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

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

// TS 38.413 §9.3.1.7.
type NRCGI struct {
	_              [0]struct{} `per:"extseq"`
	PLMNIdentity   PLMNIdentity
	NRCellIdentity NRCellIdentity
	_              ieExtensions `per:",skip"`
}

// TS 38.413 §9.3.1.9.
type EUTRACGI struct {
	_                 [0]struct{} `per:"extseq"`
	PLMNIdentity      PLMNIdentity
	EUTRACellIdentity EUTRACellIdentity
	_                 ieExtensions `per:",skip"`
}

// TS 38.413 §9.3.1.35.
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

// TS 38.413 §9.3.1.36.
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

// TS 38.413 §9.3.1.49.
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

// TS 38.413 §9.3.1.38.
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

// TS 38.413 §9.3.1.39.
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

// TS 38.413 §9.3.1.41.
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

// TS 38.413 §9.3.1.42.
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

// TS 38.413 §9.3.1.112.
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

// TS 38.413 §9.3.1.46.
type ConcurrentWarningMessageInd uint8

const (
	ConcurrentWarningMessageIndTrue ConcurrentWarningMessageInd = iota

	concurrentWarningMessageIndRootCount = 1
)

func (c ConcurrentWarningMessageInd) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, concurrentWarningMessageIndRootCount, int64(c), "ConcurrentWarningMessageInd")
}

func (c *ConcurrentWarningMessageInd) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeRootEnumerated(r, enc, concurrentWarningMessageIndRootCount, "ConcurrentWarningMessageInd")
	if err != nil {
		return err
	}

	*c = ConcurrentWarningMessageInd(v)

	return nil
}

// TS 38.413 §9.3.1.48.
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

func chosenAlternative(choice string, set ...bool) (int64, error) {
	idx := int64(-1)

	for i, s := range set {
		if !s {
			continue
		}

		if idx >= 0 {
			return 0, fmt.Errorf("ngap: %s has more than one alternative set", choice)
		}

		idx = int64(i)
	}

	if idx < 0 {
		return 0, fmt.Errorf("ngap: %s has no alternative set", choice)
	}

	return idx, nil
}

const (
	warningAreaListEUTRACGI = iota
	warningAreaListNRCGI
	warningAreaListTAI
	warningAreaListEmergencyAreaID
	warningAreaListChoiceExtensions

	warningAreaListAlternatives = 5
)

// TS 38.413 §9.3.1.37.
type WarningAreaList struct {
	EUTRACGIs        []EUTRACGI
	NRCGIs           []NRCGI
	TAIs             []TAI
	EmergencyAreaIDs []EmergencyAreaID
}

func (l WarningAreaList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	idx, err := chosenAlternative("WarningAreaList", len(l.EUTRACGIs) > 0, len(l.NRCGIs) > 0, len(l.TAIs) > 0, len(l.EmergencyAreaIDs) > 0)
	if err != nil {
		return err
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, warningAreaListAlternatives-1, idx); err != nil {
		return err
	}

	switch idx {
	case warningAreaListEUTRACGI:
		return marshalSeqOf(w, enc, 1, maxnoofCellIDforWarning, l.EUTRACGIs)
	case warningAreaListNRCGI:
		return marshalSeqOf(w, enc, 1, maxnoofCellIDforWarning, l.NRCGIs)
	case warningAreaListTAI:
		return marshalSeqOf(w, enc, 1, maxnoofTAIforWarning, l.TAIs)
	default:
		return marshalSeqOf(w, enc, 1, maxnoofEmergencyAreaID, l.EmergencyAreaIDs)
	}
}

func (l *WarningAreaList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, warningAreaListAlternatives-1)
	if err != nil {
		return fmt.Errorf("ngap: WarningAreaList choice: %w", err)
	}

	var v WarningAreaList

	switch idx {
	case warningAreaListEUTRACGI:
		v.EUTRACGIs, err = unmarshalSeqOf[EUTRACGI](r, enc, 1, maxnoofCellIDforWarning)
	case warningAreaListNRCGI:
		v.NRCGIs, err = unmarshalSeqOf[NRCGI](r, enc, 1, maxnoofCellIDforWarning)
	case warningAreaListTAI:
		v.TAIs, err = unmarshalSeqOf[TAI](r, enc, 1, maxnoofTAIforWarning)
	case warningAreaListEmergencyAreaID:
		v.EmergencyAreaIDs, err = unmarshalSeqOf[EmergencyAreaID](r, enc, 1, maxnoofEmergencyAreaID)
	case warningAreaListChoiceExtensions:
		return decodeChoiceExtension(r, enc, "WarningAreaList")
	}

	if err != nil {
		return err
	}

	*l = v

	return nil
}

type CellIDBroadcastEUTRAItem struct {
	_        [0]struct{} `per:"extseq"`
	EUTRACGI EUTRACGI
	_        ieExtensions `per:",skip"`
}

type CompletedCellsInTAIEUTRAItem struct {
	_        [0]struct{} `per:"extseq"`
	EUTRACGI EUTRACGI
	_        ieExtensions `per:",skip"`
}

type TAIBroadcastEUTRAItem struct {
	_                   [0]struct{} `per:"extseq"`
	TAI                 TAI
	CompletedCellsInTAI []CompletedCellsInTAIEUTRAItem `per:"SEQUENCE-OF,size:1..65535"`
	_                   ieExtensions                   `per:",skip"`
}

type CompletedCellsInEAIEUTRAItem struct {
	_        [0]struct{} `per:"extseq"`
	EUTRACGI EUTRACGI
	_        ieExtensions `per:",skip"`
}

type EmergencyAreaIDBroadcastEUTRAItem struct {
	_                   [0]struct{} `per:"extseq"`
	EmergencyAreaID     EmergencyAreaID
	CompletedCellsInEAI []CompletedCellsInEAIEUTRAItem `per:"SEQUENCE-OF,size:1..65535"`
	_                   ieExtensions                   `per:",skip"`
}

type CellIDBroadcastNRItem struct {
	_     [0]struct{} `per:"extseq"`
	NRCGI NRCGI
	_     ieExtensions `per:",skip"`
}

type CompletedCellsInTAINRItem struct {
	_     [0]struct{} `per:"extseq"`
	NRCGI NRCGI
	_     ieExtensions `per:",skip"`
}

type TAIBroadcastNRItem struct {
	_                   [0]struct{} `per:"extseq"`
	TAI                 TAI
	CompletedCellsInTAI []CompletedCellsInTAINRItem `per:"SEQUENCE-OF,size:1..65535"`
	_                   ieExtensions                `per:",skip"`
}

type CompletedCellsInEAINRItem struct {
	_     [0]struct{} `per:"extseq"`
	NRCGI NRCGI
	_     ieExtensions `per:",skip"`
}

type EmergencyAreaIDBroadcastNRItem struct {
	_                   [0]struct{} `per:"extseq"`
	EmergencyAreaID     EmergencyAreaID
	CompletedCellsInEAI []CompletedCellsInEAINRItem `per:"SEQUENCE-OF,size:1..65535"`
	_                   ieExtensions                `per:",skip"`
}

const (
	broadcastAreaCellIDEUTRA = iota
	broadcastAreaTAIEUTRA
	broadcastAreaEmergencyAreaIDEUTRA
	broadcastAreaCellIDNR
	broadcastAreaTAINR
	broadcastAreaEmergencyAreaIDNR
	broadcastAreaChoiceExtensions

	broadcastAreaAlternatives = 7
)

// TS 38.413 §9.3.1.43.
type BroadcastCompletedAreaList struct {
	CellIDBroadcastEUTRA          []CellIDBroadcastEUTRAItem
	TAIBroadcastEUTRA             []TAIBroadcastEUTRAItem
	EmergencyAreaIDBroadcastEUTRA []EmergencyAreaIDBroadcastEUTRAItem
	CellIDBroadcastNR             []CellIDBroadcastNRItem
	TAIBroadcastNR                []TAIBroadcastNRItem
	EmergencyAreaIDBroadcastNR    []EmergencyAreaIDBroadcastNRItem
}

func (l BroadcastCompletedAreaList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	idx, err := chosenAlternative("BroadcastCompletedAreaList",
		len(l.CellIDBroadcastEUTRA) > 0, len(l.TAIBroadcastEUTRA) > 0, len(l.EmergencyAreaIDBroadcastEUTRA) > 0,
		len(l.CellIDBroadcastNR) > 0, len(l.TAIBroadcastNR) > 0, len(l.EmergencyAreaIDBroadcastNR) > 0)
	if err != nil {
		return err
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, broadcastAreaAlternatives-1, idx); err != nil {
		return err
	}

	switch idx {
	case broadcastAreaCellIDEUTRA:
		return marshalSeqOf(w, enc, 1, maxnoofCellIDforWarning, l.CellIDBroadcastEUTRA)
	case broadcastAreaTAIEUTRA:
		return marshalSeqOf(w, enc, 1, maxnoofTAIforWarning, l.TAIBroadcastEUTRA)
	case broadcastAreaEmergencyAreaIDEUTRA:
		return marshalSeqOf(w, enc, 1, maxnoofEmergencyAreaID, l.EmergencyAreaIDBroadcastEUTRA)
	case broadcastAreaCellIDNR:
		return marshalSeqOf(w, enc, 1, maxnoofCellIDforWarning, l.CellIDBroadcastNR)
	case broadcastAreaTAINR:
		return marshalSeqOf(w, enc, 1, maxnoofTAIforWarning, l.TAIBroadcastNR)
	default:
		return marshalSeqOf(w, enc, 1, maxnoofEmergencyAreaID, l.EmergencyAreaIDBroadcastNR)
	}
}

func (l *BroadcastCompletedAreaList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, broadcastAreaAlternatives-1)
	if err != nil {
		return fmt.Errorf("ngap: BroadcastCompletedAreaList choice: %w", err)
	}

	var v BroadcastCompletedAreaList

	switch idx {
	case broadcastAreaCellIDEUTRA:
		v.CellIDBroadcastEUTRA, err = unmarshalSeqOf[CellIDBroadcastEUTRAItem](r, enc, 1, maxnoofCellIDforWarning)
	case broadcastAreaTAIEUTRA:
		v.TAIBroadcastEUTRA, err = unmarshalSeqOf[TAIBroadcastEUTRAItem](r, enc, 1, maxnoofTAIforWarning)
	case broadcastAreaEmergencyAreaIDEUTRA:
		v.EmergencyAreaIDBroadcastEUTRA, err = unmarshalSeqOf[EmergencyAreaIDBroadcastEUTRAItem](r, enc, 1, maxnoofEmergencyAreaID)
	case broadcastAreaCellIDNR:
		v.CellIDBroadcastNR, err = unmarshalSeqOf[CellIDBroadcastNRItem](r, enc, 1, maxnoofCellIDforWarning)
	case broadcastAreaTAINR:
		v.TAIBroadcastNR, err = unmarshalSeqOf[TAIBroadcastNRItem](r, enc, 1, maxnoofTAIforWarning)
	case broadcastAreaEmergencyAreaIDNR:
		v.EmergencyAreaIDBroadcastNR, err = unmarshalSeqOf[EmergencyAreaIDBroadcastNRItem](r, enc, 1, maxnoofEmergencyAreaID)
	case broadcastAreaChoiceExtensions:
		return decodeChoiceExtension(r, enc, "BroadcastCompletedAreaList")
	}

	if err != nil {
		return err
	}

	*l = v

	return nil
}

// TS 38.413 §9.3.1.47.
type CancelAllWarningMessages uint8

const (
	CancelAllWarningMessagesTrue CancelAllWarningMessages = iota

	cancelAllWarningMessagesRootCount = 1
)

func (c CancelAllWarningMessages) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, cancelAllWarningMessagesRootCount, int64(c), "CancelAllWarningMessages")
}

func (c *CancelAllWarningMessages) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeRootEnumerated(r, enc, cancelAllWarningMessagesRootCount, "CancelAllWarningMessages")
	if err != nil {
		return err
	}

	*c = CancelAllWarningMessages(v)

	return nil
}

type CellIDCancelledEUTRAItem struct {
	_                  [0]struct{} `per:"extseq"`
	EUTRACGI           EUTRACGI
	NumberOfBroadcasts uint16       `per:",range:0..65535"`
	_                  ieExtensions `per:",skip"`
}

type CancelledCellsInTAIEUTRAItem struct {
	_                  [0]struct{} `per:"extseq"`
	EUTRACGI           EUTRACGI
	NumberOfBroadcasts uint16       `per:",range:0..65535"`
	_                  ieExtensions `per:",skip"`
}

type TAICancelledEUTRAItem struct {
	_                   [0]struct{} `per:"extseq"`
	TAI                 TAI
	CancelledCellsInTAI []CancelledCellsInTAIEUTRAItem `per:"SEQUENCE-OF,size:1..65535"`
	_                   ieExtensions                   `per:",skip"`
}

type CancelledCellsInEAIEUTRAItem struct {
	_                  [0]struct{} `per:"extseq"`
	EUTRACGI           EUTRACGI
	NumberOfBroadcasts uint16       `per:",range:0..65535"`
	_                  ieExtensions `per:",skip"`
}

type EmergencyAreaIDCancelledEUTRAItem struct {
	_                   [0]struct{} `per:"extseq"`
	EmergencyAreaID     EmergencyAreaID
	CancelledCellsInEAI []CancelledCellsInEAIEUTRAItem `per:"SEQUENCE-OF,size:1..65535"`
	_                   ieExtensions                   `per:",skip"`
}

type CellIDCancelledNRItem struct {
	_                  [0]struct{} `per:"extseq"`
	NRCGI              NRCGI
	NumberOfBroadcasts uint16       `per:",range:0..65535"`
	_                  ieExtensions `per:",skip"`
}

type CancelledCellsInTAINRItem struct {
	_                  [0]struct{} `per:"extseq"`
	NRCGI              NRCGI
	NumberOfBroadcasts uint16       `per:",range:0..65535"`
	_                  ieExtensions `per:",skip"`
}

type TAICancelledNRItem struct {
	_                   [0]struct{} `per:"extseq"`
	TAI                 TAI
	CancelledCellsInTAI []CancelledCellsInTAINRItem `per:"SEQUENCE-OF,size:1..65535"`
	_                   ieExtensions                `per:",skip"`
}

type CancelledCellsInEAINRItem struct {
	_                  [0]struct{} `per:"extseq"`
	NRCGI              NRCGI
	NumberOfBroadcasts uint16       `per:",range:0..65535"`
	_                  ieExtensions `per:",skip"`
}

type EmergencyAreaIDCancelledNRItem struct {
	_                   [0]struct{} `per:"extseq"`
	EmergencyAreaID     EmergencyAreaID
	CancelledCellsInEAI []CancelledCellsInEAINRItem `per:"SEQUENCE-OF,size:1..65535"`
	_                   ieExtensions                `per:",skip"`
}

// TS 38.413 §9.3.1.44.
type BroadcastCancelledAreaList struct {
	CellIDCancelledEUTRA          []CellIDCancelledEUTRAItem
	TAICancelledEUTRA             []TAICancelledEUTRAItem
	EmergencyAreaIDCancelledEUTRA []EmergencyAreaIDCancelledEUTRAItem
	CellIDCancelledNR             []CellIDCancelledNRItem
	TAICancelledNR                []TAICancelledNRItem
	EmergencyAreaIDCancelledNR    []EmergencyAreaIDCancelledNRItem
}

func (l BroadcastCancelledAreaList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	idx, err := chosenAlternative("BroadcastCancelledAreaList",
		len(l.CellIDCancelledEUTRA) > 0, len(l.TAICancelledEUTRA) > 0, len(l.EmergencyAreaIDCancelledEUTRA) > 0,
		len(l.CellIDCancelledNR) > 0, len(l.TAICancelledNR) > 0, len(l.EmergencyAreaIDCancelledNR) > 0)
	if err != nil {
		return err
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, broadcastAreaAlternatives-1, idx); err != nil {
		return err
	}

	switch idx {
	case broadcastAreaCellIDEUTRA:
		return marshalSeqOf(w, enc, 1, maxnoofCellIDforWarning, l.CellIDCancelledEUTRA)
	case broadcastAreaTAIEUTRA:
		return marshalSeqOf(w, enc, 1, maxnoofTAIforWarning, l.TAICancelledEUTRA)
	case broadcastAreaEmergencyAreaIDEUTRA:
		return marshalSeqOf(w, enc, 1, maxnoofEmergencyAreaID, l.EmergencyAreaIDCancelledEUTRA)
	case broadcastAreaCellIDNR:
		return marshalSeqOf(w, enc, 1, maxnoofCellIDforWarning, l.CellIDCancelledNR)
	case broadcastAreaTAINR:
		return marshalSeqOf(w, enc, 1, maxnoofTAIforWarning, l.TAICancelledNR)
	default:
		return marshalSeqOf(w, enc, 1, maxnoofEmergencyAreaID, l.EmergencyAreaIDCancelledNR)
	}
}

func (l *BroadcastCancelledAreaList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, broadcastAreaAlternatives-1)
	if err != nil {
		return fmt.Errorf("ngap: BroadcastCancelledAreaList choice: %w", err)
	}

	var v BroadcastCancelledAreaList

	switch idx {
	case broadcastAreaCellIDEUTRA:
		v.CellIDCancelledEUTRA, err = unmarshalSeqOf[CellIDCancelledEUTRAItem](r, enc, 1, maxnoofCellIDforWarning)
	case broadcastAreaTAIEUTRA:
		v.TAICancelledEUTRA, err = unmarshalSeqOf[TAICancelledEUTRAItem](r, enc, 1, maxnoofTAIforWarning)
	case broadcastAreaEmergencyAreaIDEUTRA:
		v.EmergencyAreaIDCancelledEUTRA, err = unmarshalSeqOf[EmergencyAreaIDCancelledEUTRAItem](r, enc, 1, maxnoofEmergencyAreaID)
	case broadcastAreaCellIDNR:
		v.CellIDCancelledNR, err = unmarshalSeqOf[CellIDCancelledNRItem](r, enc, 1, maxnoofCellIDforWarning)
	case broadcastAreaTAINR:
		v.TAICancelledNR, err = unmarshalSeqOf[TAICancelledNRItem](r, enc, 1, maxnoofTAIforWarning)
	case broadcastAreaEmergencyAreaIDNR:
		v.EmergencyAreaIDCancelledNR, err = unmarshalSeqOf[EmergencyAreaIDCancelledNRItem](r, enc, 1, maxnoofEmergencyAreaID)
	case broadcastAreaChoiceExtensions:
		return decodeChoiceExtension(r, enc, "BroadcastCancelledAreaList")
	}

	if err != nil {
		return err
	}

	*l = v

	return nil
}

const (
	maxnoofTAIforRestart = 2048
	maxnoofEAIforRestart = 256
)

const (
	cellListEUTRA = iota
	cellListNR
	cellListChoiceExtensions

	cellListAlternatives = 3
)

func marshalCellList(w *per.Writer, enc per.Encoding, choice string, eutra []EUTRACGI, nr []NRCGI) error {
	idx, err := chosenAlternative(choice, len(eutra) > 0, len(nr) > 0)
	if err != nil {
		return err
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, cellListAlternatives-1, idx); err != nil {
		return err
	}

	if idx == cellListEUTRA {
		return marshalSeqOf(w, enc, 1, maxnoofCellsinngeNB, eutra)
	}

	return marshalSeqOf(w, enc, 1, maxnoofCellsingNB, nr)
}

func unmarshalCellList(r *per.Reader, enc per.Encoding, choice string) ([]EUTRACGI, []NRCGI, error) {
	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, cellListAlternatives-1)
	if err != nil {
		return nil, nil, fmt.Errorf("ngap: %s choice: %w", choice, err)
	}

	switch idx {
	case cellListEUTRA:
		eutra, err := unmarshalSeqOf[EUTRACGI](r, enc, 1, maxnoofCellsinngeNB)
		return eutra, nil, err
	case cellListNR:
		nr, err := unmarshalSeqOf[NRCGI](r, enc, 1, maxnoofCellsingNB)
		return nil, nr, err
	default:
		return nil, nil, decodeChoiceExtension(r, enc, choice)
	}
}

type CellIDListForRestart struct {
	EUTRACGIs []EUTRACGI
	NRCGIs    []NRCGI
}

func (l CellIDListForRestart) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalCellList(w, enc, "CellIDListForRestart", l.EUTRACGIs, l.NRCGIs)
}

func (l *CellIDListForRestart) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	eutra, nr, err := unmarshalCellList(r, enc, "CellIDListForRestart")
	if err != nil {
		return err
	}

	*l = CellIDListForRestart{EUTRACGIs: eutra, NRCGIs: nr}

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
	EUTRACGIs []EUTRACGI
	NRCGIs    []NRCGI
}

func (l PWSFailedCellIDList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalCellList(w, enc, "PWSFailedCellIDList", l.EUTRACGIs, l.NRCGIs)
}

func (l *PWSFailedCellIDList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	eutra, nr, err := unmarshalCellList(r, enc, "PWSFailedCellIDList")
	if err != nil {
		return err
	}

	*l = PWSFailedCellIDList{EUTRACGIs: eutra, NRCGIs: nr}

	return nil
}
