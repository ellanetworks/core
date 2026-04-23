package gnb

import (
	"fmt"

	"github.com/free5gc/ngap/ngapType"
)

type UEContextReleaseCompleteOpts struct {
	AMFUENGAPID   int64
	RANUENGAPID   int64
	PDUSessionIDs [16]bool
}

func BuildUEContextReleaseComplete(opts *UEContextReleaseCompleteOpts) (ngapType.NGAPPDU, error) {
	if opts == nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("UEContextReleaseCompleteOpts is nil")
	}

	pdu := ngapType.NGAPPDU{}
	pdu.Present = ngapType.NGAPPDUPresentSuccessfulOutcome
	pdu.SuccessfulOutcome = new(ngapType.SuccessfulOutcome)

	successfulOutcome := pdu.SuccessfulOutcome
	successfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodeUEContextRelease
	successfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject

	successfulOutcome.Value.Present = ngapType.SuccessfulOutcomePresentUEContextReleaseComplete
	successfulOutcome.Value.UEContextReleaseComplete = new(ngapType.UEContextReleaseComplete)

	ueContextReleaseComplete := successfulOutcome.Value.UEContextReleaseComplete
	ueContextReleaseCompleteIEs := &ueContextReleaseComplete.ProtocolIEs

	// AMF UE NGAP ID
	ie := ngapType.UEContextReleaseCompleteIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.UEContextReleaseCompleteIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = opts.AMFUENGAPID

	ueContextReleaseCompleteIEs.List = append(ueContextReleaseCompleteIEs.List, ie)

	// RAN UE NGAP ID
	ie = ngapType.UEContextReleaseCompleteIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.UEContextReleaseCompleteIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = opts.RANUENGAPID

	ueContextReleaseCompleteIEs.List = append(ueContextReleaseCompleteIEs.List, ie)

	ie = ngapType.UEContextReleaseCompleteIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceListCxtRelCpl
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.UEContextReleaseCompleteIEsPresentPDUSessionResourceListCxtRelCpl
	ie.Value.PDUSessionResourceListCxtRelCpl = new(ngapType.PDUSessionResourceListCxtRelCpl)

	pDUSessionResourceListCxtRelCompl := ie.Value.PDUSessionResourceListCxtRelCpl

	for i, pduSessionID := range opts.PDUSessionIDs {
		if !pduSessionID {
			continue
		}

		pDUSessionResourceItem := ngapType.PDUSessionResourceItemCxtRelCpl{}
		pDUSessionResourceItem.PDUSessionID.Value = int64(i)
		pDUSessionResourceListCxtRelCompl.List = append(pDUSessionResourceListCxtRelCompl.List, pDUSessionResourceItem)
	}

	if len(pDUSessionResourceListCxtRelCompl.List) > 0 {
		ueContextReleaseCompleteIEs.List = append(ueContextReleaseCompleteIEs.List, ie)
	}

	return pdu, nil
}
