package gnb

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handlePDUSessionResourceSetupRequest(gnb *GnodeB, pduSessionResourceSetupRequest *ngapType.PDUSessionResourceSetupRequest) error {
	var (
		amfueNGAPID                                  *ngapType.AMFUENGAPID
		ranueNGAPID                                  *ngapType.RANUENGAPID
		protocolIEIDPDUSessionResourceSetupListSUReq *ngapType.PDUSessionResourceSetupListSUReq
		ueAggregateMaximumBitRate                    *ngapType.UEAggregateMaximumBitRate
	)

	for _, ie := range pduSessionResourceSetupRequest.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			amfueNGAPID = ie.Value.AMFUENGAPID
		case ngapType.ProtocolIEIDRANUENGAPID:
			ranueNGAPID = ie.Value.RANUENGAPID
		case ngapType.ProtocolIEIDPDUSessionResourceSetupListSUReq:
			protocolIEIDPDUSessionResourceSetupListSUReq = ie.Value.PDUSessionResourceSetupListSUReq
		case ngapType.ProtocolIEIDUEAggregateMaximumBitRate:
			ueAggregateMaximumBitRate = ie.Value.UEAggregateMaximumBitRate
		}
	}

	if amfueNGAPID == nil {
		return fmt.Errorf("missing AMF UE NGAP ID in PDUSessionResourceSetupRequest")
	}

	if ranueNGAPID == nil {
		return fmt.Errorf("missing RAN UE NGAP ID in PDUSessionResourceSetupRequest")
	}

	if protocolIEIDPDUSessionResourceSetupListSUReq == nil {
		return fmt.Errorf("missing PDU Session Resource Setup List in PDUSessionResourceSetupRequest")
	}

	logger.GnbLogger.Debug(
		"Received PDU Session Resource Setup Request",
		zap.String("GNB ID", gnb.GnbID),
		zap.Int64("RAN UE NGAP ID", ranueNGAPID.Value),
		zap.Int64("AMF UE NGAP ID", amfueNGAPID.Value),
	)

	if ueAggregateMaximumBitRate != nil {
		gnb.StoreUEAmbr(ranueNGAPID.Value, &UEAmbrInformation{
			UplinkBps:   ueAggregateMaximumBitRate.UEAggregateMaximumBitRateUL.Value,
			DownlinkBps: ueAggregateMaximumBitRate.UEAggregateMaximumBitRateDL.Value,
		})
	}

	ue, err := gnb.LoadUE(ranueNGAPID.Value)
	if err != nil {
		return fmt.Errorf("could not load UE with RAN UE NGAP ID %d: %v", ranueNGAPID.Value, err)
	}

	for _, pduSession := range protocolIEIDPDUSessionResourceSetupListSUReq.List {
		pduSessionID := pduSession.PDUSessionID.Value

		// PDUSessionNASPDU may be omitted by some AMF implementations when there is
		// no NAS payload to deliver. Treat that as a non-fatal condition: if a
		// NAS PDU is present, deliver it to the UE; otherwise continue and try
		// to parse the PDU Session Resource Setup Request Transfer (if present)
		// to extract UPF/TEID information.
		if pduSession.PDUSessionNASPDU == nil {
			logger.GnbLogger.Debug("PDU Session Resource Setup Request contains no PDUSessionNASPDU, skipping NAS delivery", zap.Any("pduSession", pduSession))
		} else {
			err = ue.SendDownlinkNAS(pduSession.PDUSessionNASPDU.Value, amfueNGAPID.Value, ranueNGAPID.Value)
			if err != nil {
				return fmt.Errorf("HandleDownlinkNASTransport failed: %v", err)
			}
		}

		// If the transfer is missing we can't build UPF/TEID info; log and skip
		// storing PDU session information rather than failing the whole NGAP
		// handling flow.
		pduSessionInfo, err := getPDUSessionInfoFromSetupRequestTransfer(gnb, pduSession.PDUSessionResourceSetupRequestTransfer)
		if err != nil {
			logger.GnbLogger.Debug("could not validate PDU Session Resource Setup Transfer, skipping PDU session store", zap.Error(err), zap.Any("pduSession", pduSession))

			// continue processing other PDU sessions if any
			continue
		}

		pduSessionInfo.PDUSessionID = pduSessionID
		pduSessionInfo.DLTeid = gnb.GenerateTEID()

		logger.GnbLogger.Debug(
			"Parsed PDU Session Resource Setup Request Transfer",
			zap.Int64("AMF UE NGAP ID", amfueNGAPID.Value),
			zap.Int64("RAN UE NGAP ID", ranueNGAPID.Value),
			zap.Int64("PDU Session ID", pduSessionID),
			zap.Uint32("UL TEID", pduSessionInfo.ULTeid),
			zap.String("UPF Address", pduSessionInfo.UpfAddress),
			zap.Int64("QOS ID", pduSessionInfo.QosId),
			zap.Int64("5QI", pduSessionInfo.FiveQi),
			zap.Int64("Priority ARP", pduSessionInfo.PriArp),
			zap.Uint64("PDU Session Type", pduSessionInfo.PduSType),
		)

		gnb.StorePDUSession(ranueNGAPID.Value, pduSessionInfo)
	}

	if !gnb.N3Address.IsValid() {
		logger.GnbLogger.Warn("N3 address not configured, skipping PDUSessionResourceSetupResponse")

		return nil
	}

	pduSessions := [16]*PDUSessionInformation{}

	sessions := gnb.GetPDUSessions(ranueNGAPID.Value)
	for _, s := range sessions {
		if s.PDUSessionID >= 1 && s.PDUSessionID <= 15 {
			pduSessions[s.PDUSessionID] = &PDUSessionInformation{
				PDUSessionID: s.PDUSessionID,
				DLTeid:       s.DLTeid,
				N3GnbIp:      gnb.N3Address,
				QFI:          1,
			}
		}
	}

	err = gnb.SendPDUSessionResourceSetupResponse(&PDUSessionResourceSetupResponseOpts{
		AMFUENGAPID: amfueNGAPID.Value,
		RANUENGAPID: ranueNGAPID.Value,
		PDUSessions: pduSessions,
	})
	if err != nil {
		return fmt.Errorf("failed to send PDUSessionResourceSetupResponse: %v", err)
	}

	logger.GnbLogger.Debug(
		"Sent PDUSession Resource Setup Response",
		zap.String("GNB ID", gnb.GnbID),
		zap.Int64("RAN UE NGAP ID", ranueNGAPID.Value),
		zap.Int64("AMF UE NGAP ID", amfueNGAPID.Value),
	)

	return nil
}

type PDUSessionInformation struct {
	ULTeid       uint32
	DLTeid       uint32
	UpfAddress   string
	N3GnbIp      netip.Addr
	QosId        int64
	QFI          int64
	FiveQi       int64
	PriArp       int64
	PduSType     uint64
	PDUSessionID int64
}

func getPDUSessionInfoFromSetupRequestTransfer(gnb *GnodeB, transfer aper.OctetString) (*PDUSessionInformation, error) {
	if transfer == nil {
		return nil, fmt.Errorf("PDU Session Resource Setup Request Transfer is missing")
	}

	pdu := &ngapType.PDUSessionResourceSetupRequestTransfer{}

	err := aper.UnmarshalWithParams(transfer, pdu, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal Pdu Session Resource Setup Request Transfer: %v", err)
	}

	var (
		ulTeid     uint32
		upfAddress []byte
		qosId      int64
		fiveQi     int64
		priArp     int64
		pduSType   uint64
	)

	for _, ies := range pdu.ProtocolIEs.List {
		switch ies.Id.Value {
		case ngapType.ProtocolIEIDULNGUUPTNLInformation:
			ulTeid = binary.BigEndian.Uint32(ies.Value.ULNGUUPTNLInformation.GTPTunnel.GTPTEID.Value)
			upfAddress = ies.Value.ULNGUUPTNLInformation.GTPTunnel.TransportLayerAddress.Value.Bytes

		case ngapType.ProtocolIEIDQosFlowSetupRequestList:
			for _, itemsQos := range ies.Value.QosFlowSetupRequestList.List {
				qosId = itemsQos.QosFlowIdentifier.Value
				fiveQi = itemsQos.QosFlowLevelQosParameters.QosCharacteristics.NonDynamic5QI.FiveQI.Value
				priArp = itemsQos.QosFlowLevelQosParameters.AllocationAndRetentionPriority.PriorityLevelARP.Value
			}

		case ngapType.ProtocolIEIDPDUSessionAggregateMaximumBitRate:

		case ngapType.ProtocolIEIDPDUSessionType:
			pduSType = uint64(ies.Value.PDUSessionType.Value)

		case ngapType.ProtocolIEIDSecurityIndication:
		}
	}

	if gnb == nil {
		return nil, fmt.Errorf("gnb is nil, cannot determine N3 address family")
	}

	upfIp, err := ParseUPFAddress(upfAddress, gnb.N3Address)
	if err != nil {
		return nil, fmt.Errorf("could not parse UPF address: %v", err)
	}

	return &PDUSessionInformation{
		ULTeid:     ulTeid,
		UpfAddress: upfIp,
		N3GnbIp:    gnb.N3Address,
		QosId:      qosId,
		QFI:        qosId,
		FiveQi:     fiveQi,
		PriArp:     priArp,
		PduSType:   pduSType,
	}, nil
}

// ParseUPFAddress selects the UPF IP address from a 3GPP TransportLayerAddress BIT STRING
// that matches the IP family of the given gNB N3 address.
//
// The encoding per 3GPP TS 38.414 is:
//   - 4 bytes  → IPv4 only
//   - 16 bytes → IPv6 only
//   - 20 bytes → dual-stack: first 4 bytes are IPv4, next 16 bytes are IPv6
//
// Returns an error when the UPF provides no address for the requested family.
func ParseUPFAddress(upfAddressBytes []byte, n3Addr netip.Addr) (string, error) {
	if len(upfAddressBytes) == 0 {
		return "", fmt.Errorf("UPF transport layer address is empty")
	}

	if !n3Addr.IsValid() {
		return "", fmt.Errorf("gNB N3 address is not set")
	}

	var ipv4Addr, ipv6Addr netip.Addr

	switch len(upfAddressBytes) {
	case 4:
		var arr [4]byte
		copy(arr[:], upfAddressBytes)
		ipv4Addr = netip.AddrFrom4(arr)
	case 16:
		var arr [16]byte
		copy(arr[:], upfAddressBytes)
		ipv6Addr = netip.AddrFrom16(arr)
	case 20:
		var arr4 [4]byte
		copy(arr4[:], upfAddressBytes[:4])
		ipv4Addr = netip.AddrFrom4(arr4)

		var arr16 [16]byte
		copy(arr16[:], upfAddressBytes[4:])
		ipv6Addr = netip.AddrFrom16(arr16)
	default:
		return "", fmt.Errorf("unexpected UPF transport layer address length: %d bytes", len(upfAddressBytes))
	}

	if n3Addr.Is4() {
		if !ipv4Addr.IsValid() {
			return "", fmt.Errorf("gNB N3 address is IPv4 but UPF provided no IPv4 address")
		}

		return ipv4Addr.String(), nil
	}

	if n3Addr.Is6() {
		if !ipv6Addr.IsValid() {
			return "", fmt.Errorf("gNB N3 address is IPv6 but UPF provided no IPv6 address")
		}

		return ipv6Addr.String(), nil
	}

	return "", fmt.Errorf("gNB N3 address is neither IPv4 nor IPv6: %s", n3Addr)
}
