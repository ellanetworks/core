package gnb

import (
	"fmt"

	"github.com/free5gc/ngap/ngapType"
)

type UplinkNasTransportOpts struct {
	AMFUeNgapID int64
	RANUeNgapID int64
	NasPDU      []byte
	Mcc         string
	Mnc         string
	GnbID       string
	Tac         string
}

func BuildUplinkNasTransport(opts *UplinkNasTransportOpts) (ngapType.NGAPPDU, error) {
	if opts == nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("UplinkNasTransportOpts is nil")
	}

	if opts.AMFUeNgapID == 0 {
		return ngapType.NGAPPDU{}, fmt.Errorf("AMF UE NGAP ID is required to build UplinkNasTransport")
	}

	if opts.RANUeNgapID == 0 {
		return ngapType.NGAPPDU{}, fmt.Errorf("RAN UE NGAP ID is required to build UplinkNasTransport")
	}

	if opts.NasPDU == nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("NAS PDU is required to build UplinkNasTransport")
	}

	if opts.Mcc == "" {
		return ngapType.NGAPPDU{}, fmt.Errorf("MCC is required to build UplinkNasTransport")
	}

	if opts.Mnc == "" {
		return ngapType.NGAPPDU{}, fmt.Errorf("MNC is required to build UplinkNasTransport")
	}

	if opts.GnbID == "" {
		return ngapType.NGAPPDU{}, fmt.Errorf("GNB ID is required to build UplinkNasTransport")
	}

	if opts.Tac == "" {
		return ngapType.NGAPPDU{}, fmt.Errorf("TAC is required to build UplinkNasTransport")
	}

	nrCellID, err := GetNRCellIdentity(opts.GnbID)
	if err != nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("could not get nrCellID: %v", err)
	}

	tac, err := GetTacInBytes(opts.Tac)
	if err != nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("could not get tac in bytes: %v", err)
	}

	plmnID := GetPLMNIdentity(opts.Mcc, opts.Mnc)

	pdu := ngapType.NGAPPDU{}
	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeUplinkNASTransport
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentIgnore

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentUplinkNASTransport
	initiatingMessage.Value.UplinkNASTransport = new(ngapType.UplinkNASTransport)

	uplinkNasTransport := initiatingMessage.Value.UplinkNASTransport
	uplinkNasTransportIEs := &uplinkNasTransport.ProtocolIEs

	// AMF UE NGAP ID
	ie := ngapType.UplinkNASTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.UplinkNASTransportIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = opts.AMFUeNgapID

	uplinkNasTransportIEs.List = append(uplinkNasTransportIEs.List, ie)

	// RAN UE NGAP ID
	ie = ngapType.UplinkNASTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.UplinkNASTransportIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = opts.RANUeNgapID

	uplinkNasTransportIEs.List = append(uplinkNasTransportIEs.List, ie)

	// NAS-PDU
	ie = ngapType.UplinkNASTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDNASPDU
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.UplinkNASTransportIEsPresentNASPDU
	ie.Value.NASPDU = new(ngapType.NASPDU)

	nASPDU := ie.Value.NASPDU
	nASPDU.Value = opts.NasPDU

	uplinkNasTransportIEs.List = append(uplinkNasTransportIEs.List, ie)

	// User Location Information
	ie = ngapType.UplinkNASTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUserLocationInformation
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.UplinkNASTransportIEsPresentUserLocationInformation
	ie.Value.UserLocationInformation = new(ngapType.UserLocationInformation)

	userLocationInformation := ie.Value.UserLocationInformation
	userLocationInformation.Present = ngapType.UserLocationInformationPresentUserLocationInformationNR
	userLocationInformation.UserLocationInformationNR = new(ngapType.UserLocationInformationNR)

	userLocationInformationNR := userLocationInformation.UserLocationInformationNR
	userLocationInformationNR.NRCGI.PLMNIdentity = plmnID
	userLocationInformationNR.NRCGI.NRCellIdentity = nrCellID

	userLocationInformationNR.TAI.PLMNIdentity = plmnID
	userLocationInformationNR.TAI.TAC.Value = tac

	uplinkNasTransportIEs.List = append(uplinkNasTransportIEs.List, ie)

	return pdu, nil
}
