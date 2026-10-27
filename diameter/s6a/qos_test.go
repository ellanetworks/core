// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func TestAMBREncoding(t *testing.T) {
	for _, tt := range []struct {
		bps      tgpp.Bandwidth
		extended bool
	}{
		{1, false},
		{1<<32 - 1, false},
		{4294968000, true},
		{6_000_000_000, true},
		{(1<<32 - 1) * 1000, true},
	} {
		a := must[diameter.AVP](t)(AMBR{UL: tt.bps, DL: 1}.avp())

		inner := must[[]diameter.AVP](t)(a.Grouped())
		if _, ok := diameter.Find(inner, tgpp.AVPExtendedMaxRequestedBWUL, tgpp.VendorID); ok != tt.extended {
			t.Errorf("%d: extended = %v", tt.bps, ok)
		}

		if bps, _ := diameter.Find(inner, tgpp.AVPMaxRequestedBandwidthUL, tgpp.VendorID); tt.extended && must[uint32](t)(bps.Unsigned32()) != 1<<32-1 {
			t.Errorf("%d: Max-Requested-Bandwidth-UL = %x", tt.bps, bps.Data)
		}

		back := must[*AMBR](t)(answerDecoder.ambr(a))
		if back.UL != tt.bps || back.DL != 1 {
			t.Errorf("%d decodes to %+v", tt.bps, back)
		}
	}
}

func TestAMBRGapValues(t *testing.T) {
	for _, bps := range []tgpp.Bandwidth{0, 1 << 32, 4294967999, 6_000_000_999, 1 << 63} {
		if _, err := (AMBR{UL: 1, DL: bps}).avp(); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%d: err = %v", bps, err)
		}
	}

	ambr := vendorGroup(AVPAMBR,
		vendorU32(tgpp.AVPMaxRequestedBandwidthUL, 1<<32-1), vendorU32(tgpp.AVPMaxRequestedBandwidthDL, 7),
		optionalU32(tgpp.AVPExtendedMaxRequestedBWUL, 5))

	got := must[*AMBR](t)(answerDecoder.ambr(ambr))
	if got.UL != 5000 || got.DL != 7 {
		t.Fatalf("small extended value = %+v", got)
	}

	rebuilt := must[diameter.AVP](t)(got.avp())
	if again := must[*AMBR](t)(answerDecoder.ambr(rebuilt)); *again != *got {
		t.Fatalf("rebuilt = %+v", again)
	}
}

func TestEPSSubscribedQoSProfile(t *testing.T) {
	for _, qci := range []uint32{5, 6, 7, 8, 9, 10, 69, 70, 79, 80, 128, 254} {
		q := EPSSubscribedQoSProfile{QCI: qci, ARP: tgpp.DefaultAllocationRetentionPriority(1)}

		back := must[*EPSSubscribedQoSProfile](t)(answerDecoder.qosProfile(must[diameter.AVP](t)(q.avp())))
		if *back != q {
			t.Errorf("QCI %d: %+v", qci, back)
		}
	}

	for _, qci := range []uint32{0, 1, 2, 3, 4, 65, 66, 67, 71, 75, 76, 82, 85, 127, 255} {
		q := EPSSubscribedQoSProfile{QCI: qci, ARP: tgpp.DefaultAllocationRetentionPriority(1)}
		if _, err := q.avp(); !errors.Is(err, ErrInvalidMessage) || NonGBRQCI(qci) {
			t.Errorf("GBR or undefined QCI %d accepted: %v", qci, err)
		}
	}

	defaults := vendorGroup(AVPEPSSubscribedQoSProfile, vendorU32(tgpp.AVPQoSClassIdentifier, 9),
		diameter.Grouped(tgpp.AVPAllocationRetentionPriority, 0, tgpp.VendorID, optionalU32(tgpp.AVPPriorityLevel, 3)))

	got := must[*EPSSubscribedQoSProfile](t)(answerDecoder.qosProfile(defaults))
	if got.ARP != tgpp.DefaultAllocationRetentionPriority(3) {
		t.Fatalf("ARP defaults = %+v", got.ARP)
	}
}
