// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

const defaultQoSRuleIdentifier uint8 = 1

func MappedFiveGSQoSContainers(ebi uint8, qos *EpsQoS) ([]nas.PCOContainer, error) {
	rules, err := fgs.QoSRules{fgs.DefaultQoSRule(defaultQoSRuleIdentifier, models.DefaultQFI)}.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("encode the mapped QoS rules: %w", err)
	}

	flow := fgs.FiveQIQoSFlow(models.DefaultQFI, qos.QCI, fgs.QoSFlowOpCreate)

	param, err := fgs.EPSBearerIDQoSFlowParameter(ebi)
	if err != nil {
		return nil, fmt.Errorf("encode the mapped QoS flow EPS bearer identity: %w", err)
	}

	flow.Parameters = append(flow.Parameters, param)

	flows, err := fgs.QoSFlowDescriptions{flow}.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("encode the mapped QoS flow descriptions: %w", err)
	}

	ambr, err := fgs.SessionAMBRFromKbps(qos.SessAmbrDL.Kbps(), qos.SessAmbrUL.Kbps())
	if err != nil {
		return nil, fmt.Errorf("encode the mapped Session-AMBR: %w", err)
	}

	ambrValue, err := ambr.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("encode the mapped Session-AMBR: %w", err)
	}

	rulesContainer, err := nas.NewQoSRulesContainer(rules, false)
	if err != nil {
		return nil, err
	}

	flowsContainer, err := nas.NewQoSFlowDescriptionsContainer(flows, false)
	if err != nil {
		return nil, err
	}

	ambrContainer, err := nas.NewSessionAMBRContainer(ambrValue)
	if err != nil {
		return nil, err
	}

	return []nas.PCOContainer{rulesContainer, ambrContainer, flowsContainer}, nil
}
