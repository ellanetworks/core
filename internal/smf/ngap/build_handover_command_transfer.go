package ngap

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func BuildHandoverCommandTransfer(teid uint32, n3IP netip.Addr) ([]byte, error) {
	teidOct := make([]byte, 4)
	binary.BigEndian.PutUint32(teidOct, teid)

	handoverCommandTransfer := ngapType.HandoverCommandTransfer{}

	handoverCommandTransfer.DLForwardingUPTNLInformation = new(ngapType.UPTransportLayerInformation)
	handoverCommandTransfer.DLForwardingUPTNLInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	handoverCommandTransfer.DLForwardingUPTNLInformation.GTPTunnel = new(ngapType.GTPTunnel)
	handoverCommandTransfer.DLForwardingUPTNLInformation.GTPTunnel.GTPTEID.Value = teidOct
	ipv4 := n3IP.As4()

	handoverCommandTransfer.DLForwardingUPTNLInformation.GTPTunnel.TransportLayerAddress.Value = aper.BitString{
		Bytes:     ipv4[:],
		BitLength: 32,
	}

	buf, err := aper.MarshalWithParams(handoverCommandTransfer, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("could not encode handover command transfer: %s", err)
	}

	return buf, nil
}
