package utils

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
)

func GetNasPduFromPduAccept(dlNas *nas.Message) (*nas.Message, error) {
	payload := dlNas.DLNASTransport.GetPayloadContainerContents()
	m := new(nas.Message)

	err := m.PlainNasDecode(&payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode NAS PDU: %v", err)
	}

	return m, nil
}

func GetNASPDUFromDownlinkNasTransport(downlinkNASTransport *ngapType.DownlinkNASTransport) *ngapType.NASPDU {
	for _, ie := range downlinkNASTransport.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDNASPDU:
			return ie.Value.NASPDU
		default:
			continue
		}
	}

	return nil
}

func GetAMFUENGAPIDFromDownlinkNASTransport(downlinkNASTransport *ngapType.DownlinkNASTransport) *ngapType.AMFUENGAPID {
	for _, ie := range downlinkNASTransport.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			return ie.Value.AMFUENGAPID
		default:
			continue
		}
	}

	return nil
}

func UEIPFromNAS(ip [12]uint8) (netip.Addr, error) {
	ueIPString := fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3])

	ueIP, err := netip.ParseAddr(ueIPString)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("could not parse UE IP: %s, %v", ueIPString, err)
	}

	return ueIP, nil
}

func MTUFromExtendProtocolConfigurationOptionsContents(pco_buf []byte) (uint16, error) {
	pco := nasConvert.NewProtocolConfigurationOptions()

	err := pco.UnMarshal(pco_buf)
	if err != nil {
		return 0, fmt.Errorf("could not decode Extended Protocol Configuration Options: %v", err)
	}

	for _, o := range pco.ProtocolOrContainerList {
		switch o.ProtocolOrContainerID {
		case nasMessage.IPv4LinkMTUDL:
			return binary.BigEndian.Uint16(o.Contents), nil
		default:
			continue
		}
	}

	return 0, nil
}

func SDFromNAS(sd [3]uint8) string {
	return fmt.Sprintf("%x%x%x", sd[0], sd[1], sd[2])
}
