// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

// MTMSI ::= OCTET STRING (SIZE(4)).
type MTMSI uint32

// STMSI identifies a UE within an MME pool (TS 36.413 §9.2.3.6).
//
//	S-TMSI ::= SEQUENCE {
//	    mMEC    MME-Code,            -- OCTET STRING (SIZE(1))
//	    m-TMSI  OCTET STRING (SIZE(4)),
//	    iE-Extensions ... OPTIONAL }
type STMSI struct {
	_     [0]struct{} `per:"extseq"`
	MMEC  MMECode
	MTMSI MTMSI
	_     ieExtensions `per:",skip"`
}
