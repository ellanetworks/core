// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
)

type ProtocolConfigurationOptions struct {
	DNSIPv4Request     bool
	DNSIPv6Request     bool
	IPv4LinkMTURequest bool
}

func HandlePDUSessionEstablishmentRequest(allowedSessionType models.PduSessionType, req *nasMessage.PDUSessionEstablishmentRequest) (*ProtocolConfigurationOptions, uint8, uint8, error) {
	// Handle PDUSessionType
	var estAcceptCause5gSMValue uint8
	selectedPDUSessionType := nasMessage.PDUSessionTypeIPv4
	if req.PDUSessionType != nil {
		selectedPDUSessionType = req.PDUSessionType.GetPDUSessionTypeValue()
		var err error
		estAcceptCause5gSMValue, err = isAllowedPDUSessionType(allowedSessionType, selectedPDUSessionType)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("requested PDUSessionType is not allowed: %v", err)
		}
	}

	pco := &ProtocolConfigurationOptions{}

	if req.ExtendedProtocolConfigurationOptions != nil {
		EPCOContents := req.ExtendedProtocolConfigurationOptions.GetExtendedProtocolConfigurationOptionsContents()
		protocolConfigurationOptions := nasConvert.NewProtocolConfigurationOptions()
		unmarshalErr := protocolConfigurationOptions.UnMarshal(EPCOContents)
		if unmarshalErr != nil {
			return nil, 0, 0, fmt.Errorf("parsing PCO failed: %v", unmarshalErr)
		}

		// Send MTU to UE always even if UE does not request it.
		// Preconfiguring MTU request flag.

		pco.IPv4LinkMTURequest = true

		for _, container := range protocolConfigurationOptions.ProtocolOrContainerList {
			switch container.ProtocolOrContainerID {
			case nasMessage.DNSServerIPv6AddressRequestUL:
				pco.DNSIPv6Request = true
			case nasMessage.DNSServerIPv4AddressRequestUL:
				pco.DNSIPv4Request = true
			default:
				continue
			}
		}
	}

	return pco, selectedPDUSessionType, estAcceptCause5gSMValue, nil
}

func HandlePDUSessionReleaseRequest(ctx context.Context, supi string) error {
	err := ReleaseUeIPAddr(ctx, supi)
	if err != nil {
		return fmt.Errorf("failed to release UE IP Addr: %v", err)
	}

	return nil
}
