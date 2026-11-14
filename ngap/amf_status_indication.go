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
	GUAMI                        GUAMI
	TimerApproachForGUAMIRemoval *TimerApproachForGUAMIRemoval
	// The AMF to reselect towards (§8.7.6.2).
	BackupAMFName         *Name
	ExtendedBackupAMFName *ExtendedAMFName

	iEExtensions messageMeta
	extensions   per.ExtensionAdditions
}

var unavailableGUAMIItemExtIEs = []ieSpec[UnavailableGUAMIItem]{
	{
		id: IDExtendedBackupAMFName, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *UnavailableGUAMIItem, raw []byte, enc per.Encoding) error {
			var v ExtendedAMFName

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.ExtendedBackupAMFName = &v

			return nil
		},
		encode: func(m *UnavailableGUAMIItem) (per.Marshaler, bool) {
			if m.ExtendedBackupAMFName == nil {
				return nil, false
			}

			return m.ExtendedBackupAMFName, true
		},
	},
}

func (u *UnavailableGUAMIItem) MarshalPER(w *per.Writer, enc per.Encoding) error {
	ext := extensionFields(unavailableGUAMIItemExtIEs, u, &u.iEExtensions)

	w.WriteBit(u.extensions.Present())
	w.WriteBit(u.TimerApproachForGUAMIRemoval != nil)
	w.WriteBit(u.BackupAMFName != nil)
	w.WriteBit(len(ext) > 0)

	if err := u.GUAMI.MarshalPER(w, enc); err != nil {
		return err
	}

	if u.TimerApproachForGUAMIRemoval != nil {
		if err := u.TimerApproachForGUAMIRemoval.MarshalPER(w, enc); err != nil {
			return err
		}
	}

	if u.BackupAMFName != nil {
		if err := u.BackupAMFName.MarshalPER(w, enc); err != nil {
			return err
		}
	}

	if len(ext) > 0 {
		if err := encodeExtensionContainer(w, enc, ext); err != nil {
			return err
		}
	}

	if u.extensions.Present() {
		return u.extensions.MarshalPER(w, enc)
	}

	return nil
}

func (u *UnavailableGUAMIItem) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	extBit, err := r.ReadBit()
	if err != nil {
		return err
	}

	timerApproach, err := r.ReadBit()
	if err != nil {
		return err
	}

	backupAMFName, err := r.ReadBit()
	if err != nil {
		return err
	}

	extContainer, err := r.ReadBit()
	if err != nil {
		return err
	}

	if err := u.GUAMI.UnmarshalPER(r, enc); err != nil {
		return err
	}

	if timerApproach {
		var v TimerApproachForGUAMIRemoval

		if err := v.UnmarshalPER(r, enc); err != nil {
			return err
		}

		u.TimerApproachForGUAMIRemoval = &v
	}

	if backupAMFName {
		var v Name

		if err := v.UnmarshalPER(r, enc); err != nil {
			return err
		}

		u.BackupAMFName = &v
	}

	if extContainer {
		if err := decodeExtensionContainer(r, enc, unavailableGUAMIItemExtIEs, u, &u.iEExtensions); err != nil {
			return err
		}
	}

	if extBit {
		return u.extensions.UnmarshalPER(r, enc)
	}

	return nil
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
		id: IDUnavailableGUAMIList, presence: presenceMandatory, crit: CriticalityReject,
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
