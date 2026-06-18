// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"github.com/free5gc/ngap/ngapType"
)

// HandoverNotifyOpts contains the parameters needed to build a
// HandoverNotify message (target gNB → AMF).
type HandoverNotifyOpts struct {
	AMFUENGAPID int64
	RANUENGAPID int64

	// Location info (auto-filled from gNB if empty).
	Mcc   string
	Mnc   string
	Tac   string
	GnbID string
}

// BuildHandoverNotify constructs an NGAP HandoverNotify PDU.
func BuildHandoverNotify(opts *HandoverNotifyOpts) (ngapType.NGAPPDU, error) {
	pdu := ngapType.NGAPPDU{}

	plmnID, err := GetMccAndMncInOctets(opts.Mcc, opts.Mnc)
	if err != nil {
		return pdu, err
	}

	plmnIdentity := GetPLMNIdentity(opts.Mcc, opts.Mnc)

	tac, err := GetTacInBytes(opts.Tac)
	if err != nil {
		return pdu, err
	}

	nrCellID, err := GetNRCellIdentity(opts.GnbID)
	if err != nil {
		return pdu, err
	}

	msg := &ngapType.HandoverNotify{}
	ies := &msg.ProtocolIEs

	// AMF UE NGAP ID
	{
		ie := ngapType.HandoverNotifyIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverNotifyIEsPresentAMFUENGAPID
		ie.Value.AMFUENGAPID = &ngapType.AMFUENGAPID{Value: opts.AMFUENGAPID}
		ies.List = append(ies.List, ie)
	}

	// RAN UE NGAP ID
	{
		ie := ngapType.HandoverNotifyIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverNotifyIEsPresentRANUENGAPID
		ie.Value.RANUENGAPID = &ngapType.RANUENGAPID{Value: opts.RANUENGAPID}
		ies.List = append(ies.List, ie)
	}

	// User Location Information
	{
		ie := ngapType.HandoverNotifyIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDUserLocationInformation
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.HandoverNotifyIEsPresentUserLocationInformation
		ie.Value.UserLocationInformation = &ngapType.UserLocationInformation{
			Present: ngapType.UserLocationInformationPresentUserLocationInformationNR,
			UserLocationInformationNR: &ngapType.UserLocationInformationNR{
				NRCGI: ngapType.NRCGI{
					PLMNIdentity:   plmnIdentity,
					NRCellIdentity: nrCellID,
				},
				TAI: ngapType.TAI{
					PLMNIdentity: ngapType.PLMNIdentity{Value: plmnID},
					TAC:          ngapType.TAC{Value: tac},
				},
			},
		}
		ies.List = append(ies.List, ie)
	}

	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)
	pdu.InitiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeHandoverNotification
	pdu.InitiatingMessage.Criticality.Value = ngapType.CriticalityPresentIgnore
	pdu.InitiatingMessage.Value.Present = ngapType.InitiatingMessagePresentHandoverNotify
	pdu.InitiatingMessage.Value.HandoverNotify = msg

	return pdu, nil
}
