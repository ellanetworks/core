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
	extensions   per.ExtensionAdditions `per:",extraw"`
	PLMNIdentity PLMNIdentity
	AMFRegionID  AMFRegionID
	AMFSetID     AMFSetID
	AMFPointer   AMFPointer
	iEExtensions extensionMeta `per:",exttable"`
}

type GUAMIType uint8

const (
	GUAMITypeNative GUAMIType = iota
	GUAMITypeMapped
)

// ServedGUAMIItem ::= SEQUENCE { gUAMI, backupAMFName OPTIONAL, iE-Extensions
// OPTIONAL } (extensible).
type ServedGUAMIItem struct {
	GUAMI                 GUAMI
	BackupAMFName         *Name                  `per:",optional"`
	GUAMIType             *GUAMIType             `per:",extie:IDGUAMIType,crit:ignore"`
	ExtendedBackupAMFName *ExtendedAMFName       `per:",extie:IDExtendedBackupAMFName,crit:ignore"`
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
}

// ServedGUAMIList ::= SEQUENCE (SIZE(1..maxnoofServedGUAMIs)) OF
// ServedGUAMIItem.
type ServedGUAMIList []ServedGUAMIItem
