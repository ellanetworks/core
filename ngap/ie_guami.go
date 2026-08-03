// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

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

// ServedGUAMIItem ::= SEQUENCE { gUAMI, backupAMFName OPTIONAL, iE-Extensions
// OPTIONAL } (extensible).
type ServedGUAMIItem struct {
	_             [0]struct{} `per:"extseq"`
	GUAMI         GUAMI
	BackupAMFName *Name        `per:",optional"`
	_             ieExtensions `per:",skip"`
}

// ServedGUAMIList ::= SEQUENCE (SIZE(1..maxnoofServedGUAMIs)) OF
// ServedGUAMIItem.
type ServedGUAMIList []ServedGUAMIItem
