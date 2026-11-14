// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "github.com/ellanetworks/core/per"

// maxnoofServedGUAMIs bounds ServedGUAMIList (TS 38.413, NGAP-Constants).
const maxnoofServedGUAMIs = 256

// GUAMI bit-string widths (TS 38.413 §9.3.3.3). Not octet multiples, so the
// fields are held as integers.
const (
	amfRegionIDBits = 8
	amfSetIDBits    = 10
	amfPointerBits  = 6
)

// AMFRegionID ::= BIT STRING (SIZE(8)).
type AMFRegionID uint8

// AMFSetID ::= BIT STRING (SIZE(10)).
type AMFSetID uint16

// AMFPointer ::= BIT STRING (SIZE(6)).
type AMFPointer uint8

// GUAMI ::= SEQUENCE { pLMNIdentity, aMFRegionID, aMFSetID, aMFPointer,
// iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.3.3.
type GUAMI struct {
	_            [0]struct{} `per:"extseq"`
	PLMNIdentity PLMNIdentity
	AMFRegionID  AMFRegionID
	AMFSetID     AMFSetID
	AMFPointer   AMFPointer
	_            ieExtensions `per:",skip"`
}

type GUAMIType uint8

const (
	GUAMITypeNative GUAMIType = iota
	GUAMITypeMapped

	guamiTypeRootCount = 2
)

func (g GUAMIType) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, guamiTypeRootCount, int64(g), "GUAMIType")
}

func (g *GUAMIType) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, guamiTypeRootCount, "GUAMIType")
	if err != nil {
		return err
	}

	*g = GUAMIType(idx)

	return nil
}

// ServedGUAMIItem ::= SEQUENCE { gUAMI, backupAMFName OPTIONAL, iE-Extensions
// OPTIONAL } (extensible).
type ServedGUAMIItem struct {
	GUAMI                 GUAMI
	BackupAMFName         *Name
	GUAMIType             *GUAMIType
	ExtendedBackupAMFName *ExtendedAMFName

	iEExtensions messageMeta
	extensions   per.ExtensionAdditions
}

var servedGUAMIItemExtIEs = []ieSpec[ServedGUAMIItem]{
	{
		id: IDGUAMIType, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ServedGUAMIItem, raw []byte, enc per.Encoding) error {
			var v GUAMIType

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.GUAMIType = &v

			return nil
		},
		encode: func(m *ServedGUAMIItem) (per.Marshaler, bool) {
			if m.GUAMIType == nil {
				return nil, false
			}

			return m.GUAMIType, true
		},
	},
	{
		id: IDExtendedBackupAMFName, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ServedGUAMIItem, raw []byte, enc per.Encoding) error {
			var v ExtendedAMFName

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.ExtendedBackupAMFName = &v

			return nil
		},
		encode: func(m *ServedGUAMIItem) (per.Marshaler, bool) {
			if m.ExtendedBackupAMFName == nil {
				return nil, false
			}

			return m.ExtendedBackupAMFName, true
		},
	},
}

func (s *ServedGUAMIItem) MarshalPER(w *per.Writer, enc per.Encoding) error {
	ext := extensionFields(servedGUAMIItemExtIEs, s, &s.iEExtensions)

	w.WriteBit(s.extensions.Present())
	w.WriteBit(s.BackupAMFName != nil)
	w.WriteBit(len(ext) > 0)

	if err := s.GUAMI.MarshalPER(w, enc); err != nil {
		return err
	}

	if s.BackupAMFName != nil {
		if err := s.BackupAMFName.MarshalPER(w, enc); err != nil {
			return err
		}
	}

	if len(ext) > 0 {
		if err := encodeExtensionContainer(w, enc, ext); err != nil {
			return err
		}
	}

	if s.extensions.Present() {
		return s.extensions.MarshalPER(w, enc)
	}

	return nil
}

func (s *ServedGUAMIItem) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	extBit, err := r.ReadBit()
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

	if err := s.GUAMI.UnmarshalPER(r, enc); err != nil {
		return err
	}

	if backupAMFName {
		var v Name

		if err := v.UnmarshalPER(r, enc); err != nil {
			return err
		}

		s.BackupAMFName = &v
	}

	if extContainer {
		if err := decodeExtensionContainer(r, enc, servedGUAMIItemExtIEs, s, &s.iEExtensions); err != nil {
			return err
		}
	}

	if extBit {
		return s.extensions.UnmarshalPER(r, enc)
	}

	return nil
}

// ServedGUAMIList ::= SEQUENCE (SIZE(1..maxnoofServedGUAMIs)) OF
// ServedGUAMIItem.
type ServedGUAMIList []ServedGUAMIItem
