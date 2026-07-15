// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/binary"

	"github.com/ellanetworks/core/aper"
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

func (s STMSI) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := w.WriteOctetString([]byte{s.MMEC}, 1, 1, false); err != nil {
		return err
	}

	var mtmsi [4]byte
	binary.BigEndian.PutUint32(mtmsi[:], s.MTMSI)

	return w.WriteOctetString(mtmsi[:], 4, 4, false)
}

func decodeSTMSI(r *aper.Reader) (STMSI, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return STMSI{}, err
	}

	mmec, err := r.ReadOctetString(1, 1, false)
	if err != nil {
		return STMSI{}, err
	}

	mtmsi, err := r.ReadOctetString(4, 4, false)
	if err != nil {
		return STMSI{}, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return STMSI{}, err
	}

	return STMSI{MMEC: mmec[0], MTMSI: binary.BigEndian.Uint32(mtmsi)}, nil
}
