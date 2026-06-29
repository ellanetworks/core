// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

type PDUSessionResourceModifyResponseOpts struct {
	AMFUENGAPID   int64
	RANUENGAPID   int64
	PDUSessionIDs []int64
}

func BuildPDUSessionResourceModifyResponse(opts *PDUSessionResourceModifyResponseOpts) (ngapType.NGAPPDU, error) {
	pdu := ngapType.NGAPPDU{}

	pdu.Present = ngapType.NGAPPDUPresentSuccessfulOutcome
	pdu.SuccessfulOutcome = new(ngapType.SuccessfulOutcome)

	successfulOutcome := pdu.SuccessfulOutcome
	successfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodePDUSessionResourceModify
	successfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject

	successfulOutcome.Value.Present = ngapType.SuccessfulOutcomePresentPDUSessionResourceModifyResponse
	successfulOutcome.Value.PDUSessionResourceModifyResponse = new(ngapType.PDUSessionResourceModifyResponse)

	modifyResponse := successfulOutcome.Value.PDUSessionResourceModifyResponse
	ies := &modifyResponse.ProtocolIEs

	amfIE := ngapType.PDUSessionResourceModifyResponseIEs{}
	amfIE.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	amfIE.Criticality.Value = ngapType.CriticalityPresentIgnore
	amfIE.Value.Present = ngapType.PDUSessionResourceModifyResponseIEsPresentAMFUENGAPID
	amfIE.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)
	amfIE.Value.AMFUENGAPID.Value = opts.AMFUENGAPID
	ies.List = append(ies.List, amfIE)

	ranIE := ngapType.PDUSessionResourceModifyResponseIEs{}
	ranIE.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ranIE.Criticality.Value = ngapType.CriticalityPresentIgnore
	ranIE.Value.Present = ngapType.PDUSessionResourceModifyResponseIEsPresentRANUENGAPID
	ranIE.Value.RANUENGAPID = new(ngapType.RANUENGAPID)
	ranIE.Value.RANUENGAPID.Value = opts.RANUENGAPID
	ies.List = append(ies.List, ranIE)

	modListIE := ngapType.PDUSessionResourceModifyResponseIEs{}
	modListIE.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceModifyListModRes
	modListIE.Criticality.Value = ngapType.CriticalityPresentIgnore
	modListIE.Value.Present = ngapType.PDUSessionResourceModifyResponseIEsPresentPDUSessionResourceModifyListModRes
	modListIE.Value.PDUSessionResourceModifyListModRes = new(ngapType.PDUSessionResourceModifyListModRes)

	modList := modListIE.Value.PDUSessionResourceModifyListModRes

	for _, pduSessionID := range opts.PDUSessionIDs {
		item := ngapType.PDUSessionResourceModifyItemModRes{}
		item.PDUSessionID.Value = pduSessionID

		transfer := &ngapType.PDUSessionResourceModifyResponseTransfer{}

		transferBytes, err := aper.MarshalWithParams(transfer, "valueExt")
		if err != nil {
			continue
		}

		item.PDUSessionResourceModifyResponseTransfer = transferBytes

		modList.List = append(modList.List, item)
	}

	ies.List = append(ies.List, modListIE)

	return pdu, nil
}
