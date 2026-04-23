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
	Mcc                    string
	Mnc                    string
	Tac                    string
	GnbID                  string
}

func BuildPathSwitchRequest(opts *PathSwitchRequestOpts) (ngapType.NGAPPDU, error) {
	pdu := ngapType.NGAPPDU{}

	if opts.Mcc == "" {
		return pdu, fmt.Errorf("MCC is required to build PathSwitchRequest")
	}

	if opts.Mnc == "" {
		return pdu, fmt.Errorf("MNC is required to build PathSwitchRequest")
	}

	if opts.Tac == "" {
		return pdu, fmt.Errorf("TAC is required to build PathSwitchRequest")
	}

	if opts.GnbID == "" {
		return pdu, fmt.Errorf("GNB ID is required to build PathSwitchRequest")
	}

	plmnID, err := GetMccAndMncInOctets(opts.Mcc, opts.Mnc)
	if err != nil {
		return pdu, fmt.Errorf("failed to get plmnID: %+v", err)
	}

	plmnIdentity := GetPLMNIdentity(opts.Mcc, opts.Mnc)

	tac, err := GetTacInBytes(opts.Tac)
	if err != nil {
		return pdu, fmt.Errorf("failed to get tac: %+v", err)
	}

	nrCellID, err := GetNRCellIdentity(opts.GnbID)
	if err != nil {
		return pdu, fmt.Errorf("failed to get nrCellID: %+v", err)
	}

	ranUeNgapID := &ngapType.RANUENGAPID{Value: opts.RANUENGAPID}
	sourceAmfUeNgapID := &ngapType.AMFUENGAPID{Value: opts.SourceAMFUENGAPID}

	userLocationInformation := &ngapType.UserLocationInformation{
		Present:                   ngapType.UserLocationInformationPresentUserLocationInformationNR,
		UserLocationInformationNR: &ngapType.UserLocationInformationNR{},
	}
	userLocationInformation.UserLocationInformationNR.NRCGI.PLMNIdentity = plmnIdentity
	userLocationInformation.UserLocationInformationNR.NRCGI.NRCellIdentity = nrCellID
	userLocationInformation.UserLocationInformationNR.TAI.PLMNIdentity.Value = plmnID
	userLocationInformation.UserLocationInformationNR.TAI.TAC.Value = tac

	pduSessionDLList := &ngapType.PDUSessionResourceToBeSwitchedDLList{}

	for _, pduSession := range opts.PDUSessions {
		if pduSession == nil {
			continue
		}

		var ipBytes []byte

		if opts.N3GnbIp.Is4() {
			ip4 := opts.N3GnbIp.As4()
			ipBytes = ip4[:]
		} else {
			ip6 := opts.N3GnbIp.As16()
			ipBytes = ip6[:]
		}

		transfer, err := buildPathSwitchRequestTransfer(pduSession.DLTeid, ipBytes)
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
		userLocationInformation,
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
	userLocationInformation *ngapType.UserLocationInformation,
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

	if userLocationInformation != nil {
		ie := ngapType.PathSwitchRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDUserLocationInformation
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PathSwitchRequestIEsPresentUserLocationInformation
		ie.Value.UserLocationInformation = userLocationInformation
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
