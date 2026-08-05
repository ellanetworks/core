// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/ngap"
)

type InitialContextSetupResponseOpts struct {
	AMFUENGAPID int64
	RANUENGAPID int64
	PDUSessions [16]*PDUSessionInformation
}

func BuildInitialContextSetupResponse(opts *InitialContextSetupResponseOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("InitialContextSetupResponseOpts is nil")
	}

	msg := &ngap.InitialContextSetupResponse{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(opts.AMFUENGAPID)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(opts.RANUENGAPID)),
	}

	for _, pduSession := range opts.PDUSessions {
		if pduSession == nil {
			continue
		}

		transfer, err := GetPDUSessionResourceSetupResponseTransfer(pduSession.N3GnbIp, pduSession.DLTeid, pduSession.QFI)
		if err != nil {
			return nil, fmt.Errorf("failed to get PDUSessionResourceSetupResponseTransfer: %v", err)
		}

		msg.PDUSessionResourceSetup = append(msg.PDUSessionResourceSetup, ngap.PDUSessionResourceSetupItemCxtRes{
			PDUSessionID: ngap.PDUSessionID(pduSession.PDUSessionID),
			Transfer:     transfer,
		})
	}

	return msg.Marshal()
}

// GetPDUSessionResourceSetupResponseTransfer encodes the per-session transfer
// naming the downlink tunnel this simulator has set up and the QoS flow it
// accepted (TS 38.413 §9.3.4.2).
func GetPDUSessionResourceSetupResponseTransfer(ip netip.Addr, teid uint32, qosID int64) (ngap.TransferContainer, error) {
	addr, err := transportLayerAddress(ip)
	if err != nil {
		return nil, err
	}

	transfer := &ngap.PDUSessionResourceSetupResponseTransfer{
		DLQosFlowPerTNLInformation: ngap.QosFlowPerTNLInformation{
			UPTransportLayerInformation: ngap.UPTransportLayerInformation{
				GTPTunnel: ngap.GTPTunnel{TransportLayerAddress: addr, GTPTEID: ngap.GTPTEID(teid)},
			},
			AssociatedQosFlowList: ngap.AssociatedQosFlowList{
				{QosFlowIdentifier: ngap.QosFlowIdentifier(qosID)},
			},
		},
	}

	return transfer.Marshal()
}
