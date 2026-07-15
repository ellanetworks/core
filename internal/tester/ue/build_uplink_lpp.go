// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"bytes"
	"fmt"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

// BuildUplinkNasTransportLPP builds a UL NAS Transport message with LPP payload
// container type. Used by the UE tester to send LPP responses (capabilities,
// location information) back to the core.
func BuildUplinkNasTransportLPP(lppPayload []byte) ([]byte, error) {
	if lppPayload == nil {
		return nil, fmt.Errorf("LPP payload is required")
	}

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeULNASTransport)

	ulNasTransport := nasMessage.NewULNASTransport(0)
	ulNasTransport.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	ulNasTransport.SetMessageType(nas.MsgTypeULNASTransport)
	ulNasTransport.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	ulNasTransport.SetPayloadContainerType(nasMessage.PayloadContainerTypeLPP)
	ulNasTransport.PayloadContainer.SetLen(uint16(len(lppPayload)))
	ulNasTransport.SetPayloadContainerContents(lppPayload)

	m.ULNASTransport = ulNasTransport

	data := new(bytes.Buffer)
	if err := m.GmmMessageEncode(data); err != nil {
		return nil, fmt.Errorf("failed to encode GMM message: %w", err)
	}

	return data.Bytes(), nil
}
