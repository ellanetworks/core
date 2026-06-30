// SPDX-FileCopyrightText: Ella Networks Inc.
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

// TS 38.413
func BuildPathSwitchRequestAcknowledgeTransfer(teid uint32, n3IPv4 netip.Addr, n3IPv6 netip.Addr) ([]byte, error) {
	teidOct := make([]byte, 4)
	binary.BigEndian.PutUint32(teidOct, teid)

	pathSwitchRequestAcknowledgeTransfer := ngapType.PathSwitchRequestAcknowledgeTransfer{}

	pathSwitchRequestAcknowledgeTransfer.ULNGUUPTNLInformation = new(ngapType.UPTransportLayerInformation)
	pathSwitchRequestAcknowledgeTransfer.ULNGUUPTNLInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	pathSwitchRequestAcknowledgeTransfer.ULNGUUPTNLInformation.GTPTunnel = new(ngapType.GTPTunnel)
	pathSwitchRequestAcknowledgeTransfer.ULNGUUPTNLInformation.GTPTunnel.GTPTEID.Value = teidOct

	tla, err := encodeTransportLayerAddress(n3IPv4, n3IPv6)
	if err != nil {
		return nil, fmt.Errorf("encode transport layer address failed: %s", err)
	}

	pathSwitchRequestAcknowledgeTransfer.ULNGUUPTNLInformation.GTPTunnel.TransportLayerAddress.Value = tla

	pathSwitchRequestAcknowledgeTransfer.SecurityIndication = new(ngapType.SecurityIndication)
	pathSwitchRequestAcknowledgeTransfer.SecurityIndication.IntegrityProtectionIndication.Value = ngapType.IntegrityProtectionIndicationPresentNotNeeded
	pathSwitchRequestAcknowledgeTransfer.SecurityIndication.ConfidentialityProtectionIndication.Value = ngapType.ConfidentialityProtectionIndicationPresentNotNeeded

	buf, err := aper.MarshalWithParams(pathSwitchRequestAcknowledgeTransfer, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("could not encode path switch request acknowledge transfer: %s", err)
	}

	return buf, nil
}
