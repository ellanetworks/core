package gnb

import (
	"fmt"
	"net/netip"

	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

type PathSwitchRequestOpts struct {
	RANUENGAPID            int64
	SourceAMFUENGAPID      int64
	PDUSessions            [16]*PDUSessionInformation
	N3GnbIp                netip.Addr
	UESecurityCapabilities *ngapType.UESecurityCapabilities
}

func BuildPathSwitchRequest(opts *PathSwitchRequestOpts) (ngapType.NGAPPDU, error) {
	pdu := ngapType.NGAPPDU{}

	ranUeNgapID := &ngapType.RANUENGAPID{Value: opts.RANUENGAPID}
	sourceAmfUeNgapID := &ngapType.AMFUENGAPID{Value: opts.SourceAMFUENGAPID}

	pduSessionDLList := &ngapType.PDUSessionResourceToBeSwitchedDLList{}

	for _, pduSession := range opts.PDUSessions {
		if pduSession == nil {
			continue
		}

		ip4 := opts.N3GnbIp.As4()

		transfer, err := buildPathSwitchRequestTransfer(pduSession.DLTeid, ip4[:])
		if err != nil {
			return pdu, fmt.Errorf("failed to build PathSwitchRequestTransfer: %v", err)
		}

		item := ngapType.PDUSessionResourceToBeSwitchedDLItem{
			PDUSessionID:              ngapType.PDUSessionID{Value: pduSession.PDUSessionID},
			PathSwitchRequestTransfer: transfer,
		}
		pduSessionDLList.List = append(pduSessionDLList.List, item)
	}

	msg := buildPathSwitchRequest(
		sourceAmfUeNgapID,
		ranUeNgapID,
		pduSessionDLList,
		nil, // no failed list
		opts.UESecurityCapabilities,
	)

	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)
	pdu.InitiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodePathSwitchRequest
	pdu.InitiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject
	pdu.InitiatingMessage.Value.Present = ngapType.InitiatingMessagePresentPathSwitchRequest
	pdu.InitiatingMessage.Value.PathSwitchRequest = msg

	return pdu, nil
}

func buildPathSwitchRequestTransfer(teid uint32, ip []byte) ([]byte, error) {
	transfer := ngapType.PathSwitchRequestTransfer{}
	transfer.DLNGUUPTNLInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	transfer.DLNGUUPTNLInformation.GTPTunnel = new(ngapType.GTPTunnel)

	teidBytes := make([]byte, 4)
	teidBytes[0] = byte(teid >> 24)
	teidBytes[1] = byte(teid >> 16)
	teidBytes[2] = byte(teid >> 8)
	teidBytes[3] = byte(teid)
	transfer.DLNGUUPTNLInformation.GTPTunnel.GTPTEID.Value = teidBytes
	transfer.DLNGUUPTNLInformation.GTPTunnel.TransportLayerAddress.Value = aper.BitString{
		Bytes:     ip,
		BitLength: uint64(len(ip) * 8),
	}

	// QosFlowAcceptedList is mandatory (sizeLB:1)
	transfer.QosFlowAcceptedList.List = append(transfer.QosFlowAcceptedList.List,
		ngapType.QosFlowAcceptedItem{
			QosFlowIdentifier: ngapType.QosFlowIdentifier{Value: 1},
		},
	)

	buf, err := aper.MarshalWithParams(transfer, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal PathSwitchRequestTransfer: %v", err)
	}

	return buf, nil
}

func buildPathSwitchRequest(
	sourceAmfUeNgapID *ngapType.AMFUENGAPID,
	ranUeNgapID *ngapType.RANUENGAPID,
	pduSessionDLList *ngapType.PDUSessionResourceToBeSwitchedDLList,
	failedList *ngapType.PDUSessionResourceFailedToSetupListPSReq,
	uESecurityCapabilities *ngapType.UESecurityCapabilities,
) *ngapType.PathSwitchRequest {
	msg := &ngapType.PathSwitchRequest{}
	ies := &msg.ProtocolIEs

	if ranUeNgapID != nil {
		ie := ngapType.PathSwitchRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.PathSwitchRequestIEsPresentRANUENGAPID
		ie.Value.RANUENGAPID = ranUeNgapID
		ies.List = append(ies.List, ie)
	}

	if sourceAmfUeNgapID != nil {
		ie := ngapType.PathSwitchRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDSourceAMFUENGAPID
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.PathSwitchRequestIEsPresentSourceAMFUENGAPID
		ie.Value.SourceAMFUENGAPID = sourceAmfUeNgapID
		ies.List = append(ies.List, ie)
	}

	if pduSessionDLList != nil {
		ie := ngapType.PathSwitchRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceToBeSwitchedDLList
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.PathSwitchRequestIEsPresentPDUSessionResourceToBeSwitchedDLList
		ie.Value.PDUSessionResourceToBeSwitchedDLList = pduSessionDLList
		ies.List = append(ies.List, ie)
	}

	if failedList != nil {
		ie := ngapType.PathSwitchRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListPSReq
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PathSwitchRequestIEsPresentPDUSessionResourceFailedToSetupListPSReq
		ie.Value.PDUSessionResourceFailedToSetupListPSReq = failedList
		ies.List = append(ies.List, ie)
	}

	if uESecurityCapabilities != nil {
		ie := ngapType.PathSwitchRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDUESecurityCapabilities
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PathSwitchRequestIEsPresentUESecurityCapabilities
		ie.Value.UESecurityCapabilities = uESecurityCapabilities
		ies.List = append(ies.List, ie)
	}

	return msg
}
