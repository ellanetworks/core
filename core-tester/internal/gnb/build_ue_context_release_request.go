package gnb

import (
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

type UeContextReleaseRequestBuilder struct {
	pdu ngapType.NGAPPDU
	ies *ngapType.ProtocolIEContainerUEContextReleaseRequestIEs
}

type UEContextReleaseRequestOpts struct {
	AMFUENGAPID   int64
	RANUENGAPID   int64
	PDUSessionIDs [16]bool
	Cause         aper.Enumerated
}

func BuildUEContextReleaseRequest(opts *UEContextReleaseRequestOpts) (ngapType.NGAPPDU, error) {
	return NewUeContextReleaseRequestBuilder().
		SetAmfUeNgapId(opts.AMFUENGAPID).SetRanUeNgapId(opts.RANUENGAPID).
		SetPduSessionResourceListCxtRelReq(opts.PDUSessionIDs).
		SetCause(opts.Cause).
		Build()
}

func NewUeContextReleaseRequestBuilder() *UeContextReleaseRequestBuilder {
	pdu := ngapType.NGAPPDU{}

	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeUEContextReleaseRequest
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentUEContextReleaseRequest
	initiatingMessage.Value.UEContextReleaseRequest = new(ngapType.UEContextReleaseRequest)

	uEContextReleaseRequest := initiatingMessage.Value.UEContextReleaseRequest
	ies := &uEContextReleaseRequest.ProtocolIEs

	return &UeContextReleaseRequestBuilder{pdu, ies}
}

func (builder *UeContextReleaseRequestBuilder) SetAmfUeNgapId(amfUeNgapID int64) *UeContextReleaseRequestBuilder {
	// AMF UE NGAP ID
	ie := ngapType.UEContextReleaseRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUeNgapID

	builder.ies.List = append(builder.ies.List, ie)

	return builder
}

func (builder *UeContextReleaseRequestBuilder) SetRanUeNgapId(ranUeNgapID int64) *UeContextReleaseRequestBuilder {
	// RAN UE NGAP ID
	ie := ngapType.UEContextReleaseRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequiredIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUeNgapID

	builder.ies.List = append(builder.ies.List, ie)

	return builder
}

func (builder *UeContextReleaseRequestBuilder) SetPduSessionResourceListCxtRelReq(pduSessions [16]bool) *UeContextReleaseRequestBuilder {
	if len(pduSessions) > 0 {
		ie := ngapType.UEContextReleaseRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceListCxtRelReq
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.UEContextReleaseRequestIEsPresentPDUSessionResourceListCxtRelReq
		ie.Value.PDUSessionResourceListCxtRelReq = new(ngapType.PDUSessionResourceListCxtRelReq)

		pDUSessionResourceListCxtRelReq := ie.Value.PDUSessionResourceListCxtRelReq

		// PDU Session Resource Item in PDU session Resource List
		for i, pduSessionID := range pduSessions {
			if !pduSessionID {
				continue
			}

			pDUSessionResourceItem := ngapType.PDUSessionResourceItemCxtRelReq{}
			pDUSessionResourceItem.PDUSessionID.Value = int64(i)
			pDUSessionResourceListCxtRelReq.List = append(pDUSessionResourceListCxtRelReq.List, pDUSessionResourceItem)
		}

		builder.ies.List = append(builder.ies.List, ie)
	}

	return builder
}

func (builder *UeContextReleaseRequestBuilder) SetCause(causeValue aper.Enumerated) *UeContextReleaseRequestBuilder {
	ie := ngapType.UEContextReleaseRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDCause
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.UEContextReleaseRequestIEsPresentCause
	ie.Value.Cause = new(ngapType.Cause)

	cause := ie.Value.Cause
	cause.Present = ngapType.CausePresentRadioNetwork
	cause.RadioNetwork = new(ngapType.CauseRadioNetwork)
	cause.RadioNetwork.Value = causeValue

	builder.ies.List = append(builder.ies.List, ie)

	return builder
}

func (builder *UeContextReleaseRequestBuilder) Build() (ngapType.NGAPPDU, error) {
	return builder.pdu, nil
}
