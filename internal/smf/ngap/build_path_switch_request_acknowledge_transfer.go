package ngap

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

// TS 38.413 9.3.4.9
func BuildPathSwitchRequestAcknowledgeTransfer(teid uint32, n3IP net.IP) ([]byte, error) {
	teidOct := make([]byte, 4)
	binary.BigEndian.PutUint32(teidOct, teid)

	pathSwitchRequestAcknowledgeTransfer := ngapType.PathSwitchRequestAcknowledgeTransfer{}

	// UL NG-U UP TNL Information(optional) TS 38.413 9.3.2.2
	pathSwitchRequestAcknowledgeTransfer.ULNGUUPTNLInformation = new(ngapType.UPTransportLayerInformation)
	pathSwitchRequestAcknowledgeTransfer.ULNGUUPTNLInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	pathSwitchRequestAcknowledgeTransfer.ULNGUUPTNLInformation.GTPTunnel = new(ngapType.GTPTunnel)
	pathSwitchRequestAcknowledgeTransfer.ULNGUUPTNLInformation.GTPTunnel.GTPTEID.Value = teidOct
	pathSwitchRequestAcknowledgeTransfer.ULNGUUPTNLInformation.GTPTunnel.TransportLayerAddress.Value = aper.BitString{
		Bytes:     n3IP,
		BitLength: uint64(len(n3IP) * 8),
	}

	// Security Indication(optional) TS 38.413 9.3.1.27
	pathSwitchRequestAcknowledgeTransfer.SecurityIndication = new(ngapType.SecurityIndication)
	pathSwitchRequestAcknowledgeTransfer.SecurityIndication.IntegrityProtectionIndication.Value = ngapType.IntegrityProtectionIndicationPresentNotNeeded
	pathSwitchRequestAcknowledgeTransfer.SecurityIndication.ConfidentialityProtectionIndication.Value = ngapType.ConfidentialityProtectionIndicationPresentNotNeeded

	buf, err := aper.MarshalWithParams(pathSwitchRequestAcknowledgeTransfer, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("could not encode path switch request acknowledge transfer: %s", err)
	}

	return buf, nil
}
