// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/tester/logger"
	"go.uber.org/zap"
)

type NGAPProcedure string

const (
	// Non-UE associated NGAP procedures
	NGAPProcedureNGSetupRequest         NGAPProcedure = "NGSetupRequest"
	NGAPProcedureNGReset                NGAPProcedure = "NGReset"
	NGAPProcedureRANConfigurationUpdate NGAPProcedure = "RANConfigurationUpdate"
	NGAPProcedureAMFConfigUpdateAck     NGAPProcedure = "AMFConfigurationUpdateAcknowledge"

	// UE-associated NGAP procedures
	NGAPProcedureInitialUEMessage                  NGAPProcedure = "InitialUEMessage"
	NGAPProcedureUplinkNASTransport                NGAPProcedure = "UplinkNASTransport"
	NGAPProcedureInitialContextSetupResponse       NGAPProcedure = "InitialContextSetupResponse"
	NGAPProcedurePDUSessionResourceSetupResponse   NGAPProcedure = "PDUSessionResourceSetupResponse"
	NGAPProcedurePDUSessionResourceModifyResponse  NGAPProcedure = "PDUSessionResourceModifyResponse"
	NGAPProcedurePDUSessionResourceReleaseResponse NGAPProcedure = "PDUSessionResourceReleaseResponse"
	NGAPProcedureUEContextReleaseComplete          NGAPProcedure = "UEContextReleaseComplete"
	NGAPProcedureUEContextReleaseRequest           NGAPProcedure = "UEContextReleaseRequest"
	NGAPProcedurePathSwitchRequest                 NGAPProcedure = "PathSwitchRequest"
	NGAPProcedureHandoverRequired                  NGAPProcedure = "HandoverRequired"
	NGAPProcedureHandoverRequestAcknowledge        NGAPProcedure = "HandoverRequestAcknowledge"
	NGAPProcedureHandoverNotify                    NGAPProcedure = "HandoverNotify"
	NGAPProcedureHandoverFailure                   NGAPProcedure = "HandoverFailure"
	NGAPProcedureHandoverCancel                    NGAPProcedure = "HandoverCancel"
	NGAPProcedureUplinkRANStatusTransfer           NGAPProcedure = "UplinkRANStatusTransfer"
	NGAPProcedureUplinkNRPPaTransport              NGAPProcedure = "UplinkNRPPaTransport"
	NGAPProcedureUERadioCapabilityInfoIndication   NGAPProcedure = "UERadioCapabilityInfoIndication"
)

func getSCTPStreamID(msgType NGAPProcedure) (uint16, error) {
	switch msgType {
	case NGAPProcedureNGSetupRequest, NGAPProcedureNGReset, NGAPProcedureRANConfigurationUpdate,
		NGAPProcedureAMFConfigUpdateAck:
		return 0, nil

	case NGAPProcedureInitialUEMessage, NGAPProcedureUplinkNASTransport,
		NGAPProcedureInitialContextSetupResponse, NGAPProcedurePDUSessionResourceSetupResponse,
		NGAPProcedurePDUSessionResourceModifyResponse, NGAPProcedurePDUSessionResourceReleaseResponse,
		NGAPProcedureUEContextReleaseComplete, NGAPProcedureUEContextReleaseRequest,
		NGAPProcedurePathSwitchRequest,
		NGAPProcedureHandoverRequired, NGAPProcedureHandoverRequestAcknowledge,
		NGAPProcedureHandoverNotify, NGAPProcedureHandoverFailure,
		NGAPProcedureHandoverCancel, NGAPProcedureUplinkRANStatusTransfer,
		NGAPProcedureUplinkNRPPaTransport,
		NGAPProcedureUERadioCapabilityInfoIndication:
		return 1, nil
	default:
		return 0, fmt.Errorf("NGAP message type (%s) not supported", msgType)
	}
}

func (g *GnodeB) SendNGSetupRequest(opts *NGSetupRequestOpts) error {
	pkt, err := BuildNGSetupRequest(opts)
	if err != nil {
		return fmt.Errorf("couldn't build NGSetupRequest: %s", err.Error())
	}

	return g.SendToRan(pkt, NGAPProcedureNGSetupRequest)
}

func (g *GnodeB) SendRANConfigurationUpdate(opts *RANConfigurationUpdateOpts) error {
	pkt, err := BuildRANConfigurationUpdate(opts)
	if err != nil {
		return fmt.Errorf("couldn't build RANConfigurationUpdate: %s", err.Error())
	}

	return g.SendToRan(pkt, NGAPProcedureRANConfigurationUpdate)
}

func (g *GnodeB) SendNGReset(opts *NGResetOpts) error {
	pkt, err := BuildNGReset(opts)
	if err != nil {
		return fmt.Errorf("couldn't build NGReset: %s", err.Error())
	}

	return g.SendToRan(pkt, NGAPProcedureNGReset)
}

func (g *GnodeB) SendUEContextReleaseRequest(opts *UEContextReleaseRequestOpts) error {
	pkt, err := BuildUEContextReleaseRequest(opts)
	if err != nil {
		return fmt.Errorf("couldn't build UEContextReleaseRequest: %s", err.Error())
	}

	if err := g.SendToRan(pkt, NGAPProcedureUEContextReleaseRequest); err != nil {
		return fmt.Errorf("couldn't send UEContextReleaseRequest: %s", err.Error())
	}

	logger.GnbLogger.Debug("Sent UE Context Release Request",
		zap.Int64("RAN UE NGAP ID", opts.RANUENGAPID),
		zap.Int64("AMF UE NGAP ID", opts.AMFUENGAPID),
	)

	return nil
}

func (g *GnodeB) SendUplinkNASTransport(opts *UplinkNasTransportOpts) error {
	pdu, err := BuildUplinkNasTransport(opts)
	if err != nil {
		return fmt.Errorf("couldn't build UplinkNasTransport: %s", err.Error())
	}

	return g.SendMessage(pdu, NGAPProcedureUplinkNASTransport)
}

func (g *GnodeB) SendInitialContextSetupResponse(opts *InitialContextSetupResponseOpts) error {
	pkt, err := BuildInitialContextSetupResponse(opts)
	if err != nil {
		return fmt.Errorf("couldn't build InitialContextSetupResponse: %s", err.Error())
	}

	return g.SendToRan(pkt, NGAPProcedureInitialContextSetupResponse)
}

func (g *GnodeB) SendPDUSessionResourceSetupResponse(opts *PDUSessionResourceSetupResponseOpts) error {
	pdu, err := BuildPDUSessionResourceSetupResponse(opts)
	if err != nil {
		return fmt.Errorf("couldn't build PDUSessionResourceSetupResponse: %s", err.Error())
	}

	return g.SendMessage(pdu, NGAPProcedurePDUSessionResourceSetupResponse)
}

func (g *GnodeB) SendPDUSessionResourceModifyResponse(opts *PDUSessionResourceModifyResponseOpts) error {
	pdu, err := BuildPDUSessionResourceModifyResponse(opts)
	if err != nil {
		return fmt.Errorf("couldn't build PDUSessionResourceModifyResponse: %s", err.Error())
	}

	return g.SendMessage(pdu, NGAPProcedurePDUSessionResourceModifyResponse)
}

func (g *GnodeB) SendPDUSessionResourceReleaseResponse(opts *PDUSessionResourceReleaseResponseOpts) error {
	pdu, err := BuildPDUSessionResourceReleaseResponse(opts)
	if err != nil {
		return fmt.Errorf("couldn't build PDUSessionResourceReleaseResponse: %s", err.Error())
	}

	return g.SendMessage(pdu, NGAPProcedurePDUSessionResourceReleaseResponse)
}

func (g *GnodeB) SendPathSwitchRequest(opts *PathSwitchRequestOpts) error {
	opts.Mcc = g.MCC
	opts.Mnc = g.MNC
	opts.Tac = g.TAC
	opts.GnbID = g.GnbID

	pdu, err := BuildPathSwitchRequest(opts)
	if err != nil {
		return fmt.Errorf("couldn't build PathSwitchRequest: %s", err.Error())
	}

	err = g.SendMessage(pdu, NGAPProcedurePathSwitchRequest)
	if err != nil {
		return fmt.Errorf("couldn't send PathSwitchRequest: %s", err.Error())
	}

	logger.GnbLogger.Debug("Sent Path Switch Request",
		zap.Int64("RAN UE NGAP ID", opts.RANUENGAPID),
		zap.Int64("Source AMF UE NGAP ID", opts.SourceAMFUENGAPID),
	)

	return nil
}

func (g *GnodeB) SendUERadioCapabilityInfoIndication(opts *UERadioCapabilityInfoIndicationOpts) error {
	pkt, err := BuildUERadioCapabilityInfoIndication(opts)
	if err != nil {
		return fmt.Errorf("couldn't build UERadioCapabilityInfoIndication: %s", err.Error())
	}

	if err := g.SendToRan(pkt, NGAPProcedureUERadioCapabilityInfoIndication); err != nil {
		return fmt.Errorf("couldn't send UERadioCapabilityInfoIndication: %s", err.Error())
	}

	return nil
}

func (g *GnodeB) SendUEContextReleaseComplete(opts *UEContextReleaseCompleteOpts) error {
	pkt, err := BuildUEContextReleaseComplete(opts)
	if err != nil {
		return fmt.Errorf("couldn't build UEContextReleaseComplete: %s", err.Error())
	}

	if err := g.SendToRan(pkt, NGAPProcedureUEContextReleaseComplete); err != nil {
		return fmt.Errorf("couldn't send UEContextReleaseComplete: %s", err.Error())
	}

	return nil
}

// SendMessage writes an encoded NGAP PDU on the SCTP stream its procedure uses.
// internal/tester/s1enb sends S1AP the same way.
func (g *GnodeB) SendMessage(pdu []byte, procedure NGAPProcedure) error {
	if err := g.SendToRan(pdu, procedure); err != nil {
		return fmt.Errorf("couldn't send packet to ran: %w", err)
	}

	return nil
}

func (g *GnodeB) SendToRan(packet []byte, msgType NGAPProcedure) error {
	g.n2Mu.RLock()

	var conn *sctp.SCTPConn

	if g.n2Active >= 0 && g.n2Active < len(g.n2Peers) {
		conn = g.n2Peers[g.n2Active].conn
	}

	g.n2Mu.RUnlock()

	if conn == nil {
		return ErrNoActivePeer
	}

	return writeToConn(conn, packet, msgType)
}

// writeToConn writes packet to conn on the SCTP stream for msgType. The locked
// startup path (n2DialAndActivateLocked) holds g.n2Mu and so cannot route
// through SendToRan.
func writeToConn(conn *sctp.SCTPConn, packet []byte, msgType NGAPProcedure) error {
	if conn == nil {
		return fmt.Errorf("ran conn is nil")
	}

	sid, err := getSCTPStreamID(msgType)
	if err != nil {
		return fmt.Errorf("could not determine SCTP stream ID from NGAP message type (%s): %s", msgType, err.Error())
	}

	if len(packet) == 0 {
		return fmt.Errorf("packet len is 0")
	}

	info := sctp.SndRcvInfo{
		Stream: sid,
		PPID:   sctp.PPIDWireOrder(ngapPPID),
	}

	if _, err := conn.WriteMsg(packet, &info); err != nil {
		return fmt.Errorf("send write to sctp connection: %s", err.Error())
	}

	return nil
}

func (g *GnodeB) SendHandoverRequired(opts *HandoverRequiredOpts) error {
	opts.TargetMcc = firstNonEmpty(opts.TargetMcc, g.MCC)
	opts.TargetMnc = firstNonEmpty(opts.TargetMnc, g.MNC)
	opts.TargetTac = firstNonEmpty(opts.TargetTac, g.TAC)

	pkt, err := BuildHandoverRequired(opts)
	if err != nil {
		return fmt.Errorf("couldn't build HandoverRequired: %s", err.Error())
	}

	if err := g.SendToRan(pkt, NGAPProcedureHandoverRequired); err != nil {
		return fmt.Errorf("couldn't send HandoverRequired: %s", err.Error())
	}

	logger.GnbLogger.Debug("Sent Handover Required",
		zap.Int64("RAN UE NGAP ID", opts.RANUENGAPID),
		zap.Int64("AMF UE NGAP ID", opts.AMFUENGAPID),
	)

	return nil
}

func (g *GnodeB) SendHandoverRequestAcknowledge(opts *HandoverRequestAcknowledgeOpts) error {
	pkt, err := BuildHandoverRequestAcknowledge(opts)
	if err != nil {
		return fmt.Errorf("couldn't build HandoverRequestAcknowledge: %s", err.Error())
	}

	if err := g.SendToRan(pkt, NGAPProcedureHandoverRequestAcknowledge); err != nil {
		return fmt.Errorf("couldn't send HandoverRequestAcknowledge: %s", err.Error())
	}

	logger.GnbLogger.Debug("Sent Handover Request Acknowledge",
		zap.Int64("RAN UE NGAP ID", opts.RANUENGAPID),
		zap.Int64("AMF UE NGAP ID", opts.AMFUENGAPID),
	)

	return nil
}

func (g *GnodeB) SendHandoverNotify(opts *HandoverNotifyOpts) error {
	opts.Mcc = firstNonEmpty(opts.Mcc, g.MCC)
	opts.Mnc = firstNonEmpty(opts.Mnc, g.MNC)
	opts.Tac = firstNonEmpty(opts.Tac, g.TAC)
	opts.GnbID = firstNonEmpty(opts.GnbID, g.GnbID)

	pdu, err := BuildHandoverNotify(opts)
	if err != nil {
		return fmt.Errorf("couldn't build HandoverNotify: %s", err.Error())
	}

	err = g.SendMessage(pdu, NGAPProcedureHandoverNotify)
	if err != nil {
		return fmt.Errorf("couldn't send HandoverNotify: %s", err.Error())
	}

	logger.GnbLogger.Debug("Sent Handover Notify",
		zap.Int64("RAN UE NGAP ID", opts.RANUENGAPID),
		zap.Int64("AMF UE NGAP ID", opts.AMFUENGAPID),
	)

	return nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}

	return b
}
