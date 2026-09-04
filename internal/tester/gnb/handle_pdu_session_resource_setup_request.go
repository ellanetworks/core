// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func handlePDUSessionResourceSetupRequest(gnb *GnodeB, value []byte) error {
	req, err := ngap.ParsePDUSessionResourceSetupRequest(value)
	if err != nil {
		return fmt.Errorf("undecodable PDUSessionResourceSetupRequest: %w", err)
	}

	amfUeNgapID, ranUeNgapID := int64(req.AMFUENGAPID), int64(req.RANUENGAPID)

	logger.GnbLogger.Debug(
		"Received PDU Session Resource Setup Request",
		zap.String("GNB ID", gnb.GnbID),
		zap.Int64("RAN UE NGAP ID", ranUeNgapID),
		zap.Int64("AMF UE NGAP ID", amfUeNgapID),
	)

	if ambr := req.UEAggregateMaximumBitRate; ambr != nil {
		gnb.StoreUEAmbr(ranUeNgapID, &UEAmbrInformation{
			UplinkBps:   int64(ambr.UL),
			DownlinkBps: int64(ambr.DL),
		})
	}

	ue, err := gnb.LoadUE(ranUeNgapID)
	if err != nil {
		return fmt.Errorf("could not load UE with RAN UE NGAP ID %d: %w", ranUeNgapID, err)
	}

	if req.NASPDU != nil {
		if err := ue.SendDownlinkNAS(*req.NASPDU, amfUeNgapID, ranUeNgapID); err != nil {
			return fmt.Errorf("could not deliver NAS-PDU to UE: %w", err)
		}
	}

	for _, pduSession := range req.PDUSessionResourceSetup {
		pduSessionID := int64(pduSession.PDUSessionID)

		pduSessionInfo, err := getPDUSessionInfoFromSetupRequestTransfer(gnb, pduSession.Transfer)
		if err != nil {
			logger.GnbLogger.Debug("could not validate PDU Session Resource Setup Transfer, skipping PDU session store",
				zap.Error(err), zap.Int64("PDU Session ID", pduSessionID))
		} else {
			pduSessionInfo.PDUSessionID = pduSessionID
			pduSessionInfo.DLTEID = gnb.allocTEID()

			logger.GnbLogger.Debug(
				"Parsed PDU Session Resource Setup Request Transfer",
				zap.Int64("AMF UE NGAP ID", amfUeNgapID),
				zap.Int64("RAN UE NGAP ID", ranUeNgapID),
				zap.Int64("PDU Session ID", pduSessionID),
				zap.Uint32("UL TEID", pduSessionInfo.ULTEID),
				zap.String("UPF Address", pduSessionInfo.UpfAddress),
				zap.Int64("QOS ID", pduSessionInfo.QosId),
				zap.Int64("5QI", pduSessionInfo.FiveQi),
				zap.Int64("Priority ARP", pduSessionInfo.PriArp),
				zap.Uint64("PDU Session Type", pduSessionInfo.PduSType),
			)

			gnb.storePDUSession(ranUeNgapID, pduSessionInfo)
		}

		// Some AMF implementations omit the NAS-PDU when there is no NAS
		// payload; this is non-fatal.
		if pduSession.NASPDU == nil {
			logger.GnbLogger.Debug("PDU Session Resource Setup Request contains no NAS-PDU, skipping NAS delivery",
				zap.Int64("PDU Session ID", pduSessionID))
		} else if err := ue.SendDownlinkNAS(*pduSession.NASPDU, amfUeNgapID, ranUeNgapID); err != nil {
			return fmt.Errorf("HandleDownlinkNASTransport failed: %w", err)
		}
	}

	if !gnb.N3Address.IsValid() {
		logger.GnbLogger.Warn("N3 address not configured, skipping PDUSessionResourceSetupResponse")

		return nil
	}

	pduSessions := [16]*PDUSessionInformation{}

	for _, s := range gnb.pduSessionsFor(ranUeNgapID) {
		if s.PDUSessionID >= 1 && s.PDUSessionID <= 15 {
			pduSessions[s.PDUSessionID] = &PDUSessionInformation{
				PDUSessionID: s.PDUSessionID,
				DLTEID:       s.DLTEID,
				N3GnbIp:      gnb.N3Address,
				QFI:          1,
			}
		}
	}

	if err := gnb.SendPDUSessionResourceSetupResponse(&PDUSessionResourceSetupResponseOpts{
		AMFUENGAPID: amfUeNgapID,
		RANUENGAPID: ranUeNgapID,
		PDUSessions: pduSessions,
	}); err != nil {
		return fmt.Errorf("failed to send PDUSessionResourceSetupResponse: %w", err)
	}

	logger.GnbLogger.Debug(
		"Sent PDUSession Resource Setup Response",
		zap.String("GNB ID", gnb.GnbID),
		zap.Int64("RAN UE NGAP ID", ranUeNgapID),
		zap.Int64("AMF UE NGAP ID", amfUeNgapID),
	)

	return nil
}

type PDUSessionInformation struct {
	ULTEID       uint32
	DLTEID       uint32
	UpfAddress   string
	N3GnbIp      netip.Addr
	QosId        int64
	QFI          int64
	FiveQi       int64
	PriArp       int64
	PduSType     uint64
	PDUSessionID int64
	AmbrUplink   int64
	AmbrDownlink int64

	// generation orders stores of the same session so a procedure can tell the
	// resources its own signalling established from ones already there.
	generation uint64
}

// getPDUSessionInfoFromSetupRequestTransfer reads the uplink tunnel and QoS the
// SMF asks the NG-RAN node to set up (TS 38.413 §9.3.4.1).
func getPDUSessionInfoFromSetupRequestTransfer(gnb *GnodeB, transfer ngap.TransferContainer) (*PDUSessionInformation, error) {
	if len(transfer) == 0 {
		return nil, fmt.Errorf("PDU Session Resource Setup Request Transfer is missing")
	}

	if gnb == nil {
		return nil, fmt.Errorf("gnb is nil, cannot determine N3 address family")
	}

	t, err := ngap.ParsePDUSessionResourceSetupRequestTransfer(transfer)
	if err != nil {
		return nil, fmt.Errorf("could not parse PDU Session Resource Setup Request Transfer: %w", err)
	}

	var qosID, fiveQi, priArp int64

	for _, qos := range t.QosFlowSetupRequest {
		qosID = int64(qos.QosFlowIdentifier)

		if qos.QosFlowLevelQosParameters.QosCharacteristics.Kind == ngap.QosCharacteristicsNonDynamic5QI {
			fiveQi = int64(qos.QosFlowLevelQosParameters.QosCharacteristics.NonDynamic5QI.FiveQI)
		}

		priArp = int64(qos.QosFlowLevelQosParameters.AllocationAndRetentionPriority.PriorityLevelARP)
	}

	upfIP, err := ParseUPFAddress(t.ULNGUUPTNLInformation.GTPTunnel.TransportLayerAddress, gnb.N3Address)
	if err != nil {
		return nil, fmt.Errorf("could not parse UPF address: %w", err)
	}

	return &PDUSessionInformation{
		ULTEID:     uint32(t.ULNGUUPTNLInformation.GTPTunnel.GTPTEID),
		UpfAddress: upfIP,
		N3GnbIp:    gnb.N3Address,
		QosId:      qosID,
		QFI:        qosID,
		FiveQi:     fiveQi,
		PriArp:     priArp,
		PduSType:   uint64(t.PDUSessionType),
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
