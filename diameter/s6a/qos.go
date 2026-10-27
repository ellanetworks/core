// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	minOperatorQCI uint32 = 128
	maxOperatorQCI uint32 = 254
)

type AMBR struct {
	UL tgpp.Bandwidth
	DL tgpp.Bandwidth
}

func (a AMBR) Known() bool {
	return a.UL >= 1 && a.DL >= 1 && a.UL.Representable() && a.DL.Representable()
}

var ambrRules = diameter.Rules{
	vendorKey(tgpp.AVPMaxRequestedBandwidthUL):  {},
	vendorKey(tgpp.AVPMaxRequestedBandwidthDL):  {},
	vendorKey(tgpp.AVPExtendedMaxRequestedBWUL): {},
	vendorKey(tgpp.AVPExtendedMaxRequestedBWDL): {},
}

func (a AMBR) avp() (diameter.AVP, error) {
	if !a.Known() {
		return diameter.AVP{}, invalidf("AMBR UL %d DL %d bit/s", uint64(a.UL), uint64(a.DL))
	}

	ul, err := tgpp.MaxRequestedBandwidthUL.Append(nil, a.UL)
	if err != nil {
		return diameter.AVP{}, invalidf("%w", err)
	}

	dl, err := tgpp.MaxRequestedBandwidthDL.Append(nil, a.DL)
	if err != nil {
		return diameter.AVP{}, invalidf("%w", err)
	}

	return groupedAVP(AVPAMBR, append([]diameter.AVP{ul[0], dl[0]}, append(ul[1:], dl[1:]...)...)...), nil
}

func parseBandwidth(avps []diameter.AVP, codes tgpp.BandwidthAVPs) (tgpp.Bandwidth, error) {
	if _, err := requiredUnsigned(avps, codes.BPS); err != nil {
		return 0, err
	}

	b, err := codes.Parse(avps)
	if err != nil {
		return 0, err
	}

	return *b, nil
}

func (d decoder) ambr(a diameter.AVP) (*AMBR, error) {
	inner, err := d.group(a, ambrRules)
	if err != nil {
		return nil, err
	}

	var ambr AMBR

	if ambr.UL, err = parseBandwidth(inner, tgpp.MaxRequestedBandwidthUL); err != nil {
		return nil, diameter.WithinGrouped(a, err)
	}

	if ambr.DL, err = parseBandwidth(inner, tgpp.MaxRequestedBandwidthDL); err != nil {
		return nil, diameter.WithinGrouped(a, err)
	}

	return &ambr, nil
}

type EPSSubscribedQoSProfile struct {
	QCI uint32
	ARP tgpp.AllocationRetentionPriority
}

func NonGBRQCI(qci uint32) bool {
	switch qci {
	case 5, 6, 7, 8, 9, 10, 69, 70, 79, 80:
		return true
	default:
		return qci >= minOperatorQCI && qci <= maxOperatorQCI
	}
}

func (q EPSSubscribedQoSProfile) Known() bool {
	return NonGBRQCI(q.QCI) && q.ARP.Known()
}

var (
	qosProfileRules = diameter.Rules{
		vendorKey(tgpp.AVPQoSClassIdentifier):          {},
		vendorKey(tgpp.AVPAllocationRetentionPriority): {},
	}
	arpRules = diameter.Rules{
		vendorKey(tgpp.AVPPriorityLevel):           {},
		vendorKey(tgpp.AVPPreemptionCapability):    {},
		vendorKey(tgpp.AVPPreemptionVulnerability): {},
	}
)

func (q EPSSubscribedQoSProfile) avp() (diameter.AVP, error) {
	if !NonGBRQCI(q.QCI) {
		return diameter.AVP{}, invalidf("QCI %d is not a non-GBR QCI", q.QCI)
	}

	arp, err := q.ARP.AVP()
	if err != nil {
		return diameter.AVP{}, invalidf("%w", err)
	}

	return groupedAVP(AVPEPSSubscribedQoSProfile, unsignedAVP(tgpp.AVPQoSClassIdentifier, q.QCI), arp), nil
}

func (d decoder) qosProfile(a diameter.AVP) (*EPSSubscribedQoSProfile, error) {
	inner, err := d.group(a, qosProfileRules)
	if err != nil {
		return nil, err
	}

	var q EPSSubscribedQoSProfile

	if q.QCI, err = requiredUnsigned(inner, tgpp.AVPQoSClassIdentifier); err != nil {
		return nil, diameter.WithinGrouped(a, err)
	}

	arpAVP, ok := diameter.Find(inner, tgpp.AVPAllocationRetentionPriority, tgpp.VendorID)
	if !ok {
		return nil, diameter.WithinGrouped(a, tgpp.MissingAVP(tgpp.AVPAllocationRetentionPriority, tgpp.VendorID, 0))
	}

	if _, err := d.group(arpAVP, arpRules); err != nil {
		return nil, diameter.WithinGrouped(a, err)
	}

	if q.ARP, err = tgpp.ParseAllocationRetentionPriority(arpAVP); err != nil {
		return nil, diameter.WithinGrouped(a, diameter.WithinGrouped(arpAVP, err))
	}

	return &q, nil
}
