// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import (
	"errors"
	"fmt"
	"math"

	"github.com/ellanetworks/core/diameter"
)

var ErrInvalidBandwidth = errors.New("tgpp: invalid bandwidth")

const bitsPerKilobit = 1000

type Bandwidth uint64

func (b Bandwidth) Representable() bool {
	return b <= math.MaxUint32 || b%bitsPerKilobit == 0 && b/bitsPerKilobit <= math.MaxUint32
}

type BandwidthAVPs struct {
	BPS      uint32
	KBPS     uint32
	BPSFlags uint8
}

var (
	MaxRequestedBandwidthUL = BandwidthAVPs{AVPMaxRequestedBandwidthUL, AVPExtendedMaxRequestedBWUL, diameter.AVPFlagMandatory}
	MaxRequestedBandwidthDL = BandwidthAVPs{AVPMaxRequestedBandwidthDL, AVPExtendedMaxRequestedBWDL, diameter.AVPFlagMandatory}
)

func (c BandwidthAVPs) Append(avps []diameter.AVP, b Bandwidth) ([]diameter.AVP, error) {
	switch {
	case b <= math.MaxUint32:
		return append(avps, diameter.Unsigned32(c.BPS, c.BPSFlags, VendorID, uint32(b))), nil
	case !b.Representable():
		return nil, fmt.Errorf("%w: %d bit/s above 2^32-1 is not a whole number of kbit/s below 2^32", ErrInvalidBandwidth, uint64(b))
	}

	return append(avps,
		diameter.Unsigned32(c.BPS, c.BPSFlags, VendorID, math.MaxUint32),
		diameter.Unsigned32(c.KBPS, 0, VendorID, uint32(b/bitsPerKilobit)),
	), nil
}

func (c BandwidthAVPs) Parse(avps []diameter.AVP) (*Bandwidth, error) {
	var bps, kbps *uint32

	for _, f := range []struct {
		code uint32
		dst  **uint32
	}{{c.BPS, &bps}, {c.KBPS, &kbps}} {
		a, ok := diameter.Find(avps, f.code, VendorID)
		if !ok {
			continue
		}

		v, err := Unsigned32(a)
		if err != nil {
			return nil, err
		}

		*f.dst = &v
	}

	switch {
	case kbps != nil:
		b := Bandwidth(*kbps) * bitsPerKilobit
		return &b, nil
	case bps != nil:
		b := Bandwidth(*bps)
		return &b, nil
	}

	return nil, nil
}
