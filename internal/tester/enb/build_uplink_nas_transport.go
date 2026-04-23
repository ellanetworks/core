package enb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/free5gc/ngap/ngapType"
)

type UplinkNasTransportOpts struct {
	AMFUeNgapID int64
	RANUeNgapID int64
	NasPDU      []byte
	Mcc         string
	Mnc         string
	EnbID       string
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

	if opts.EnbID == "" {
		return ngapType.NGAPPDU{}, fmt.Errorf("ENB ID is required to build UplinkNasTransport")
	}

	if opts.Tac == "" {
		return ngapType.NGAPPDU{}, fmt.Errorf("TAC is required to build UplinkNasTransport")
	}

	eutraCellID, err := GetEUTRACellIdentity(opts.EnbID)
	if err != nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("could not get eutraCellID: %v", err)
	}

	tac, err := gnb.GetTacInBytes(opts.Tac)
	if err != nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("could not get tac in bytes: %v", err)
	}

	plmnID := gnb.GetPLMNIdentity(opts.Mcc, opts.Mnc)

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

	ie := ngapType.UplinkNASTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.UplinkNASTransportIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = opts.AMFUeNgapID

	uplinkNasTransportIEs.List = append(uplinkNasTransportIEs.List, ie)

	ie = ngapType.UplinkNASTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.UplinkNASTransportIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = opts.RANUeNgapID

	uplinkNasTransportIEs.List = append(uplinkNasTransportIEs.List, ie)

	ie = ngapType.UplinkNASTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDNASPDU
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.UplinkNASTransportIEsPresentNASPDU
	ie.Value.NASPDU = new(ngapType.NASPDU)

	nASPDU := ie.Value.NASPDU
	nASPDU.Value = opts.NasPDU

	uplinkNasTransportIEs.List = append(uplinkNasTransportIEs.List, ie)

	ie = ngapType.UplinkNASTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUserLocationInformation
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.UplinkNASTransportIEsPresentUserLocationInformation
	ie.Value.UserLocationInformation = new(ngapType.UserLocationInformation)

	userLocationInformation := ie.Value.UserLocationInformation
	userLocationInformation.Present = ngapType.UserLocationInformationPresentUserLocationInformationEUTRA
	userLocationInformation.UserLocationInformationEUTRA = new(ngapType.UserLocationInformationEUTRA)

	userLocationInformationEUTRA := userLocationInformation.UserLocationInformationEUTRA
	userLocationInformationEUTRA.EUTRACGI.PLMNIdentity = plmnID
	userLocationInformationEUTRA.EUTRACGI.EUTRACellIdentity = eutraCellID

	userLocationInformationEUTRA.TAI.PLMNIdentity = plmnID
	userLocationInformationEUTRA.TAI.TAC.Value = tac

	uplinkNasTransportIEs.List = append(uplinkNasTransportIEs.List, ie)

	return pdu, nil
}
