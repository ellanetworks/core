// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// UE NGAP ID ranges (TS 38.413 §9.3.3). The AMF id is 40 bits wide, past what
// S1AP's 32-bit MME-UE-S1AP-ID needs, so both are held in a uint64.
const (
	amfUENGAPIDMax = 1099511627775 // INTEGER (0..2^40-1)
	ranUENGAPIDMax = 4294967295    // INTEGER (0..2^32-1)
)

// AMFUENGAPID ::= INTEGER (0..1099511627775).
type AMFUENGAPID uint64

func (id AMFUENGAPID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: amfUENGAPIDMax, HasUB: true}, int64(id))
}

func (id *AMFUENGAPID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: amfUENGAPIDMax, HasUB: true})
	if err != nil {
		return err
	}

	*id = AMFUENGAPID(v)

	return nil
}

// RANUENGAPID ::= INTEGER (0..4294967295).
type RANUENGAPID uint32

func (id RANUENGAPID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: ranUENGAPIDMax, HasUB: true}, int64(id))
}

func (id *RANUENGAPID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: ranUENGAPIDMax, HasUB: true})
	if err != nil {
		return err
	}

	*id = RANUENGAPID(v)

	return nil
}

// TAI ::= SEQUENCE { pLMNIdentity, tAC, iE-Extensions OPTIONAL } (extensible)
// — TS 38.413 §9.3.3.11.
type TAI struct {
	_            [0]struct{} `per:"extseq"`
	PLMNIdentity PLMNIdentity
	TAC          TAC
	_            ieExtensions `per:",skip"`
}
