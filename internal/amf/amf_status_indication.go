// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"fmt"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

// BuildAMFStatusIndication encodes an AMF STATUS INDICATION naming this AMF's
// GUAMI as unavailable, which an NG-RAN node acts on by reselecting another AMF
// (TS 38.413 §8.7.6.2). Sent when the AMF is draining or shutting down.
//
// The Unavailable GUAMI List is mandatory and SIZE(1..maxnoofServedGUAMIs), so a
// GUAMI that cannot be encoded is an error rather than an empty list: a message
// naming nothing would tell the gNB to reselect away from no one.
func BuildAMFStatusIndication(guami *models.Guami) ([]byte, error) {
	if guami == nil {
		return nil, fmt.Errorf("amf: AMF Status Indication needs a GUAMI")
	}

	g, err := util.GUAMIToNGAP(*guami)
	if err != nil {
		return nil, fmt.Errorf("amf: encode GUAMI: %w", err)
	}

	msg := &ngap.AMFStatusIndication{
		UnavailableGUAMIList: ngap.UnavailableGUAMIList{{GUAMI: g}},
	}

	return msg.Marshal()
}
