// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/ngap"
)

type HandoverRequestAcknowledgeOpts struct {
	AMFUENGAPID int64
	RANUENGAPID int64

	PDUSessions []HandoverAdmittedPDUSession

	// Opaque RRC container.
	TargetToSourceTransparentContainer []byte
}

type HandoverAdmittedPDUSession struct {
	PDUSessionID int64
	DLTeid       uint32
	DLIP         netip.Addr
}

func BuildHandoverRequestAcknowledge(opts *HandoverRequestAcknowledgeOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("HandoverRequestAcknowledgeOpts is nil")
	}

	admitted := make(ngap.PDUSessionResourceAdmittedList, 0, len(opts.PDUSessions))

	for _, ps := range opts.PDUSessions {
		transfer, err := buildHandoverRequestAcknowledgeTransfer(ps.DLTeid, ps.DLIP)
		if err != nil {
			return nil, fmt.Errorf("build transfer for session %d: %w", ps.PDUSessionID, err)
		}

		admitted = append(admitted, ngap.PDUSessionResourceAdmittedItem{
			PDUSessionID: ngap.PDUSessionID(ps.PDUSessionID),
			Transfer:     transfer,
		})
	}

	container := opts.TargetToSourceTransparentContainer
	if container == nil {
		container = []byte{0x00}
	}

	msg := &ngap.HandoverRequestAcknowledge{
		AMFUENGAPID:                        ngap.Ptr(ngap.AMFUENGAPID(opts.AMFUENGAPID)),
		RANUENGAPID:                        ngap.Ptr(ngap.RANUENGAPID(opts.RANUENGAPID)),
		PDUSessionResourceAdmittedList:     admitted,
		TargetToSourceTransparentContainer: container,
	}

	return msg.Marshal()
}

func buildHandoverRequestAcknowledgeTransfer(teid uint32, ip netip.Addr) (ngap.TransferContainer, error) {
	addr, err := transportLayerAddress(ip)
	if err != nil {
		return nil, err
	}

	return (&ngap.HandoverRequestAcknowledgeTransfer{
		DLNGUUPTNLInformation: ngap.UPTransportLayerInformation{GTPTunnel: ngap.GTPTunnel{
			TransportLayerAddress: addr,
			GTPTEID:               ngap.GTPTEID(teid),
		}},
		// QosFlowSetupResponseList is mandatory.
		QosFlowSetupResponse: ngap.QosFlowListWithDataForwarding{{QosFlowIdentifier: 1}},
	}).Marshal()
}
