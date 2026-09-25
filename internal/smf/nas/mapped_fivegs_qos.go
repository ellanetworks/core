// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func MappedFiveGSQoS(ebi uint8, qosData *models.QosData, ambr *models.Ambr) ([]nas.PCOContainer, error) {
	rules, err := fgs.QoSRules{fgs.DefaultQoSRule(DefaultQosRuleID, qosData.QFI)}.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("encode the mapped QoS rules: %w", err)
	}

	rulesContainer, err := nas.NewQoSRulesContainer(rules, false)
	if err != nil {
		return nil, err
	}

	flowAndAMBR, err := mappedFlowAndAMBR(ebi, qosData, ambr, fgs.QoSFlowOpCreate)
	if err != nil {
		return nil, err
	}

	return append([]nas.PCOContainer{rulesContainer}, flowAndAMBR...), nil
}

func MappedFiveGSQoSRefresh(ebi uint8, qosData *models.QosData, ambr *models.Ambr) ([]nas.PCOContainer, error) {
	return mappedFlowAndAMBR(ebi, qosData, ambr, fgs.QoSFlowOpModify)
}

func mappedFlowAndAMBR(ebi uint8, qosData *models.QosData, ambr *models.Ambr, op fgs.QoSFlowOperation) ([]nas.PCOContainer, error) {
	flow := fgs.FiveQIQoSFlow(qosData.QFI, uint8(qosData.Var5qi), op)

	param, err := fgs.EPSBearerIDQoSFlowParameter(ebi)
	if err != nil {
		return nil, fmt.Errorf("encode the mapped QoS flow EPS bearer identity: %w", err)
	}

	flow.Parameters = append(flow.Parameters, param)

	flows, err := fgs.QoSFlowDescriptions{flow}.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("encode the mapped QoS flow descriptions: %w", err)
	}

	sessionAMBR, err := fgs.SessionAMBRFromKbps(ambr.Downlink.Kbps(), ambr.Uplink.Kbps())
	if err != nil {
		return nil, fmt.Errorf("encode the mapped Session-AMBR: %w", err)
	}

	ambrValue, err := sessionAMBR.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("encode the mapped Session-AMBR: %w", err)
	}

	flowsContainer, err := nas.NewQoSFlowDescriptionsContainer(flows, false)
	if err != nil {
		return nil, err
	}

	ambrContainer, err := nas.NewSessionAMBRContainer(ambrValue)
	if err != nil {
		return nil, err
	}

	return []nas.PCOContainer{ambrContainer, flowsContainer}, nil
}
