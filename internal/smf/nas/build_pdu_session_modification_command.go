// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

type MappedEPSQoS struct {
	QosData models.QosData
	Ambr    models.Ambr
}

// BuildPDUSessionModificationCommand constructs a NAS PDU SESSION MODIFICATION
// COMMAND message (TS 24.501 §8.3.9). At least one of ambr, qosData, or dns must
// be non-nil.
func BuildPDUSessionModificationCommand(pduSessionID uint8, ambr *models.Ambr, qosData *models.QosData, dns net.IP, epsBearerIdentity uint8, mappedEPSQoS *MappedEPSQoS) ([]byte, error) {
	if ambr == nil && qosData == nil && dns == nil {
		return nil, fmt.Errorf("at least one of ambr, qosData, or dns must be provided")
	}

	m := &fgs.PDUSessionModificationCommand{PDUSessionID: fgs.PDUSessionID(pduSessionID)}

	if ambr != nil {
		sessAMBR, err := ModelsToSessionAMBR(ambr)
		if err != nil {
			return nil, fmt.Errorf("convert AMBR: %v", err)
		}

		m.SessionAMBR = &sessAMBR
	}

	if qosData != nil {
		flow, err := modifiedQoSFlow(qosData, epsBearerIdentity)
		if err != nil {
			return nil, err
		}

		m.QoSFlowDescriptions = fgs.QoSFlowDescriptions{flow}
	}

	if mappedEPSQoS != nil && epsBearerIdentity != 0 {
		mapped, err := mappedEPSBearerContexts(epsBearerIdentity, fgs.MappedEPSBearerOpModify, &mappedEPSQoS.QosData, &mappedEPSQoS.Ambr)
		if err != nil {
			return nil, err
		}

		m.MappedEPSBearerContexts = mapped
	}

	if dns != nil {
		var dnsServer []byte
		if v4 := dns.To4(); v4 != nil {
			dnsServer = v4
		} else {
			dnsServer = dns.To16()
		}

		opts := nas.NewProtocolConfigurationOptions([][]byte{dnsServer}, 0)
		m.ExtendedPCO = &opts
	}

	return m.MarshalBinary()
}

func modifiedQoSFlow(qosData *models.QosData, epsBearerIdentity uint8) (fgs.QoSFlowDescription, error) {
	flow := fgs.FiveQIQoSFlow(qosData.QFI, uint8(qosData.Var5qi), fgs.QoSFlowOpModify)
	if epsBearerIdentity == 0 {
		return flow, nil
	}

	param, err := fgs.EPSBearerIDQoSFlowParameter(epsBearerIdentity)
	if err != nil {
		return flow, fmt.Errorf("EPS bearer identity parameter: %w", err)
	}

	flow.Parameters = append(flow.Parameters, param)

	return flow, nil
}
