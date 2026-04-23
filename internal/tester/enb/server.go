package enb

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas/nasType"
	"github.com/ishidawataru/sctp"
	"go.uber.org/zap"
)

type NgeNB struct {
	*gnb.GnodeB
	EnbID string
}

func Start(
	EnbID string,
	MCC string,
	MNC string,
	SST int32,
	SD string,
	DNN string,
	TAC string,
	Name string,
	coreN2Address string,
	enbN2Address string,
	enbN3Address string,
) (*NgeNB, error) {
	rem, err := sctp.ResolveSCTPAddr("sctp", coreN2Address)
	if err != nil {
		return nil, fmt.Errorf("could not resolve Ella Core SCTP address: %w", err)
	}

	localAddr := &sctp.SCTPAddr{
		IPAddrs: []net.IPAddr{
			{IP: net.ParseIP(enbN2Address)},
		},
	}

	n2Conn, err := sctp.DialSCTPExt(
		"sctp",
		localAddr,
		rem,
		sctp.InitMsg{NumOstreams: 2, MaxInstreams: 2})
	if err != nil {
		return nil, fmt.Errorf("could not dial SCTP: %w", err)
	}

	err = n2Conn.SubscribeEvents(sctp.SCTP_EVENT_DATA_IO)
	if err != nil {
		return nil, fmt.Errorf("could not subscribe SCTP events: %w", err)
	}

	var (
		n3Conn         *net.UDPConn
		enbN3IPAddress netip.Addr
	)

	if enbN3Address != "" {
		laddr := &net.UDPAddr{
			IP:   net.ParseIP(enbN3Address),
			Port: 2152,
		}

		n3Conn, err = net.ListenUDP("udp", laddr)
		if err != nil {
			return nil, fmt.Errorf("could not listen on GTP-U UDP address %s: %v", enbN3Address, err)
		}

		enbN3IPAddress, err = netip.ParseAddr(enbN3Address)
		if err != nil {
			return nil, fmt.Errorf("could not parse eNB N3 IP address: %v", err)
		}
	}

	gnodeB := gnb.NewGnodeB(
		EnbID,
		MCC,
		MNC,
		SST,
		SD,
		DNN,
		TAC,
		Name,
		n2Conn,
		n3Conn,
		enbN3IPAddress,
	)

	if n3Conn != nil {
		go gnodeB.GTPReader()
	}

	ngeNB := &NgeNB{
		GnodeB: gnodeB,
		EnbID:  EnbID,
	}

	opts := &NGSetupRequestOpts{
		EnbID: ngeNB.EnbID,
		Mcc:   ngeNB.MCC,
		Mnc:   ngeNB.MNC,
		Sst:   ngeNB.SST,
		Tac:   ngeNB.TAC,
		Name:  ngeNB.Name,
	}

	pdu, err := BuildNGSetupRequest(opts)
	if err != nil {
		return nil, fmt.Errorf("couldn't build NGSetupRequest: %v", err)
	}

	err = ngeNB.SendMessage(pdu, gnb.NGAPProcedureNGSetupRequest)
	if err != nil {
		return nil, fmt.Errorf("could not send NGSetupRequest: %v", err)
	}

	logger.GnbLogger.Debug(
		"Sent NGSetupRequest",
		zap.String("MCC", opts.Mcc),
		zap.String("MNC", opts.Mnc),
		zap.Int32("SST", opts.Sst),
		zap.String("TAC", opts.Tac),
		zap.String("Name", opts.Name),
	)

	return ngeNB, nil
}

func (n *NgeNB) SendInitialUEMessage(nasPDU []byte, ranUENGAPID int64, guti5G *nasType.GUTI5G, cause aper.Enumerated) error {
	opts := &InitialUEMessageOpts{
		Mcc:                   n.MCC,
		Mnc:                   n.MNC,
		EnbID:                 n.EnbID,
		Tac:                   n.TAC,
		RanUENGAPID:           ranUENGAPID,
		NasPDU:                nasPDU,
		Guti5g:                guti5G,
		RRCEstablishmentCause: cause,
	}

	pdu, err := BuildInitialUEMessage(opts)
	if err != nil {
		return fmt.Errorf("couldn't build InitialUEMessage: %v", err)
	}

	err = n.SendMessage(pdu, gnb.NGAPProcedureInitialUEMessage)
	if err != nil {
		return fmt.Errorf("could not send InitialUEMessage: %v", err)
	}

	logger.GnbLogger.Debug(
		"Sent Initial UE Message",
		zap.String("ENB ID", n.EnbID),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
		zap.String("MCC", n.MCC),
		zap.String("MNC", n.MNC),
		zap.String("TAC", n.TAC),
	)

	return nil
}

func (n *NgeNB) SendUplinkNAS(nasPDU []byte, amfUENGAPID int64, ranUENGAPID int64) error {
	err := n.sendUplinkNASTransport(&UplinkNasTransportOpts{
		AMFUeNgapID: amfUENGAPID,
		RANUeNgapID: ranUENGAPID,
		Mcc:         n.MCC,
		Mnc:         n.MNC,
		EnbID:       n.EnbID,
		Tac:         n.TAC,
		NasPDU:      nasPDU,
	})
	if err != nil {
		return fmt.Errorf("could not send UplinkNASTransport: %v", err)
	}

	logger.GnbLogger.Debug(
		"Sent Uplink NAS Transport",
		zap.Int64("AMF UE NGAP ID", amfUENGAPID),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
		zap.String("ENB ID", n.EnbID),
	)

	return nil
}

func (n *NgeNB) sendUplinkNASTransport(opts *UplinkNasTransportOpts) error {
	pdu, err := BuildUplinkNasTransport(opts)
	if err != nil {
		return fmt.Errorf("couldn't build UplinkNasTransport: %v", err)
	}

	return n.SendMessage(pdu, gnb.NGAPProcedureUplinkNASTransport)
}
