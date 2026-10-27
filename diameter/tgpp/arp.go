// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/diameter"
)

var ErrInvalidARP = errors.New("tgpp: invalid Allocation-Retention-Priority")

const (
	MinPriorityLevel uint32 = 1
	MaxPriorityLevel uint32 = 15
)

type PreemptionCapability uint32

const (
	PreemptionCapabilityEnabled  PreemptionCapability = 0
	PreemptionCapabilityDisabled PreemptionCapability = 1
)

var preemptionCapabilityNames = EnumNames{"PRE-EMPTION_CAPABILITY_ENABLED", "PRE-EMPTION_CAPABILITY_DISABLED"}

func (c PreemptionCapability) String() string {
	return preemptionCapabilityNames.Name("PreemptionCapability", uint32(c))
}

func (c PreemptionCapability) Known() bool { return preemptionCapabilityNames.Has(uint32(c)) }

type PreemptionVulnerability uint32

const (
	PreemptionVulnerabilityEnabled  PreemptionVulnerability = 0
	PreemptionVulnerabilityDisabled PreemptionVulnerability = 1
)

var preemptionVulnerabilityNames = EnumNames{"PRE-EMPTION_VULNERABILITY_ENABLED", "PRE-EMPTION_VULNERABILITY_DISABLED"}

func (v PreemptionVulnerability) String() string {
	return preemptionVulnerabilityNames.Name("PreemptionVulnerability", uint32(v))
}

func (v PreemptionVulnerability) Known() bool { return preemptionVulnerabilityNames.Has(uint32(v)) }

type AllocationRetentionPriority struct {
	PriorityLevel           uint32
	PreemptionCapability    PreemptionCapability
	PreemptionVulnerability PreemptionVulnerability
}

func DefaultAllocationRetentionPriority(level uint32) AllocationRetentionPriority {
	return AllocationRetentionPriority{
		PriorityLevel:           level,
		PreemptionCapability:    PreemptionCapabilityDisabled,
		PreemptionVulnerability: PreemptionVulnerabilityEnabled,
	}
}

func (a AllocationRetentionPriority) Known() bool {
	return a.PriorityLevel >= MinPriorityLevel && a.PriorityLevel <= MaxPriorityLevel &&
		a.PreemptionCapability.Known() && a.PreemptionVulnerability.Known()
}

func (a AllocationRetentionPriority) AVP() (diameter.AVP, error) {
	if !a.Known() {
		return diameter.AVP{}, fmt.Errorf("%w: %+v", ErrInvalidARP, a)
	}

	return diameter.Grouped(AVPAllocationRetentionPriority, 0, VendorID,
		diameter.Unsigned32(AVPPriorityLevel, 0, VendorID, a.PriorityLevel),
		diameter.Unsigned32(AVPPreemptionCapability, 0, VendorID, uint32(a.PreemptionCapability)),
		diameter.Unsigned32(AVPPreemptionVulnerability, 0, VendorID, uint32(a.PreemptionVulnerability)),
	), nil
}

func ParseAllocationRetentionPriority(a diameter.AVP) (AllocationRetentionPriority, error) {
	inner, err := a.Grouped()
	if err != nil {
		return AllocationRetentionPriority{}, InvalidGroupedAVP(a, err)
	}

	level, ok := diameter.Find(inner, AVPPriorityLevel, VendorID)
	if !ok {
		return AllocationRetentionPriority{}, MissingAVP(AVPPriorityLevel, VendorID, 4)
	}

	arp := DefaultAllocationRetentionPriority(0)

	if arp.PriorityLevel, err = Unsigned32(level); err != nil {
		return AllocationRetentionPriority{}, err
	}

	for _, f := range []struct {
		code uint32
		dst  *uint32
	}{
		{AVPPreemptionCapability, (*uint32)(&arp.PreemptionCapability)},
		{AVPPreemptionVulnerability, (*uint32)(&arp.PreemptionVulnerability)},
	} {
		if v, ok := diameter.Find(inner, f.code, VendorID); ok {
			if *f.dst, err = Unsigned32(v); err != nil {
				return AllocationRetentionPriority{}, err
			}
		}
	}

	return arp, nil
}
