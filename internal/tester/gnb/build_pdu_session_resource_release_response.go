// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

type PDUSessionResourceReleaseResponseOpts struct {
	AMFUENGAPID   int64
	RANUENGAPID   int64
	PDUSessionIDs []int64
}

func BuildPDUSessionResourceReleaseResponse(opts *PDUSessionResourceReleaseResponseOpts) (ngapType.NGAPPDU, error) {
	pdu := ngapType.NGAPPDU{}

	pdu.Present = ngapType.NGAPPDUPresentSuccessfulOutcome
	pdu.SuccessfulOutcome = new(ngapType.SuccessfulOutcome)

	successfulOutcome := pdu.SuccessfulOutcome
	successfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodePDUSessionResourceRelease
	successfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject

	successfulOutcome.Value.Present = ngapType.SuccessfulOutcomePresentPDUSessionResourceReleaseResponse
	successfulOutcome.Value.PDUSessionResourceReleaseResponse = new(ngapType.PDUSessionResourceReleaseResponse)

	releaseResponse := successfulOutcome.Value.PDUSessionResourceReleaseResponse
	ies := &releaseResponse.ProtocolIEs

	// AMF UE NGAP ID
	amfIE := ngapType.PDUSessionResourceReleaseResponseIEs{}
	amfIE.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	amfIE.Criticality.Value = ngapType.CriticalityPresentIgnore
	amfIE.Value.Present = ngapType.PDUSessionResourceReleaseResponseIEsPresentAMFUENGAPID
	amfIE.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)
	amfIE.Value.AMFUENGAPID.Value = opts.AMFUENGAPID
	ies.List = append(ies.List, amfIE)

	// RAN UE NGAP ID
	ranIE := ngapType.PDUSessionResourceReleaseResponseIEs{}
	ranIE.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ranIE.Criticality.Value = ngapType.CriticalityPresentIgnore
	ranIE.Value.Present = ngapType.PDUSessionResourceReleaseResponseIEsPresentRANUENGAPID
	ranIE.Value.RANUENGAPID = new(ngapType.RANUENGAPID)
	ranIE.Value.RANUENGAPID.Value = opts.RANUENGAPID
	ies.List = append(ies.List, ranIE)

	// PDU Session Resource Released List
	relListIE := ngapType.PDUSessionResourceReleaseResponseIEs{}
	relListIE.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceReleasedListRelRes
	relListIE.Criticality.Value = ngapType.CriticalityPresentIgnore
	relListIE.Value.Present = ngapType.PDUSessionResourceReleaseResponseIEsPresentPDUSessionResourceReleasedListRelRes
	relListIE.Value.PDUSessionResourceReleasedListRelRes = new(ngapType.PDUSessionResourceReleasedListRelRes)

	relList := relListIE.Value.PDUSessionResourceReleasedListRelRes

	for _, pduSessionID := range opts.PDUSessionIDs {
		item := ngapType.PDUSessionResourceReleasedItemRelRes{}
		item.PDUSessionID.Value = pduSessionID

		// Build an empty Release Response Transfer (success acknowledgement)
		transfer := &ngapType.PDUSessionResourceReleaseResponseTransfer{}

		transferBytes, err := aper.MarshalWithParams(transfer, "valueExt")
		if err != nil {
			continue
		}

		item.PDUSessionResourceReleaseResponseTransfer = transferBytes

		relList.List = append(relList.List, item)
	}

	ies.List = append(ies.List, relListIE)

	return pdu, nil
}
