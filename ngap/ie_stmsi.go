// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

// FiveGTMSI ::= OCTET STRING (SIZE(4)).
type FiveGTMSI uint32

// FiveGSTMSI identifies a UE within an AMF set (TS 38.413 §9.3.3.20).
//
//	FiveG-S-TMSI ::= SEQUENCE {
//	    aMFSetID      AMFSetID,      -- BIT STRING (SIZE(10))
//	    aMFPointer    AMFPointer,    -- BIT STRING (SIZE(6))
//	    fiveG-TMSI    FiveG-TMSI,    -- OCTET STRING (SIZE(4))
//	    iE-Extensions ... OPTIONAL }
type FiveGSTMSI struct {
	_          [0]struct{} `per:"extseq"`
	AMFSetID   AMFSetID
	AMFPointer AMFPointer
	FiveGTMSI  FiveGTMSI
	_          ieExtensions `per:",skip"`
}
