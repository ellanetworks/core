// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/ngap"
)

func (g *GnodeB) WaitForAMFConfigurationUpdate(timeout time.Duration) (*ngap.AMFConfigurationUpdate, error) {
	frame, err := g.WaitForMessage(Initiating, ngap.ProcAMFConfigurationUpdate, timeout)
	if err != nil {
		return nil, err
	}

	update, err := ngap.ParseAMFConfigurationUpdate(frame.Value)
	if err != nil {
		return nil, fmt.Errorf("gnb: parse AMF Configuration Update: %w", err)
	}

	return update, nil
}

func (g *GnodeB) SendAMFConfigurationUpdateAcknowledge() error {
	b, err := (&ngap.AMFConfigurationUpdateAcknowledge{}).Marshal()
	if err != nil {
		return fmt.Errorf("gnb: build AMF Configuration Update Acknowledge: %w", err)
	}

	return g.SendMessage(b, NGAPProcedureAMFConfigUpdateAck)
}

func (g *GnodeB) WaitForAMFStatusIndication(timeout time.Duration) (*ngap.AMFStatusIndication, error) {
	frame, err := g.WaitForMessage(Initiating, ngap.ProcAMFStatusIndication, timeout)
	if err != nil {
		return nil, err
	}

	ind, err := ngap.ParseAMFStatusIndication(frame.Value)
	if err != nil {
		return nil, fmt.Errorf("gnb: parse AMF Status Indication: %w", err)
	}

	return ind, nil
}
