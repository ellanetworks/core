// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

type InitialUEMessageOpts struct {
	RanUENGAPID           int64
	NasPDU                []byte
	Guti5g                []byte
	Mcc                   string
	Mnc                   string
	Tac                   string
	GnbID                 string
	RRCEstablishmentCause aper.Enumerated
}

func BuildInitialUEMessage(opts *InitialUEMessageOpts) (ngapType.NGAPPDU, error) {
	if opts.Mcc == "" {
		return ngapType.NGAPPDU{}, fmt.Errorf("MCC is required to build InitialUEMessage")
	}

	if opts.Mnc == "" {
		return ngapType.NGAPPDU{}, fmt.Errorf("MNC is required to build InitialUEMessage")
	}

	if opts.Tac == "" {
		return ngapType.NGAPPDU{}, fmt.Errorf("TAC is required to build InitialUEMessage")
	}

	if opts.GnbID == "" {
		return ngapType.NGAPPDU{}, fmt.Errorf("GNB ID is required to build InitialUEMessage")
	}

	if opts.NasPDU == nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("NAS PDU is required to build InitialUEMessage")
	}

	if opts.RanUENGAPID == 0 {
		return ngapType.NGAPPDU{}, fmt.Errorf("RAN UE NGAP ID is required to build InitialUEMessage")
	}

	plmnID, err := GetMccAndMncInOctets(opts.Mcc, opts.Mnc)
	if err != nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("failed to get plmnID: %+v", err)
	}

	plmnIdentity := GetPLMNIdentity(opts.Mcc, opts.Mnc)

	tac, err := GetTacInBytes(opts.Tac)
	if err != nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("failed to get tac: %+v", err)
	}

	nrCellID, err := GetNRCellIdentity(opts.GnbID)
	if err != nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("failed to get nrCellID: %+v", err)
	}

	pdu := ngapType.NGAPPDU{}
	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeInitialUEMessage
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentIgnore

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentInitialUEMessage
	initiatingMessage.Value.InitialUEMessage = new(ngapType.InitialUEMessage)

	initialUEMessage := initiatingMessage.Value.InitialUEMessage
	initialUEMessageIEs := &initialUEMessage.ProtocolIEs

	ie := ngapType.InitialUEMessageIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.InitialUEMessageIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = opts.RanUENGAPID

	initialUEMessageIEs.List = append(initialUEMessageIEs.List, ie)

	ie = ngapType.InitialUEMessageIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDNASPDU
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.InitialUEMessageIEsPresentNASPDU
	ie.Value.NASPDU = new(ngapType.NASPDU)

	nASPDU := ie.Value.NASPDU
	nASPDU.Value = opts.NasPDU

	initialUEMessageIEs.List = append(initialUEMessageIEs.List, ie)

	ie = ngapType.InitialUEMessageIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUserLocationInformation
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.InitialUEMessageIEsPresentUserLocationInformation
	ie.Value.UserLocationInformation = new(ngapType.UserLocationInformation)

	userLocationInformation := ie.Value.UserLocationInformation
	userLocationInformation.Present = ngapType.UserLocationInformationPresentUserLocationInformationNR
	userLocationInformation.UserLocationInformationNR = new(ngapType.UserLocationInformationNR)

	userLocationInformationNR := userLocationInformation.UserLocationInformationNR
	userLocationInformationNR.NRCGI.PLMNIdentity = plmnIdentity
	userLocationInformationNR.NRCGI.NRCellIdentity = nrCellID

	userLocationInformationNR.TAI.PLMNIdentity.Value = plmnID
	userLocationInformationNR.TAI.TAC.Value = tac

	initialUEMessageIEs.List = append(initialUEMessageIEs.List, ie)

	ie = ngapType.InitialUEMessageIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRRCEstablishmentCause
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.InitialUEMessageIEsPresentRRCEstablishmentCause
	ie.Value.RRCEstablishmentCause = new(ngapType.RRCEstablishmentCause)

	rRCEstablishmentCause := ie.Value.RRCEstablishmentCause
	rRCEstablishmentCause.Value = opts.RRCEstablishmentCause

	initialUEMessageIEs.List = append(initialUEMessageIEs.List, ie)

	if len(opts.Guti5g) >= 11 {
		ie = ngapType.InitialUEMessageIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDFiveGSTMSI
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.InitialUEMessageIEsPresentFiveGSTMSI
		ie.Value.FiveGSTMSI = new(ngapType.FiveGSTMSI)

		// The AMF Set ID (10 bits) and AMF Pointer (6 bits) sit in octets 6-7 of the
		// 5G-GUTI value, and the 5G-TMSI in octets 8-11 (TS 24.501 §9.11.3.4).
		fiveGSTMSI := ie.Value.FiveGSTMSI
		fiveGSTMSI.AMFSetID.Value = aper.BitString{
			Bytes:     []byte{opts.Guti5g[5], opts.Guti5g[6]},
			BitLength: 10,
		}
		fiveGSTMSI.AMFPointer.Value = aper.BitString{
			Bytes:     []byte{(opts.Guti5g[6] & 0x3f) << 2},
			BitLength: 6,
		}
		fiveGSTMSI.FiveGTMSI.Value = append([]byte(nil), opts.Guti5g[7:11]...)

		initialUEMessageIEs.List = append(initialUEMessageIEs.List, ie)
	}

	ie = ngapType.InitialUEMessageIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUEContextRequest
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.InitialUEMessageIEsPresentUEContextRequest
	ie.Value.UEContextRequest = new(ngapType.UEContextRequest)
	ie.Value.UEContextRequest.Value = ngapType.UEContextRequestPresentRequested
	initialUEMessageIEs.List = append(initialUEMessageIEs.List, ie)

	return pdu, nil
}
