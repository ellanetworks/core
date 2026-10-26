// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pcf

import (
	"strings"

	"github.com/ellanetworks/core/diameter/rx"
	"github.com/ellanetworks/core/internal/models"
)

const (
	mcpttICSI = "urn:urn-7:3gpp-service.ims.icsi.mcptt"

	qciMCPTT             = 65
	mcpttBaseARPPriority = 8

	mcpttPreemptionNamespace = "mcpttp"
	mcpttQueueingNamespace   = "mcpttq"
	mcpttProtectedARPLevel   = 2
)

type serviceContext struct {
	applicationID string
	mcptt         mcpttPriority
}

type mcpttPriority struct {
	namespace string
	level     rx.ReservationPriority
}

func (c serviceContext) updated(r rx.AARequest) serviceContext {
	if r.AFApplicationIdentifier != "" {
		c.applicationID = r.AFApplicationIdentifier
	}

	if r.MCPTTIdentifier != "" {
		c.mcptt = mcpttPriority{namespace: mcpttNamespace(r.MCPTTIdentifier)}
		if r.ReservationPriority != nil {
			c.mcptt.level = *r.ReservationPriority
		}
	}

	return c
}

func mcpttNamespace(identifier string) string {
	id := strings.ToLower(strings.TrimSpace(identifier))

	for _, ns := range []string{mcpttPreemptionNamespace, mcpttQueueingNamespace} {
		if strings.HasPrefix(id, ns) {
			return ns
		}
	}

	return ""
}

func (c serviceContext) service(component rx.MediaComponent) string {
	if component.AFApplicationIdentifier != "" {
		return component.AFApplicationIdentifier
	}

	return c.applicationID
}

func mcpttQoS(c rx.MediaComponent, p mcpttPriority) (uint8, models.Arp, bool) {
	if c.Type == nil {
		return 0, models.Arp{}, false
	}

	switch *c.Type {
	case rx.MediaAudio, rx.MediaApplication:
		return qciMCPTT, p.arp(), true
	default:
		return 0, models.Arp{}, false
	}
}

func (p mcpttPriority) arp() models.Arp {
	if p.namespace == "" {
		return models.Arp{PriorityLevel: mcpttBaseARPPriority, PreemptCap: models.PreemptionCapabilityNotPreempt, PreemptVuln: models.PreemptionVulnerabilityPreemptable}
	}

	arp := models.Arp{
		PriorityLevel: mcpttBaseARPPriority - int32(p.level)/2,
		PreemptCap:    models.PreemptionCapabilityNotPreempt,
		PreemptVuln:   models.PreemptionVulnerabilityPreemptable,
	}

	if p.namespace == mcpttPreemptionNamespace {
		arp.PreemptCap = models.PreemptionCapabilityMayPreempt
	}

	if arp.PriorityLevel <= mcpttProtectedARPLevel {
		arp.PreemptVuln = models.PreemptionVulnerabilityNotPreemptable
	}

	return arp
}
