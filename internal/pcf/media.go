// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pcf

import (
	"fmt"
	"slices"

	"github.com/ellanetworks/core/diameter/rx"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

const (
	qciConversationalVoice  = 1
	qciConversationalVideo  = 2
	voiceARPPriority        = 2
	videoARPPriority        = 4
	rtcpBandwidthPercentage = 5
	firstFilterPrecedence   = 32
	defaultAudioBitRate     = 49000
	defaultVideoBitRate     = 512000
)

var errFilterRestrictions = fmt.Errorf("flow description outside the Rx restrictions")

type flowKey struct {
	component uint32
	flow      uint32
}

func mediaRules(sessionID string, components []rx.MediaComponent) (map[flowKey]smf.PCCRule, error) {
	rules := make(map[flowKey]smf.PCCRule)

	precedence := uint8(firstFilterPrecedence)

	for _, c := range components {
		qci, arp, ok := mediaQoS(c)
		if !ok {
			continue
		}

		for _, sub := range c.SubComponents {
			status := sub.FlowStatus
			if status == nil {
				status = c.FlowStatus
			}

			if removed(status) || signalling(sub) {
				continue
			}

			rule := smf.PCCRule{
				ID:   fmt.Sprintf("%s#%d.%d", sessionID, c.Number, sub.FlowNumber),
				QCI:  qci,
				ARP:  arp,
				Gate: gate(status, sub),
			}

			hasUL, hasDL := false, false

			for _, raw := range sub.FlowDescriptions {
				fd, err := rx.ParseFlowDescription(raw)
				if err != nil {
					return nil, fmt.Errorf("%w: %v", errFilterRestrictions, err)
				}

				rule.Filters = append(rule.Filters, sdfFilter(fd, precedence))
				precedence++

				if fd.Direction == rx.FlowDirectionIn {
					hasUL = true
				} else {
					hasDL = true
				}
			}

			if len(rule.Filters) == 0 {
				continue
			}

			ul, dl := subComponentRates(c, sub, hasUL, hasDL)
			rule.MBR = models.Ambr{Uplink: models.BitRateFromBps(ul), Downlink: models.BitRateFromBps(dl)}
			rule.GBR = guaranteedRates(c, sub, rule.MBR)
			rules[flowKey{component: c.Number, flow: sub.FlowNumber}] = rule
		}
	}

	return rules, nil
}

func gate(status *rx.FlowStatus, sub rx.MediaSubComponent) models.GateStatus {
	if status == nil || sub.FlowUsage != nil && *sub.FlowUsage == rx.FlowUsageRTCP {
		return models.GateStatus{}
	}

	switch *status {
	case rx.FlowStatusEnabledUplink:
		return models.GateStatus{DLGate: models.GateClose}
	case rx.FlowStatusEnabledDownlink:
		return models.GateStatus{ULGate: models.GateClose}
	case rx.FlowStatusDisabled:
		return models.GateStatus{ULGate: models.GateClose, DLGate: models.GateClose}
	default:
		return models.GateStatus{}
	}
}

func forkedRule(prev, next smf.PCCRule) smf.PCCRule {
	out := next
	out.Filters = slices.Clone(prev.Filters)

	for _, f := range next.Filters {
		if !slices.ContainsFunc(out.Filters, func(o models.SDFFilter) bool { return sameFlow(o, f) }) {
			out.Filters = append(out.Filters, f)
		}
	}

	out.MBR = maxRate(prev.MBR, next.MBR)
	out.GBR = maxRate(prev.GBR, next.GBR)
	out.Gate = models.GateStatus{ULGate: min(prev.Gate.ULGate, next.Gate.ULGate), DLGate: min(prev.Gate.DLGate, next.Gate.DLGate)}

	return out
}

func sameFlow(a, b models.SDFFilter) bool {
	a.Precedence, b.Precedence = 0, 0
	return a == b
}

func maxRate(a, b models.Ambr) models.Ambr {
	return models.Ambr{
		Uplink:   models.BitRateFromBps(max(a.Uplink.Bps(), b.Uplink.Bps())),
		Downlink: models.BitRateFromBps(max(a.Downlink.Bps(), b.Downlink.Bps())),
	}
}

func mediaQoS(c rx.MediaComponent) (uint8, models.Arp, bool) {
	if c.Type == nil {
		return 0, models.Arp{}, false
	}

	switch *c.Type {
	case rx.MediaAudio:
		return qciConversationalVoice, models.Arp{PriorityLevel: voiceARPPriority, PreemptCap: models.PreemptionCapabilityMayPreempt, PreemptVuln: models.PreemptionVulnerabilityNotPreemptable}, true
	case rx.MediaVideo:
		return qciConversationalVideo, models.Arp{PriorityLevel: videoARPPriority, PreemptCap: models.PreemptionCapabilityNotPreempt, PreemptVuln: models.PreemptionVulnerabilityPreemptable}, true
	default:
		return 0, models.Arp{}, false
	}
}

func guaranteedRates(c rx.MediaComponent, sub rx.MediaSubComponent, mbr models.Ambr) models.Ambr {
	if sub.FlowUsage != nil && *sub.FlowUsage == rx.FlowUsageRTCP {
		return mbr
	}

	return models.Ambr{
		Uplink:   guaranteed(c.MinRequestedBandwidthUL, mbr.Uplink),
		Downlink: guaranteed(c.MinRequestedBandwidthDL, mbr.Downlink),
	}
}

func guaranteed(minRequested *rx.Bandwidth, mbr models.BitRate) models.BitRate {
	if minRequested == nil {
		return mbr
	}

	return models.BitRateFromBps(min(uint64(*minRequested), mbr.Bps()))
}

func defaultBandwidth(c rx.MediaComponent) *rx.Bandwidth {
	b := rx.Bandwidth(defaultAudioBitRate)
	if c.Type != nil && *c.Type == rx.MediaVideo {
		b = defaultVideoBitRate
	}

	return &b
}

func subComponentRates(c rx.MediaComponent, sub rx.MediaSubComponent, hasUL, hasDL bool) (ulBps, dlBps uint64) {
	maxUL, maxDL := c.MaxRequestedBandwidthUL, c.MaxRequestedBandwidthDL
	if sub.MaxRequestedBandwidthUL != nil {
		maxUL = sub.MaxRequestedBandwidthUL
	}

	if sub.MaxRequestedBandwidthDL != nil {
		maxDL = sub.MaxRequestedBandwidthDL
	}

	if maxUL == nil {
		maxUL = defaultBandwidth(c)
	}

	if maxDL == nil {
		maxDL = defaultBandwidth(c)
	}

	if sub.FlowUsage != nil && *sub.FlowUsage == rx.FlowUsageRTCP {
		return rtcpRate(c, maxUL, hasUL), rtcpRate(c, maxDL, hasDL)
	}

	return requested(maxUL, hasUL), requested(maxDL, hasDL)
}

func rtcpRate(c rx.MediaComponent, maxRequested *rx.Bandwidth, present bool) uint64 {
	if !present {
		return 0
	}

	rs, rr := c.RSBandwidth, c.RRBandwidth

	if rs != nil && rr != nil {
		return uint64(*rs) + uint64(*rr)
	}

	share := uint64(*maxRequested) * rtcpBandwidthPercentage / 100

	if rs != nil {
		return max(share, uint64(*rs))
	}

	if rr != nil {
		return max(share, uint64(*rr))
	}

	return share
}

func requested(b *rx.Bandwidth, present bool) uint64 {
	if !present || b == nil {
		return 0
	}

	return uint64(*b)
}

func removed(s *rx.FlowStatus) bool {
	return s != nil && *s == rx.FlowStatusRemoved
}

func signalling(sub rx.MediaSubComponent) bool {
	return sub.FlowUsage != nil && *sub.FlowUsage == rx.FlowUsageAFSignalling
}

func sdfFilter(fd rx.FlowDescription, precedence uint8) models.SDFFilter {
	f := models.SDFFilter{Precedence: precedence}

	if fd.Protocol != rx.ProtocolIP {
		f.Protocol = uint8(fd.Protocol)
	}

	if fd.Direction == rx.FlowDirectionIn {
		f.Direction = models.FilterUplink
		f.Remote, f.LocalPort, f.RemotePort = fd.Destination, fd.SourcePort, fd.DestinationPort
	} else {
		f.Direction = models.FilterDownlink
		f.Remote, f.LocalPort, f.RemotePort = fd.Source, fd.DestinationPort, fd.SourcePort
	}

	return f
}

func overlayComponent(prev, next rx.MediaComponent) rx.MediaComponent {
	out := prev
	out.Number = next.Number

	for _, f := range []struct{ dst, src **rx.Bandwidth }{
		{&out.MaxRequestedBandwidthUL, &next.MaxRequestedBandwidthUL},
		{&out.MaxRequestedBandwidthDL, &next.MaxRequestedBandwidthDL},
		{&out.MaxSupportedBandwidthUL, &next.MaxSupportedBandwidthUL},
		{&out.MaxSupportedBandwidthDL, &next.MaxSupportedBandwidthDL},
		{&out.MinDesiredBandwidthUL, &next.MinDesiredBandwidthUL},
		{&out.MinDesiredBandwidthDL, &next.MinDesiredBandwidthDL},
		{&out.MinRequestedBandwidthUL, &next.MinRequestedBandwidthUL},
		{&out.MinRequestedBandwidthDL, &next.MinRequestedBandwidthDL},
	} {
		if *f.src != nil {
			*f.dst = *f.src
		}
	}

	if next.Type != nil {
		out.Type = next.Type
	}

	if next.FlowStatus != nil {
		out.FlowStatus = next.FlowStatus
	}

	if next.RSBandwidth != nil {
		out.RSBandwidth = next.RSBandwidth
	}

	if next.RRBandwidth != nil {
		out.RRBandwidth = next.RRBandwidth
	}

	if next.AFApplicationIdentifier != "" {
		out.AFApplicationIdentifier = next.AFApplicationIdentifier
	}

	if len(next.CodecData) > 0 {
		out.CodecData = next.CodecData
	}

	if next.ContentVersion != nil {
		out.ContentVersion = next.ContentVersion
	}

	out.SubComponents = slices.Clone(prev.SubComponents)

	if next.FlowStatus != nil {
		for i := range out.SubComponents {
			out.SubComponents[i].FlowStatus = nil
		}
	}

	for _, sub := range next.SubComponents {
		i := slices.IndexFunc(out.SubComponents, func(s rx.MediaSubComponent) bool { return s.FlowNumber == sub.FlowNumber })
		if i < 0 {
			out.SubComponents = append(out.SubComponents, sub)
			continue
		}

		out.SubComponents[i] = overlaySubComponent(out.SubComponents[i], sub)
	}

	return out
}

func overlaySubComponent(prev, next rx.MediaSubComponent) rx.MediaSubComponent {
	out := prev

	if len(next.FlowDescriptions) > 0 {
		out.FlowDescriptions = next.FlowDescriptions
	}

	if next.FlowStatus != nil {
		out.FlowStatus = next.FlowStatus
	}

	if next.FlowUsage != nil {
		out.FlowUsage = next.FlowUsage
	}

	if next.MaxRequestedBandwidthUL != nil {
		out.MaxRequestedBandwidthUL = next.MaxRequestedBandwidthUL
	}

	if next.MaxRequestedBandwidthDL != nil {
		out.MaxRequestedBandwidthDL = next.MaxRequestedBandwidthDL
	}

	if next.SignallingProtocol != 0 {
		out.SignallingProtocol = next.SignallingProtocol
	}

	return out
}
