// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/diameter"
)

func TestAllocationRetentionPriorityRoundTrip(t *testing.T) {
	arp := AllocationRetentionPriority{
		PriorityLevel: 8, PreemptionCapability: PreemptionCapabilityEnabled, PreemptionVulnerability: PreemptionVulnerabilityDisabled,
	}

	a, err := arp.AVP()
	if err != nil {
		t.Fatal(err)
	}

	if a.Flags&diameter.AVPFlagMandatory != 0 || a.VendorID != VendorID {
		t.Fatalf("header = %+v", a)
	}

	if got, err := ParseAllocationRetentionPriority(a); err != nil || got != arp {
		t.Fatalf("parsed = %+v, %v", got, err)
	}
}

func TestAllocationRetentionPriorityDefaults(t *testing.T) {
	a := diameter.Grouped(AVPAllocationRetentionPriority, 0, VendorID, diameter.Unsigned32(AVPPriorityLevel, 0, VendorID, 3))

	got, err := ParseAllocationRetentionPriority(a)
	if err != nil || got != DefaultAllocationRetentionPriority(3) {
		t.Fatalf("parsed = %+v, %v", got, err)
	}

	if got.PreemptionCapability != PreemptionCapabilityDisabled || got.PreemptionVulnerability != PreemptionVulnerabilityEnabled {
		t.Fatalf("defaults = %+v", got)
	}
}

func TestAllocationRetentionPriorityLenient(t *testing.T) {
	a := diameter.Grouped(AVPAllocationRetentionPriority, 0, VendorID,
		diameter.Unsigned32(AVPPriorityLevel, 0, VendorID, 16),
		diameter.Unsigned32(AVPPreemptionCapability, 0, VendorID, 7),
	)

	got, err := ParseAllocationRetentionPriority(a)
	if err != nil || got.PriorityLevel != 16 || got.PreemptionCapability != 7 || got.Known() {
		t.Fatalf("parsed = %+v, %v", got, err)
	}

	if _, err := got.AVP(); !errors.Is(err, ErrInvalidARP) {
		t.Fatalf("unknown ARP builds: %v", err)
	}

	if s := got.PreemptionCapability.String(); s != "PreemptionCapability(7)" {
		t.Fatalf("String = %q", s)
	}
}

func TestParseAllocationRetentionPriorityErrors(t *testing.T) {
	for name, tt := range map[string]struct {
		a    diameter.AVP
		code uint32
	}{
		"no level":    {diameter.Grouped(AVPAllocationRetentionPriority, 0, VendorID), diameter.ResultMissingAVP},
		"short level": {diameter.Grouped(AVPAllocationRetentionPriority, 0, VendorID, diameter.OctetString(AVPPriorityLevel, 0, VendorID, []byte{1})), diameter.ResultInvalidAVPLength},
		"short vulnerability": {diameter.Grouped(AVPAllocationRetentionPriority, 0, VendorID,
			diameter.Unsigned32(AVPPriorityLevel, 0, VendorID, 1),
			diameter.OctetString(AVPPreemptionVulnerability, 0, VendorID, []byte{1})), diameter.ResultInvalidAVPLength},
		"not grouped": {diameter.OctetString(AVPAllocationRetentionPriority, 0, VendorID, []byte{1}), diameter.ResultInvalidAVPLength},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseAllocationRetentionPriority(tt.a)

			var avpErr *diameter.AVPError
			if !errors.As(err, &avpErr) || avpErr.ResultCode != tt.code {
				t.Fatalf("err = %v", err)
			}
		})
	}
}
