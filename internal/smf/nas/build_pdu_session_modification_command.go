// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
)

// BuildPDUSessionModificationCommand constructs a NAS PDU SESSION MODIFICATION
// COMMAND message (TS 24.501 §8.3.9). At least one of ambr, qosData, or dns must
// be non-nil.
func BuildPDUSessionModificationCommand(pduSessionID uint8, ambr *models.Ambr, qosData *models.QosData, dns net.IP) ([]byte, error) {
	if ambr == nil && qosData == nil && dns == nil {
		return nil, fmt.Errorf("at least one of ambr, qosData, or dns must be provided")
	}

	m := &fgs.PDUSessionModificationCommand{PDUSessionID: pduSessionID}

	if ambr != nil {
		sessAMBR, err := ModelsToSessionAMBR(ambr)
		if err != nil {
			return nil, fmt.Errorf("convert AMBR: %v", err)
		}

		m.SessionAMBR = &sessAMBR
	}

	if qosData != nil {
		m.QoSFlowDescriptions = fgs.MarshalModifyQoSFlow(qosData.QFI, uint8(qosData.Var5qi))
	}

	if dns != nil {
		var dnsServer []byte
		if v4 := dns.To4(); v4 != nil {
			dnsServer = v4
		} else {
			dnsServer = dns.To16()
		}

		m.ExtendedPCO = fgs.BuildProtocolConfigurationOptions([][]byte{dnsServer}, 0)
	}

	return m.Marshal()
}
