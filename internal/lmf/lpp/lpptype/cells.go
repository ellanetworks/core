// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

type ECGI struct {
	MCC          []MCCMNCDigit `per:"SEQUENCE-OF,size:3"`
	MNC          []MCCMNCDigit `per:"SEQUENCE-OF,size:2..3"`
	CellIdentity []bool        `per:",size:28"`
}

type MCCMNCDigit struct {
	Value int64 `per:",range:0..9"`
}

type PLMNIdentity struct {
	MCC []MCCMNCDigit `per:"SEQUENCE-OF,size:3"`
	MNC []MCCMNCDigit `per:"SEQUENCE-OF,size:2..3"`
}

type CellGlobalIdEUTRAAndUTRA struct {
	_            [0]struct{} `per:"extseq"`
	PLMNIdentity PLMNIdentity
	CellIdentity CellGlobalIdEUTRAAndUTRACellIdentity
}

type CellGlobalIdEUTRAAndUTRACellIdentity struct {
	EUTRA *[]bool `per:",choice:0,optional,size:28"`
	UTRA  *[]bool `per:",choice:1,optional,size:32"`
}

type CellGlobalIdGERAN struct {
	_                [0]struct{} `per:"extseq"`
	PLMNIdentity     PLMNIdentity
	LocationAreaCode []bool `per:",size:16"`
	CellIdentity     []bool `per:",size:16"`
}
