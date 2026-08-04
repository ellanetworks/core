// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// TimerApproachForGUAMIRemoval ::= ENUMERATED { apply-timer, ... }, defined
// inline in TS 38.413 §9.2.6.10. Present means keep the GUAMI for the guard
// period rather than dropping it at once (TS 23.501).
type TimerApproachForGUAMIRemoval uint8

const (
	TimerApproachApplyTimer TimerApproachForGUAMIRemoval = iota

	timerApproachForGUAMIRemovalRootCount = 1
)

func (t TimerApproachForGUAMIRemoval) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, timerApproachForGUAMIRemovalRootCount, int64(t), "TimerApproachForGUAMIRemoval")
}

func (t *TimerApproachForGUAMIRemoval) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeRootEnumerated(r, enc, timerApproachForGUAMIRemovalRootCount, "TimerApproachForGUAMIRemoval")
	if err != nil {
		return err
	}

	*t = TimerApproachForGUAMIRemoval(v)

	return nil
}

// UnavailableGUAMIItem ::= SEQUENCE { gUAMI, timerApproachForGUAMIRemoval
// OPTIONAL, backupAMFName OPTIONAL, iE-Extensions OPTIONAL } (extensible),
// defined inline in TS 38.413 §9.2.6.10.
type UnavailableGUAMIItem struct {
	_                            [0]struct{} `per:"extseq"`
	GUAMI                        GUAMI
	TimerApproachForGUAMIRemoval *TimerApproachForGUAMIRemoval `per:",optional"`
	// The AMF to reselect towards (§8.7.6.2).
	BackupAMFName *Name        `per:",optional"`
	_             ieExtensions `per:",skip"`
}

// UnavailableGUAMIList ::= SEQUENCE (SIZE(1..maxnoofServedGUAMIs)) OF
// UnavailableGUAMIItem.
type UnavailableGUAMIList []UnavailableGUAMIItem

// TS 38.413 §9.2.6.10. The one IE is mandatory with reject criticality, so it
// is a value type.
type AMFStatusIndication struct {
	UnavailableGUAMIList UnavailableGUAMIList

	messageMeta
}

var aMFStatusIndicationIEs = []ieSpec[AMFStatusIndication]{
	{
		id: idUnavailableGUAMIList, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *AMFStatusIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UnavailableGUAMIList)
		},
		encode: func(m *AMFStatusIndication) (per.Marshaler, bool) {
			return m.UnavailableGUAMIList, true
		},
	},
}

func (m *AMFStatusIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcAMFStatusIndication, aMFStatusIndicationIEs, m)
}

func (m *AMFStatusIndication) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcAMFStatusIndication,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseAMFStatusIndication(value []byte) (*AMFStatusIndication, error) {
	return parseMessageBody[AMFStatusIndication](ProcAMFStatusIndication, TriggeringInitiatingMessage, aMFStatusIndicationIEs, value)
}
