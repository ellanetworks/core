package gnb

import (
	"fmt"
	"io"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/ellanetworks/core/core-tester/internal/air"
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas/nasType"
	"github.com/ishidawataru/sctp"
	"go.uber.org/zap"
)

const (
	SCTPReadBufferSize = 65535
)

type GnodeB struct {
	GnbID             string
	MCC               string
	MNC               string
	SST               int32
	SD                string
	TAC               string
	DNN               string
	Name              string
	UEPool            map[int64]air.DownlinkSender // RANUENGAPID -> UE
	NGAPIDs           map[int64]int64              // RANUENGAPID -> AMFUENGAPID
	N2Conn            *sctp.SCTPConn
	N3Conn            *net.UDPConn
	tunnels           map[uint32]*Tunnel // local TEID -> Tunnel
	lastGeneratedTEID uint32
	receivedFrames    map[int]map[int][]SCTPFrame // pduType -> msgType -> frames
	mu                sync.Mutex
	N3Address         netip.Addr
	PDUSessions       map[int64]*PDUSessionInformation // RANUENGAPID -> PDUSessionInformation
}

func (g *GnodeB) StorePDUSession(ranUeId int64, pduSessionInfo *PDUSessionInformation) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.PDUSessions == nil {
		g.PDUSessions = make(map[int64]*PDUSessionInformation)
	}

	g.PDUSessions[ranUeId] = pduSessionInfo
}

func (g *GnodeB) GetPDUSession(ranUeId int64) *PDUSessionInformation {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.PDUSessions[ranUeId]
}

func (g *GnodeB) WaitForPDUSession(ranUeId int64, timeout time.Duration) (*PDUSessionInformation, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		g.mu.Lock()

		pduSession, ok := g.PDUSessions[ranUeId]
		g.mu.Unlock()

		if ok {
			return pduSession, nil
		}

		time.Sleep(1 * time.Millisecond)
	}

	return nil, fmt.Errorf("timeout waiting for PDU session for RAN UE ID %d", ranUeId)
}

func (g *GnodeB) GetAMFUENGAPID(ranUeId int64) int64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.NGAPIDs[ranUeId]
}

func (g *GnodeB) UpdateNGAPIDs(ranUeId int64, amfUeId int64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.NGAPIDs == nil {
		g.NGAPIDs = make(map[int64]int64)
	}

	g.NGAPIDs[ranUeId] = amfUeId
}

func (g *GnodeB) LoadUE(ranUeId int64) (air.DownlinkSender, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	ue, ok := g.UEPool[ranUeId]
	if !ok {
		return nil, fmt.Errorf("UE is not found in GNB UE POOL with RAN UE ID %d", ranUeId)
	}

	return ue, nil
}

func (g *GnodeB) WaitForMessage(pduType int, msgType int, timeout time.Duration) (SCTPFrame, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		g.mu.Lock()

		msgTypeMap, ok := g.receivedFrames[pduType]
		if !ok {
			g.mu.Unlock()
			time.Sleep(1 * time.Millisecond)

			continue
		}

		frames, ok := msgTypeMap[msgType]
		if !ok {
			g.mu.Unlock()
			time.Sleep(1 * time.Millisecond)

			continue
		}

		frame := frames[0]

		if len(frames) == 1 {
			delete(msgTypeMap, msgType)
		} else {
			msgTypeMap[msgType] = frames[1:]
		}

		g.receivedFrames[pduType] = msgTypeMap

		g.mu.Unlock()

		return frame, nil
	}

	return SCTPFrame{}, fmt.Errorf("timeout waiting for NGAP message %v", getMessageName(pduType, msgType))
}

type SCTPFrame struct {
	Data []byte
	Info *sctp.SndRcvInfo
}

func Start(
	GnbID string,
	MCC string,
	MNC string,
	SST int32,
	SD string,
	DNN string,
	TAC string,
	Name string,
	coreN2Address string,
	gnbN2Address string,
	gnbN3Address string,
) (*GnodeB, error) {
	rem, err := sctp.ResolveSCTPAddr("sctp", coreN2Address)
	if err != nil {
		return nil, fmt.Errorf("could not resolve Ella Core SCTP address: %w", err)
	}

	localAddr := &sctp.SCTPAddr{
		IPAddrs: []net.IPAddr{
			{IP: net.ParseIP(gnbN2Address)},
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

	var n3Conn *net.UDPConn

	var gnbN3IPAddress netip.Addr

	if gnbN3Address != "" {
		laddr := &net.UDPAddr{
			IP:   net.ParseIP(gnbN3Address),
			Port: 2152,
		}

		n3Conn, err = net.ListenUDP("udp", laddr)
		if err != nil {
			return nil, fmt.Errorf("could not listen on GTP-U UDP address %s: %v", gnbN3Address, err)
		}

		gnbN3IPAddress, err = netip.ParseAddr(gnbN3Address)
		if err != nil {
			return nil, fmt.Errorf("could not parse gNB N3 IP address: %v", err)
		}
	}

	gnodeB := &GnodeB{
		GnbID:     GnbID,
		MCC:       MCC,
		MNC:       MNC,
		SST:       SST,
		SD:        SD,
		DNN:       DNN,
		TAC:       TAC,
		Name:      Name,
		N2Conn:    n2Conn,
		N3Conn:    n3Conn,
		tunnels:   make(map[uint32]*Tunnel),
		N3Address: gnbN3IPAddress,
	}

	if n3Conn != nil {
		go gnodeB.gtpReader()
	}

	gnodeB.listenAndServe(n2Conn)

	opts := &NGSetupRequestOpts{
		GnbID: gnodeB.GnbID,
		Mcc:   gnodeB.MCC,
		Mnc:   gnodeB.MNC,
		Sst:   gnodeB.SST,
		Tac:   gnodeB.TAC,
		Name:  gnodeB.Name,
	}

	err = gnodeB.SendNGSetupRequest(opts)
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

	return gnodeB, nil
}

func (g *GnodeB) GenerateTEID() uint32 {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.lastGeneratedTEID++

	return g.lastGeneratedTEID
}

func (g *GnodeB) AddUE(ranUENGAPID int64, ue air.DownlinkSender) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.UEPool == nil {
		g.UEPool = make(map[int64]air.DownlinkSender)
	}

	g.UEPool[ranUENGAPID] = ue
}

func (g *GnodeB) listenAndServe(conn *sctp.SCTPConn) {
	go func() {
		buf := make([]byte, SCTPReadBufferSize)

		for {
			if conn == nil {
				logger.GnbLogger.Info("SCTP connection is nil, stopping listener")
				return
			}

			n, info, err := conn.SCTPRead(buf)
			if err != nil {
				if err == io.EOF {
					logger.GnbLogger.Debug("SCTP connection closed (EOF)")
				} else {
					logger.GnbLogger.Error("could not read SCTP frame", zap.Error(err))
				}

				return
			}

			if n == 0 {
				logger.GnbLogger.Info("SCTP read returned 0 bytes (connection closed?)")
				return
			}

			cp := append([]byte(nil), buf[:n]...) // copy to isolate from buffer reuse

			sctpFrame := SCTPFrame{
				Data: cp,
				Info: info,
			}

			go func(sctpFrame SCTPFrame) {
				if err := HandleFrame(g, sctpFrame); err != nil {
					logger.GnbLogger.Error("could not handle SCTP frame", zap.Error(err))
				}
			}(sctpFrame)
		}
	}()
}

func (g *GnodeB) Close() {
	if g.N2Conn != nil {
		err := g.N2Conn.Close()
		if err != nil {
			fmt.Println("could not close SCTP connection:", err)
		}
	}

	if g.N3Conn != nil {
		err := g.N3Conn.Close()
		if err != nil {
			fmt.Println("could not close GTP-U UDP connection:", err)
		}
	}
}

func (g *GnodeB) SendUplinkNAS(nasPDU []byte, amfUENGAPID int64, ranUENGAPID int64) error {
	err := g.SendUplinkNASTransport(&UplinkNasTransportOpts{
		AMFUeNgapID: amfUENGAPID,
		RANUeNgapID: ranUENGAPID,
		Mcc:         g.MCC,
		Mnc:         g.MNC,
		GnbID:       g.GnbID,
		Tac:         g.TAC,
		NasPDU:      nasPDU,
	})
	if err != nil {
		return fmt.Errorf("could not send UplinkNASTransport: %v", err)
	}

	logger.GnbLogger.Debug(
		"Sent Uplink NAS Transport",
		zap.Int64("AMF UE NGAP ID", amfUENGAPID),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
		zap.String("GNB ID", g.GnbID),
	)

	return nil
}

func (g *GnodeB) SendInitialUEMessage(nasPDU []byte, ranUENGAPID int64, guti5G *nasType.GUTI5G, cause aper.Enumerated) error {
	opts := &InitialUEMessageOpts{
		Mcc:                   g.MCC,
		Mnc:                   g.MNC,
		GnbID:                 g.GnbID,
		Tac:                   g.TAC,
		RanUENGAPID:           ranUENGAPID,
		NasPDU:                nasPDU,
		Guti5g:                guti5G,
		RRCEstablishmentCause: cause,
	}

	pdu, err := BuildInitialUEMessage(opts)
	if err != nil {
		return fmt.Errorf("couldn't build InitialUEMessage: %s", err.Error())
	}

	err = g.SendMessage(pdu, NGAPProcedureInitialUEMessage)
	if err != nil {
		return fmt.Errorf("could not send InitialUEMessage: %v", err)
	}

	logger.GnbLogger.Debug(
		"Sent Initial UE Message",
		zap.String("GNB ID", g.GnbID),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
		zap.String("MCC", g.MCC),
		zap.String("MNC", g.MNC),
		zap.String("TAC", g.TAC),
	)

	return nil
}

func isClosedErr(err error) bool {
	if err == nil {
		return false
	}

	s := err.Error()

	return strings.Contains(s, "use of closed network connection") ||
		strings.Contains(s, "file already closed")
}
