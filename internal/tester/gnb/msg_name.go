// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

// messageName renders a received PDU for a wait timeout. Only the procedures
// this simulator drives are named; anything else prints its code.
// internal/tester/s1enb names S1AP the same way.
func messageName(cat Category, code ngap.ProcedureCode) string {
	name := map[ngap.ProcedureCode]string{
		ngap.ProcNGSetup:                  "NGSetup",
		ngap.ProcNGReset:                  "NGReset",
		ngap.ProcRANConfigurationUpdate:   "RANConfigurationUpdate",
		ngap.ProcInitialContextSetup:      "InitialContextSetup",
		ngap.ProcDownlinkNASTransport:     "DownlinkNASTransport",
		ngap.ProcUplinkNASTransport:       "UplinkNASTransport",
		ngap.ProcInitialUEMessage:         "InitialUEMessage",
		ngap.ProcUEContextRelease:         "UEContextRelease",
		ngap.ProcUEContextReleaseRequest:  "UEContextReleaseRequest",
		ngap.ProcPathSwitchRequest:        "PathSwitchRequest",
		ngap.ProcPaging:                   "Paging",
		ngap.ProcErrorIndication:          "ErrorIndication",
		ngap.ProcPDUSessionResourceSetup:  "PDUSessionResourceSetup",
		ngap.ProcPDUSessionResourceModify: "PDUSessionResourceModify",

		ngap.ProcPDUSessionResourceRelease:          "PDUSessionResourceRelease",
		ngap.ProcHandoverPreparation:                "HandoverPreparation",
		ngap.ProcHandoverResourceAllocation:         "HandoverResourceAllocation",
		ngap.ProcHandoverNotification:               "HandoverNotification",
		ngap.ProcHandoverCancel:                     "HandoverCancel",
		ngap.ProcDownlinkUEAssociatedNRPPaTransport: "DownlinkUEAssociatedNRPPaTransport",
		ngap.ProcUplinkUEAssociatedNRPPaTransport:   "UplinkUEAssociatedNRPPaTransport",
	}[code]
	if name == "" {
		name = fmt.Sprintf("procedure-%d", code)
	}

	cats := [...]string{"InitiatingMessage", "SuccessfulOutcome", "UnsuccessfulOutcome"}

	c := "Unknown"
	if int(cat) < len(cats) {
		c = cats[cat]
	}

	return c + "/" + name
}
