package gnb

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
)

type InitialContextSetupResponseOpts struct {
	AMFUENGAPID int64
	RANUENGAPID int64
	N3GnbIp     netip.Addr
	PDUSessions [16]*PDUSessionInformation
}

func BuildInitialContextSetupResponse(opts *InitialContextSetupResponseOpts) (ngapType.NGAPPDU, error) {
	pdu := ngapType.NGAPPDU{}

	pdu.Present = ngapType.NGAPPDUPresentSuccessfulOutcome
	pdu.SuccessfulOutcome = new(ngapType.SuccessfulOutcome)

	successfulOutcome := pdu.SuccessfulOutcome
	successfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodeInitialContextSetup
	successfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject

	successfulOutcome.Value.Present = ngapType.SuccessfulOutcomePresentInitialContextSetupResponse
	successfulOutcome.Value.InitialContextSetupResponse = new(ngapType.InitialContextSetupResponse)

	ies := &successfulOutcome.Value.InitialContextSetupResponse.ProtocolIEs

	amfIE := ngapType.InitialContextSetupResponseIEs{}
	amfIE.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	amfIE.Criticality.Value = ngapType.CriticalityPresentReject
	amfIE.Value.Present = ngapType.HandoverRequiredIEsPresentAMFUENGAPID
	amfIE.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := amfIE.Value.AMFUENGAPID
	aMFUENGAPID.Value = opts.AMFUENGAPID

	ies.List = append(ies.List, amfIE)

	ranIE := ngapType.InitialContextSetupResponseIEs{}
	ranIE.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ranIE.Criticality.Value = ngapType.CriticalityPresentReject
	ranIE.Value.Present = ngapType.HandoverRequiredIEsPresentRANUENGAPID
	ranIE.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ranIE.Value.RANUENGAPID
	rANUENGAPID.Value = opts.RANUENGAPID

	ies.List = append(ies.List, ranIE)

	setupListIE := ngapType.InitialContextSetupResponseIEs{}
	setupListIE.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtRes
	setupListIE.Criticality.Value = ngapType.CriticalityPresentIgnore
	setupListIE.Value.Present = ngapType.InitialContextSetupResponseIEsPresentPDUSessionResourceSetupListCxtRes
	setupListIE.Value.PDUSessionResourceSetupListCxtRes = new(ngapType.PDUSessionResourceSetupListCxtRes)

	PDUSessionResourceSetupListCxtRes := setupListIE.Value.PDUSessionResourceSetupListCxtRes

	for _, pduSession := range opts.PDUSessions {
		if pduSession == nil {
			continue
		}

		pDUSessionResourceSetupItemCxtRes := ngapType.PDUSessionResourceSetupItemCxtRes{}

		transferData, err := GetPDUSessionResourceSetupResponseTransfer(opts.N3GnbIp, pduSession.DLTeid, pduSession.QFI)
		if err != nil {
			return pdu, fmt.Errorf("failed to get PDUSessionResourceSetupResponseTransfer: %v", err)
		}

		pDUSessionResourceSetupItemCxtRes.PDUSessionID.Value = pduSession.PDUSessionID
		pDUSessionResourceSetupItemCxtRes.PDUSessionResourceSetupResponseTransfer = transferData
		PDUSessionResourceSetupListCxtRes.List = append(PDUSessionResourceSetupListCxtRes.List, pDUSessionResourceSetupItemCxtRes)
	}

	if len(PDUSessionResourceSetupListCxtRes.List) > 0 {
		ies.List = append(ies.List, setupListIE)
	}

	return pdu, nil
}

func GetPDUSessionResourceSetupResponseTransfer(ipv4 netip.Addr, teid uint32, qosId int64) ([]byte, error) {
	data, err := buildPDUSessionResourceSetupResponseTransfer(ipv4, teid, qosId)
	if err != nil {
		return nil, fmt.Errorf("failed to build PDUSessionResourceSetupResponseTransfer: %v", err)
	}

	encodeData, err := aper.MarshalWithParams(data, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("failed to encode PDUSessionResourceSetupResponseTransfer: %v", err)
	}

	return encodeData, nil
}

type QosFlowItemExtIEsExtensionValue struct {
	Present int
}

type QosFlowItemExtIEs struct {
	Id             ngapType.ProtocolExtensionID
	Criticality    ngapType.Criticality
	ExtensionValue QosFlowItemExtIEsExtensionValue `aper:"openType,referenceFieldName:Id"`
}

type ProtocolExtensionContainerQosFlowItemExtIEs struct {
	List []QosFlowItemExtIEs `aper:"sizeLB:1,sizeUB:65535"`
}

type QosFlowItem struct {
	QosFlowIdentifier ngapType.QosFlowIdentifier
	Cause             ngapType.Cause                               `aper:"valueLB:0,valueUB:5"`
	IEExtensions      *ProtocolExtensionContainerQosFlowItemExtIEs `aper:"optional"`
}

type QosFlowList struct {
	List []QosFlowItem `aper:"valueExt,sizeLB:1,sizeUB:64"`
}

type PDUSessionResourceSetupResponseTransfer struct {
	QosFlowPerTNLInformation           ngapType.QosFlowPerTNLInformation                                                 `aper:"valueExt"`
	AdditionalQosFlowPerTNLInformation *ngapType.QosFlowPerTNLInformation                                                `aper:"valueExt,optional"`
	SecurityResult                     *ngapType.SecurityResult                                                          `aper:"valueExt,optional"`
	QosFlowFailedToSetupList           *QosFlowList                                                                      `aper:"optional"`
	IEExtensions                       *ngapType.ProtocolExtensionContainerPDUSessionResourceSetupResponseTransferExtIEs `aper:"optional"`
}

func buildPDUSessionResourceSetupResponseTransfer(ipv4 netip.Addr, teid uint32, qosId int64) (data PDUSessionResourceSetupResponseTransfer, err error) {
	// QoS Flow per TNL Information
	if !ipv4.IsValid() {
		return data, fmt.Errorf("invalid IPv4 address: %s", ipv4)
	}

	qosFlowPerTNLInformation := &data.QosFlowPerTNLInformation
	qosFlowPerTNLInformation.UPTransportLayerInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel

	// UP Transport Layer Information in QoS Flow per TNL Information
	upTransportLayerInformation := &qosFlowPerTNLInformation.UPTransportLayerInformation
	upTransportLayerInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	upTransportLayerInformation.GTPTunnel = new(ngapType.GTPTunnel)

	dowlinkTeid := binary.BigEndian.AppendUint32(nil, teid)
	upTransportLayerInformation.GTPTunnel.GTPTEID.Value = dowlinkTeid

	upTransportLayerInformation.GTPTunnel.TransportLayerAddress = ngapConvert.IPAddressToNgap(ipv4.String(), "")

	// Associated QoS Flow List in QoS Flow per TNL Information
	associatedQosFlowList := &qosFlowPerTNLInformation.AssociatedQosFlowList

	associatedQosFlowItem := ngapType.AssociatedQosFlowItem{}
	associatedQosFlowItem.QosFlowIdentifier.Value = qosId
	associatedQosFlowList.List = append(associatedQosFlowList.List, associatedQosFlowItem)

	return
}
