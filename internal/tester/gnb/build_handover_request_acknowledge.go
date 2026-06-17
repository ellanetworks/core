// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package gnb

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

// HandoverRequestAcknowledgeOpts contains the parameters needed to build a
// HandoverRequestAcknowledge message (target gNB → AMF).
type HandoverRequestAcknowledgeOpts struct {
	AMFUENGAPID int64
	RANUENGAPID int64

	// PDUSessions lists the admitted PDU sessions with their DL tunnel info.
	PDUSessions []HandoverAdmittedPDUSession

	// TargetToSourceTransparentContainer is an opaque RRC container.
	TargetToSourceTransparentContainer []byte
}

// HandoverAdmittedPDUSession describes one admitted PDU session.
type HandoverAdmittedPDUSession struct {
	PDUSessionID int64
	DLTeid       uint32
	DLIP         netip.Addr
}

// BuildHandoverRequestAcknowledge constructs an NGAP HandoverRequestAcknowledge PDU.
func BuildHandoverRequestAcknowledge(opts *HandoverRequestAcknowledgeOpts) (ngapType.NGAPPDU, error) {
	pdu := ngapType.NGAPPDU{}

	msg := &ngapType.HandoverRequestAcknowledge{}
	ies := &msg.ProtocolIEs

	// AMF UE NGAP ID
	{
		ie := ngapType.HandoverRequestAcknowledgeIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.HandoverRequestAcknowledgeIEsPresentAMFUENGAPID
		ie.Value.AMFUENGAPID = &ngapType.AMFUENGAPID{Value: opts.AMFUENGAPID}
		ies.List = append(ies.List, ie)
	}

	// RAN UE NGAP ID
	{
		ie := ngapType.HandoverRequestAcknowledgeIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.HandoverRequestAcknowledgeIEsPresentRANUENGAPID
		ie.Value.RANUENGAPID = &ngapType.RANUENGAPID{Value: opts.RANUENGAPID}
		ies.List = append(ies.List, ie)
	}

	// PDU Session Resource Admitted List
	{
		ie := ngapType.HandoverRequestAcknowledgeIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceAdmittedList
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.HandoverRequestAcknowledgeIEsPresentPDUSessionResourceAdmittedList

		list := &ngapType.PDUSessionResourceAdmittedList{}

		for _, ps := range opts.PDUSessions {
			transfer, err := buildHandoverRequestAcknowledgeTransfer(ps.DLTeid, ps.DLIP)
			if err != nil {
				return pdu, fmt.Errorf("build transfer for session %d: %v", ps.PDUSessionID, err)
			}

			item := ngapType.PDUSessionResourceAdmittedItem{
				PDUSessionID:                       ngapType.PDUSessionID{Value: ps.PDUSessionID},
				HandoverRequestAcknowledgeTransfer: transfer,
			}
			list.List = append(list.List, item)
		}

		ie.Value.PDUSessionResourceAdmittedList = list
		ies.List = append(ies.List, ie)
	}

	// Target to Source Transparent Container
	{
		ie := ngapType.HandoverRequestAcknowledgeIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDTargetToSourceTransparentContainer
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverRequestAcknowledgeIEsPresentTargetToSourceTransparentContainer

		container := opts.TargetToSourceTransparentContainer
		if container == nil {
			container = []byte{0x00}
		}

		ie.Value.TargetToSourceTransparentContainer = &ngapType.TargetToSourceTransparentContainer{Value: container}
		ies.List = append(ies.List, ie)
	}

	pdu.Present = ngapType.NGAPPDUPresentSuccessfulOutcome
	pdu.SuccessfulOutcome = new(ngapType.SuccessfulOutcome)
	pdu.SuccessfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodeHandoverResourceAllocation
	pdu.SuccessfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject
	pdu.SuccessfulOutcome.Value.Present = ngapType.SuccessfulOutcomePresentHandoverRequestAcknowledge
	pdu.SuccessfulOutcome.Value.HandoverRequestAcknowledge = msg

	return pdu, nil
}

// buildHandoverRequestAcknowledgeTransfer builds the APER-encoded
// HandoverRequestAcknowledgeTransfer containing the DL GTP tunnel info
// for the target gNB.
func buildHandoverRequestAcknowledgeTransfer(teid uint32, ip netip.Addr) ([]byte, error) {
	transfer := ngapType.HandoverRequestAcknowledgeTransfer{}

	var ipBytes []byte

	if ip.Is4() {
		v4 := ip.As4()
		ipBytes = v4[:]
	} else {
		v6 := ip.As16()
		ipBytes = v6[:]
	}

	teidBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(teidBytes, teid)

	transfer.DLNGUUPTNLInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	transfer.DLNGUUPTNLInformation.GTPTunnel = &ngapType.GTPTunnel{
		TransportLayerAddress: ngapType.TransportLayerAddress{
			Value: aper.BitString{
				Bytes:     ipBytes,
				BitLength: uint64(len(ipBytes) * 8),
			},
		},
		GTPTEID: ngapType.GTPTEID{Value: teidBytes},
	}

	// QosFlowSetupResponseList is mandatory.
	transfer.QosFlowSetupResponseList.List = append(transfer.QosFlowSetupResponseList.List,
		ngapType.QosFlowItemWithDataForwarding{
			QosFlowIdentifier: ngapType.QosFlowIdentifier{Value: 1},
		},
	)

	buf, err := aper.MarshalWithParams(transfer, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("marshal HandoverRequestAcknowledgeTransfer: %v", err)
	}

	return buf, nil
}
