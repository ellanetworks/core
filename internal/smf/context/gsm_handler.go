// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

type ProtocolConfigurationOptions struct {
	DNSIPv4Request     bool
	DNSIPv6Request     bool
	IPv4LinkMTURequest bool
}

func (smContext *SMContext) HandlePDUSessionEstablishmentRequest(req *nasMessage.PDUSessionEstablishmentRequest) (*ProtocolConfigurationOptions, uint8, error) {
	smContext.PDUSessionID = int32(req.PDUSessionID.GetPDUSessionID())

	smContext.Pti = req.GetPTI()

	// Handle PDUSessionType
	selectedPDUSessionType := nasMessage.PDUSessionTypeIPv4
	if req.PDUSessionType != nil {
		selectedPDUSessionType = req.PDUSessionType.GetPDUSessionTypeValue()
		err := smContext.isAllowedPDUSessionType(selectedPDUSessionType)
		if err != nil {
			return nil, 0, fmt.Errorf("requested PDUSessionType is not allowed: %v", err)
		}
	}

	pco := &ProtocolConfigurationOptions{}

	if req.ExtendedProtocolConfigurationOptions != nil {
		EPCOContents := req.ExtendedProtocolConfigurationOptions.GetExtendedProtocolConfigurationOptionsContents()
		protocolConfigurationOptions := nasConvert.NewProtocolConfigurationOptions()
		unmarshalErr := protocolConfigurationOptions.UnMarshal(EPCOContents)
		if unmarshalErr != nil {
			logger.SmfLog.Error("Parsing PCO failed", zap.Error(unmarshalErr), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
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

	return pco, selectedPDUSessionType, nil
}

func (smContext *SMContext) HandlePDUSessionReleaseRequest(ctx context.Context, req *nasMessage.PDUSessionReleaseRequest) {
	smContext.Pti = req.GetPTI()
	err := smContext.ReleaseUeIPAddr(ctx)
	if err != nil {
		logger.SmfLog.Error("Releasing UE IP Addr", zap.Error(err), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
		return
	}
	logger.SmfLog.Info("Successfully completed PDU Session Release Request", zap.Int32("PDUSessionID", smContext.PDUSessionID))
}
