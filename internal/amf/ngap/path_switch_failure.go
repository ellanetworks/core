// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// sendPathSwitchRequestFailure rejects a Path Switch Request, naming every
// distinct requested PDU session in the mandatory PDU Session Resource Released
// List, each with an AMF-generated Path Switch Request Unsuccessful Transfer
// carrying causeValue (TS 38.413; the AMF generating the transfer is the note
// exception).
func sendPathSwitchRequestFailure(ctx context.Context, ran *amf.Radio, msg decode.PathSwitchRequest, causeValue aper.Enumerated) {
	released, err := pathSwitchReleasedList(msg.PDUSessionResourceItems, causeValue)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error building path switch released list", zap.Error(err))
	}

	if err := ran.NGAPSender.SendPathSwitchRequestFailure(ctx, msg.SourceAMFUENGAPID, msg.RANUENGAPID, released, nil); err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error sending path switch request failure", zap.Error(err))
	}
}

// pathSwitchReleasedList builds the PDU Session Resource Released List for a
// Path Switch Request Failure (TS 38.413): one item per distinct
// requested PDU session, each carrying an Unsuccessful Transfer with causeValue.
func pathSwitchReleasedList(items []ngapType.PDUSessionResourceToBeSwitchedDLItem, causeValue aper.Enumerated) (*ngapType.PDUSessionResourceReleasedListPSFail, error) {
	transfer, err := buildPathSwitchRequestUnsuccessfulTransfer(causeValue)
	if err != nil {
		return nil, err
	}

	list := &ngapType.PDUSessionResourceReleasedListPSFail{}
	seen := make(map[int64]struct{}, len(items))

	for _, item := range items {
		id := item.PDUSessionID.Value
		if _, dup := seen[id]; dup {
			continue
		}

		seen[id] = struct{}{}

		list.List = append(list.List, ngapType.PDUSessionResourceReleasedItemPSFail{
			PDUSessionID:                          ngapType.PDUSessionID{Value: id},
			PathSwitchRequestUnsuccessfulTransfer: transfer,
		})
	}

	return list, nil
}

// buildPathSwitchRequestUnsuccessfulTransfer encodes a Path Switch Request
// Unsuccessful Transfer (TS 38.413) carrying a radio-network cause.
func buildPathSwitchRequestUnsuccessfulTransfer(causeValue aper.Enumerated) ([]byte, error) {
	transfer := ngapType.PathSwitchRequestUnsuccessfulTransfer{
		Cause: ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: causeValue},
		},
	}

	buf, err := aper.MarshalWithParams(transfer, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("encode path switch request unsuccessful transfer: %w", err)
	}

	return buf, nil
}
