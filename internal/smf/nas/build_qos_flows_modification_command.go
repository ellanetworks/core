// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func BuildQoSFlowsModificationCommand(pduSessionID uint8, pti uint8, rules fgs.QoSRules, flows fgs.QoSFlowDescriptions, mapped fgs.MappedEPSBearerContexts) ([]byte, error) {
	if len(rules) == 0 && len(flows) == 0 && len(mapped) == 0 {
		return nil, fmt.Errorf("no QoS rule, QoS flow description or mapped EPS bearer context to modify")
	}

	return (&fgs.PDUSessionModificationCommand{
		PDUSessionID:            fgs.PDUSessionID(pduSessionID),
		PTI:                     nas.ProcedureTransactionIdentity(pti),
		QoSRules:                rules,
		QoSFlowDescriptions:     flows,
		MappedEPSBearerContexts: mapped,
	}).MarshalBinary()
}
