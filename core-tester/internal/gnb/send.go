package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
	"github.com/ishidawataru/sctp"
	"go.uber.org/zap"
)

type NGAPProcedure string

const (
	// Non-UE associated NGAP procedures
	NGAPProcedureNGSetupRequest NGAPProcedure = "NGSetupRequest"
	NGAPProcedureNGReset        NGAPProcedure = "NGReset"

	// UE-associated NGAP procedures
	NGAPProcedureInitialUEMessage                NGAPProcedure = "InitialUEMessage"
	NGAPProcedureUplinkNASTransport              NGAPProcedure = "UplinkNASTransport"
	NGAPProcedureInitialContextSetupResponse     NGAPProcedure = "InitialContextSetupResponse"
	NGAPProcedurePDUSessionResourceSetupResponse NGAPProcedure = "PDUSessionResourceSetupResponse"
	NGAPProcedureUEContextReleaseComplete        NGAPProcedure = "UEContextReleaseComplete"
	NGAPProcedureUEContextReleaseRequest         NGAPProcedure = "UEContextReleaseRequest"
	NGAPProcedurePathSwitchRequest               NGAPProcedure = "PathSwitchRequest"
)

func getSCTPStreamID(msgType NGAPProcedure) (uint16, error) {
	switch msgType {
	// Non-UE procedures
	case NGAPProcedureNGSetupRequest, NGAPProcedureNGReset:
		return 0, nil

	// UE-associated procedures
	case NGAPProcedureInitialUEMessage, NGAPProcedureUplinkNASTransport,
		NGAPProcedureInitialContextSetupResponse, NGAPProcedurePDUSessionResourceSetupResponse,
		NGAPProcedureUEContextReleaseComplete, NGAPProcedureUEContextReleaseRequest,
		NGAPProcedurePathSwitchRequest:
		return 1, nil
	default:
		return 0, fmt.Errorf("NGAP message type (%s) not supported", msgType)
	}
}

func (g *GnodeB) SendNGSetupRequest(opts *NGSetupRequestOpts) error {
	pdu, err := BuildNGSetupRequest(opts)
	if err != nil {
		return fmt.Errorf("couldn't build NGSetupRequest: %s", err.Error())
	}

	return g.SendMessage(pdu, NGAPProcedureNGSetupRequest)
}

func (g *GnodeB) SendNGReset(opts *NGResetOpts) error {
	pdu, err := BuildNGReset(opts)
	if err != nil {
		return fmt.Errorf("couldn't build NGReset: %s", err.Error())
	}

	return g.SendMessage(pdu, NGAPProcedureNGReset)
}

func (g *GnodeB) SendUEContextReleaseRequest(opts *UEContextReleaseRequestOpts) error {
	pdu, err := BuildUEContextReleaseRequest(opts)
	if err != nil {
		return fmt.Errorf("couldn't build UEContextReleaseRequest: %s", err.Error())
	}

	err = g.SendMessage(pdu, NGAPProcedureUEContextReleaseRequest)
	if err != nil {
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
	pdu, err := BuildInitialContextSetupResponse(opts)
	if err != nil {
		return fmt.Errorf("couldn't build InitialContextSetupResponse: %s", err.Error())
	}

	return g.SendMessage(pdu, NGAPProcedureInitialContextSetupResponse)
}

func (g *GnodeB) SendPDUSessionResourceSetupResponse(opts *PDUSessionResourceSetupResponseOpts) error {
	pdu, err := BuildPDUSessionResourceSetupResponse(opts)
	if err != nil {
		return fmt.Errorf("couldn't build PDUSessionResourceSetupResponse: %s", err.Error())
	}

	return g.SendMessage(pdu, NGAPProcedurePDUSessionResourceSetupResponse)
}

func (g *GnodeB) SendPathSwitchRequest(opts *PathSwitchRequestOpts) error {
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

func (g *GnodeB) SendUEContextReleaseComplete(opts *UEContextReleaseCompleteOpts) error {
	pdu, err := BuildUEContextReleaseComplete(opts)
	if err != nil {
		return fmt.Errorf("couldn't build UEContextReleaseComplete: %s", err.Error())
	}

	err = g.SendMessage(pdu, NGAPProcedureUEContextReleaseComplete)
	if err != nil {
		return fmt.Errorf("couldn't send UEContextReleaseComplete: %s", err.Error())
	}

	return nil
}

func (g *GnodeB) SendMessage(pdu ngapType.NGAPPDU, procedure NGAPProcedure) error {
	bytes, err := ngap.Encoder(pdu)
	if err != nil {
		return fmt.Errorf("couldn't encode message for procedure %s: %s", procedure, err.Error())
	}

	err = g.SendToRan(bytes, procedure)
	if err != nil {
		return fmt.Errorf("couldn't send packet to ran: %s", err.Error())
	}

	return nil
}

func (g *GnodeB) SendToRan(packet []byte, msgType NGAPProcedure) error {
	if g.N2Conn == nil {
		return fmt.Errorf("ran conn is nil")
	}

	if g.N2Conn.RemoteAddr() == nil {
		return fmt.Errorf("ran address is nil")
	}

	sid, err := getSCTPStreamID(msgType)
	if err != nil {
		return fmt.Errorf("could not determine SCTP stream ID from NGAP message type (%s): %s", msgType, err.Error())
	}

	defer func() {
		err := recover()
		if err != nil {
			logger.GnbLogger.Error("panic recovered", zap.Any("error", err))
		}
	}()

	if len(packet) == 0 {
		return fmt.Errorf("packet len is 0")
	}

	info := sctp.SndRcvInfo{
		Stream: sid,
		PPID:   ngap.PPID,
	}
	if _, err := g.N2Conn.SCTPWrite(packet, &info); err != nil {
		return fmt.Errorf("send write to sctp connection: %s", err.Error())
	}

	return nil
}
