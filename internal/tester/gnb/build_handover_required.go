// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"net/netip"

	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
)

type HandoverRequiredOpts struct {
	AMFUENGAPID int64
	RANUENGAPID int64

	// ngapType.HandoverTypeIntra5GS (0) or InterSystem (1).
	HandoverType int64

	CausePresent int
	CauseValue   int64

	TargetMcc   string
	TargetMnc   string
	TargetGnbID string
	TargetTac   string

	PDUSessions []HandoverRequiredPDUSession

	// Opaque RRC container.
	SourceToTargetTransparentContainer []byte
}

type HandoverRequiredPDUSession struct {
	PDUSessionID int64
	// APER-encoded transfer IE; a minimal default is built when nil.
	HandoverRequiredTransfer []byte
}

func BuildHandoverRequired(opts *HandoverRequiredOpts) (ngapType.NGAPPDU, error) {
	pdu := ngapType.NGAPPDU{}

	if opts.TargetMcc == "" || opts.TargetMnc == "" || opts.TargetGnbID == "" || opts.TargetTac == "" {
		return pdu, fmt.Errorf("target identity fields are required")
	}

	msg := &ngapType.HandoverRequired{}
	ies := &msg.ProtocolIEs

	{
		ie := ngapType.HandoverRequiredIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverRequiredIEsPresentAMFUENGAPID
		ie.Value.AMFUENGAPID = &ngapType.AMFUENGAPID{Value: opts.AMFUENGAPID}
		ies.List = append(ies.List, ie)
	}

	{
		ie := ngapType.HandoverRequiredIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverRequiredIEsPresentRANUENGAPID
		ie.Value.RANUENGAPID = &ngapType.RANUENGAPID{Value: opts.RANUENGAPID}
		ies.List = append(ies.List, ie)
	}

	{
		ie := ngapType.HandoverRequiredIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDHandoverType
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverRequiredIEsPresentHandoverType
		ie.Value.HandoverType = &ngapType.HandoverType{Value: aper.Enumerated(opts.HandoverType)}
		ies.List = append(ies.List, ie)
	}

	{
		ie := ngapType.HandoverRequiredIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDCause
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.HandoverRequiredIEsPresentCause

		cause := &ngapType.Cause{}
		if opts.CausePresent == 0 {
			cause.Present = ngapType.CausePresentRadioNetwork
			cause.RadioNetwork = &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentHandoverDesirableForRadioReason,
			}
		} else {
			cause.Present = opts.CausePresent
			switch cause.Present {
			case ngapType.CausePresentRadioNetwork:
				cause.RadioNetwork = &ngapType.CauseRadioNetwork{Value: aper.Enumerated(opts.CauseValue)}
			case ngapType.CausePresentTransport:
				cause.Transport = &ngapType.CauseTransport{Value: aper.Enumerated(opts.CauseValue)}
			case ngapType.CausePresentNas:
				cause.Nas = &ngapType.CauseNas{Value: aper.Enumerated(opts.CauseValue)}
			case ngapType.CausePresentProtocol:
				cause.Protocol = &ngapType.CauseProtocol{Value: aper.Enumerated(opts.CauseValue)}
			case ngapType.CausePresentMisc:
				cause.Misc = &ngapType.CauseMisc{Value: aper.Enumerated(opts.CauseValue)}
			}
		}

		ie.Value.Cause = cause
		ies.List = append(ies.List, ie)
	}

	{
		ie := ngapType.HandoverRequiredIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDTargetID
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverRequiredIEsPresentTargetID

		plmnID, err := GetMccAndMncInOctets(opts.TargetMcc, opts.TargetMnc)
		if err != nil {
			return pdu, fmt.Errorf("target PLMN: %v", err)
		}

		tac, err := GetTacInBytes(opts.TargetTac)
		if err != nil {
			return pdu, fmt.Errorf("target TAC: %v", err)
		}

		gnbIDBitString := ngapConvert.HexToBitString(opts.TargetGnbID, 24)

		targetID := &ngapType.TargetID{
			Present: ngapType.TargetIDPresentTargetRANNodeID,
			TargetRANNodeID: &ngapType.TargetRANNodeID{
				GlobalRANNodeID: ngapType.GlobalRANNodeID{
					Present: ngapType.GlobalRANNodeIDPresentGlobalGNBID,
					GlobalGNBID: &ngapType.GlobalGNBID{
						PLMNIdentity: ngapType.PLMNIdentity{Value: plmnID},
						GNBID: ngapType.GNBID{
							Present: ngapType.GNBIDPresentGNBID,
							GNBID:   &gnbIDBitString,
						},
					},
				},
				SelectedTAI: ngapType.TAI{
					PLMNIdentity: ngapType.PLMNIdentity{Value: plmnID},
					TAC:          ngapType.TAC{Value: tac},
				},
			},
		}

		ie.Value.TargetID = targetID
		ies.List = append(ies.List, ie)
	}

	{
		ie := ngapType.HandoverRequiredIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceListHORqd
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverRequiredIEsPresentPDUSessionResourceListHORqd

		list := &ngapType.PDUSessionResourceListHORqd{}

		for _, ps := range opts.PDUSessions {
			transfer := ps.HandoverRequiredTransfer
			if transfer == nil {
				var err error

				transfer, err = buildMinimalHandoverRequiredTransfer()
				if err != nil {
					return pdu, fmt.Errorf("build HandoverRequiredTransfer for session %d: %v", ps.PDUSessionID, err)
				}
			}

			item := ngapType.PDUSessionResourceItemHORqd{
				PDUSessionID:             ngapType.PDUSessionID{Value: ps.PDUSessionID},
				HandoverRequiredTransfer: transfer,
			}
			list.List = append(list.List, item)
		}

		ie.Value.PDUSessionResourceListHORqd = list
		ies.List = append(ies.List, ie)
	}

	{
		ie := ngapType.HandoverRequiredIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDSourceToTargetTransparentContainer
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverRequiredIEsPresentSourceToTargetTransparentContainer

		container := opts.SourceToTargetTransparentContainer
		if container == nil {
			// Minimal opaque container (the target gNB passes it through).
			container = []byte{0x00}
		}

		ie.Value.SourceToTargetTransparentContainer = &ngapType.SourceToTargetTransparentContainer{Value: container}
		ies.List = append(ies.List, ie)
	}

	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)
	pdu.InitiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeHandoverPreparation
	pdu.InitiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject
	pdu.InitiatingMessage.Value.Present = ngapType.InitiatingMessagePresentHandoverRequired
	pdu.InitiatingMessage.Value.HandoverRequired = msg

	return pdu, nil
}

// buildMinimalHandoverRequiredTransfer builds an empty APER-encoded transfer;
// the SMF decodes it but requires no content for the basic flow.
func buildMinimalHandoverRequiredTransfer() ([]byte, error) {
	transfer := ngapType.HandoverRequiredTransfer{}

	buf, err := aper.MarshalWithParams(transfer, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("marshal HandoverRequiredTransfer: %v", err)
	}

	return buf, nil
}

func BuildHandoverRequiredTransferWithDirectForwarding(_ netip.Addr) ([]byte, error) {
	transfer := ngapType.HandoverRequiredTransfer{
		DirectForwardingPathAvailability: &ngapType.DirectForwardingPathAvailability{
			Value: ngapType.DirectForwardingPathAvailabilityPresentDirectPathAvailable,
		},
	}

	buf, err := aper.MarshalWithParams(transfer, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("marshal HandoverRequiredTransfer: %v", err)
	}

	return buf, nil
}
