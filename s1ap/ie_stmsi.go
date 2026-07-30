// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/binary"

	"github.com/ellanetworks/core/per"
)

// STMSI is the S-TMSI IE (TS 36.413): the MME Code plus the M-TMSI that
// together identify a UE within an MME pool. The eNB includes it in the Initial
// UE Message when the UE re-establishes with an S-TMSI (e.g. a Service Request).
//
//	S-TMSI ::= SEQUENCE {
//	    mMEC    MME-Code,            -- OCTET STRING (SIZE(1))
//	    m-TMSI  OCTET STRING (SIZE(4)),
//	    iE-Extensions ... OPTIONAL }
type STMSI struct {
	MMEC  uint8
	MTMSI uint32
}

func (s STMSI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)
	w.WriteBit(false)

	if err := per.EncodeOctetString(w, enc, 1, 1, true, true, false, []byte{s.MMEC}); err != nil {
		return err
	}

	var mtmsi [4]byte
	binary.BigEndian.PutUint32(mtmsi[:], s.MTMSI)

	return per.EncodeOctetString(w, enc, 4, 4, true, true, false, mtmsi[:])
}

func (s *STMSI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	extBit, err := r.ReadBit()
	if err != nil {
		return err
	}

	extContainer, err := r.ReadBit()
	if err != nil {
		return err
	}

	mmec, err := per.DecodeOctetString(r, enc, 1, 1, true, true, false)
	if err != nil {
		return err
	}

	mtmsi, err := per.DecodeOctetString(r, enc, 4, 4, true, true, false)
	if err != nil {
		return err
	}

	if err := skipSequenceExtensionsPER(r, enc, extContainer, extBit); err != nil {
		return err
	}

	*s = STMSI{MMEC: mmec[0], MTMSI: binary.BigEndian.Uint32(mtmsi)}

	return nil
}
