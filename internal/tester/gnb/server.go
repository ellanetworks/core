package gnb

import (
	"fmt"
	"io"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/tester/air"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas/nasType"
	"github.com/ishidawataru/sctp"
	"github.com/vishvananda/netlink"
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
	Slices            []SliceOpt // Additional slices beyond SST/SD
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
	cond              *sync.Cond
	N3Address         netip.Addr
	PDUSessions       map[int64]map[int64]*PDUSessionInformation // RANUENGAPID -> PDUSessionID -> PDUSessionInformation
	UEAmbr            map[int64]*UEAmbrInformation               // RANUENGAPID -> UE AMBR
}

func (g *GnodeB) StorePDUSession(ranUeId int64, pduSessionInfo *PDUSessionInformation) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.PDUSessions == nil {
		g.PDUSessions = make(map[int64]map[int64]*PDUSessionInformation)
	}

	if g.PDUSessions[ranUeId] == nil {
		g.PDUSessions[ranUeId] = make(map[int64]*PDUSessionInformation)
	}

	g.PDUSessions[ranUeId][pduSessionInfo.PDUSessionID] = pduSessionInfo
	g.cond.Broadcast()
}

type UEAmbrInformation struct {
	UplinkBps   int64
	DownlinkBps int64
}

func (g *GnodeB) StoreUEAmbr(ranUeId int64, ambr *UEAmbrInformation) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.UEAmbr == nil {
		g.UEAmbr = make(map[int64]*UEAmbrInformation)
	}

	g.UEAmbr[ranUeId] = ambr
}

func (g *GnodeB) GetUEAmbr(ranUeId int64) *UEAmbrInformation {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.UEAmbr == nil {
		return nil
	}

	return g.UEAmbr[ranUeId]
}

func (g *GnodeB) GetPDUSession(ranUeId int64, pduSessionID int64) *PDUSessionInformation {
	g.mu.Lock()
	defer g.mu.Unlock()

	sessions := g.PDUSessions[ranUeId]
	if sessions == nil {
		return nil
	}

	return sessions[pduSessionID]
}

// GetPDUSessions returns all PDU sessions for a given RAN UE.
func (g *GnodeB) GetPDUSessions(ranUeId int64) map[int64]*PDUSessionInformation {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.PDUSessions[ranUeId]
}

func (g *GnodeB) WaitForPDUSession(ranUeId int64, pduSessionID int64, timeout time.Duration) (*PDUSessionInformation, error) {
	deadline := time.Now().Add(timeout)

	timer := time.AfterFunc(timeout, func() {
		g.cond.Broadcast()
	})
	defer timer.Stop()

	g.mu.Lock()
	defer g.mu.Unlock()

	for {
		if sessions, ok := g.PDUSessions[ranUeId]; ok {
			if pduSession, ok := sessions[pduSessionID]; ok {
				return pduSession, nil
			}
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for PDU session %d for RAN UE ID %d", pduSessionID, ranUeId)
		}

		g.cond.Wait()
	}
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

	timer := time.AfterFunc(timeout, func() {
		g.cond.Broadcast()
	})
	defer timer.Stop()

	g.mu.Lock()
	defer g.mu.Unlock()

	for {
		msgTypeMap, ok := g.receivedFrames[pduType]
		if ok {
			frames, ok := msgTypeMap[msgType]
			if ok && len(frames) > 0 {
				frame := frames[0]

				if len(frames) == 1 {
					delete(msgTypeMap, msgType)
				} else {
					msgTypeMap[msgType] = frames[1:]
				}

				g.receivedFrames[pduType] = msgTypeMap

				return frame, nil
			}
		}

		if time.Now().After(deadline) {
			return SCTPFrame{}, fmt.Errorf("timeout waiting for NGAP message %v", getMessageName(pduType, msgType))
		}

		g.cond.Wait()
	}
}

type SCTPFrame struct {
	Data []byte
	Info *sctp.SndRcvInfo
}

func NewGnodeB(
	gnbID string,
	mcc string,
	mnc string,
	sst int32,
	sd string,
	dnn string,
	tac string,
	name string,
	n2Conn *sctp.SCTPConn,
	n3Conn *net.UDPConn,
	n3Address netip.Addr,
) *GnodeB {
	g := &GnodeB{
		GnbID:     gnbID,
		MCC:       mcc,
		MNC:       mnc,
		SST:       sst,
		SD:        sd,
		DNN:       dnn,
		TAC:       tac,
		Name:      name,
		N2Conn:    n2Conn,
		N3Conn:    n3Conn,
		tunnels:   make(map[uint32]*Tunnel),
		N3Address: n3Address,
	}
	g.cond = sync.NewCond(&g.mu)

	return g
}

type StartOpts struct {
	GnbID         string
	MCC           string
	MNC           string
	SST           int32
	SD            string
	Slices        []SliceOpt
	DNN           string
	TAC           string
	Name          string
	CoreN2Address string
	GnbN2Address  string
	GnbN3Address  string
}

func Start(opts *StartOpts) (*GnodeB, error) {
	rem, err := sctp.ResolveSCTPAddr("sctp", opts.CoreN2Address)
	if err != nil {
		return nil, fmt.Errorf("could not resolve Ella Core SCTP address: %w", err)
	}

	localAddr := &sctp.SCTPAddr{
		IPAddrs: []net.IPAddr{
			{IP: net.ParseIP(opts.GnbN2Address)},
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

	if opts.GnbN3Address != "" {
		laddr := &net.UDPAddr{
			IP:   net.ParseIP(opts.GnbN3Address),
			Port: 2152,
		}

		n3Conn, err = net.ListenUDP("udp", laddr)
		if err != nil {
			return nil, fmt.Errorf("could not listen on GTP-U UDP address %s: %v", opts.GnbN3Address, err)
		}

		gnbN3IPAddress, err = netip.ParseAddr(opts.GnbN3Address)
		if err != nil {
			return nil, fmt.Errorf("could not parse gNB N3 IP address: %v", err)
		}
	}

	gnodeB := &GnodeB{
		GnbID:     opts.GnbID,
		MCC:       opts.MCC,
		MNC:       opts.MNC,
		SST:       opts.SST,
		SD:        opts.SD,
		Slices:    opts.Slices,
		DNN:       opts.DNN,
		TAC:       opts.TAC,
		Name:      opts.Name,
		N2Conn:    n2Conn,
		N3Conn:    n3Conn,
		tunnels:   make(map[uint32]*Tunnel),
		N3Address: gnbN3IPAddress,
	}
	gnodeB.cond = sync.NewCond(&gnodeB.mu)

	if n3Conn != nil {
		go gnodeB.GTPReader()
	}

	gnodeB.ListenAndServe(n2Conn)

	ngSetupOpts := &NGSetupRequestOpts{
		GnbID:  gnodeB.GnbID,
		Mcc:    gnodeB.MCC,
		Mnc:    gnodeB.MNC,
		Sst:    gnodeB.SST,
		Tac:    gnodeB.TAC,
		Name:   gnodeB.Name,
		Slices: gnodeB.Slices,
	}

	err = gnodeB.SendNGSetupRequest(ngSetupOpts)
	if err != nil {
		return nil, fmt.Errorf("could not send NGSetupRequest: %v", err)
	}

	logger.GnbLogger.Debug(
		"Sent NGSetupRequest",
		zap.String("MCC", ngSetupOpts.Mcc),
		zap.String("MNC", ngSetupOpts.Mnc),
		zap.Int32("SST", ngSetupOpts.Sst),
		zap.String("TAC", ngSetupOpts.Tac),
		zap.String("Name", ngSetupOpts.Name),
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

func (g *GnodeB) ListenAndServe(conn *sctp.SCTPConn) {
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
	g.mu.Lock()

	tunnelsToClose := make(map[uint32]*Tunnel, len(g.tunnels))
	for teid, t := range g.tunnels {
		tunnelsToClose[teid] = t
	}
	g.mu.Unlock()

	for _, t := range tunnelsToClose {
		if err := t.tunIF.Close(); err != nil {
			logger.GnbLogger.Error("error closing TUN interface", zap.String("if", t.Name), zap.Error(err))
		}

		link, err := netlink.LinkByName(t.Name)
		if err == nil {
			if err = netlink.LinkDel(link); err != nil {
				logger.GnbLogger.Error("error deleting TUN interface", zap.String("if", t.Name), zap.Error(err))
			}
		}
	}

	g.mu.Lock()
	g.tunnels = make(map[uint32]*Tunnel)
	g.mu.Unlock()

	if g.N2Conn != nil {
		err := g.N2Conn.Close()
		if err != nil {
			logger.GnbLogger.Error("could not close SCTP connection", zap.Error(err))
		}
	}

	if g.N3Conn != nil {
		err := g.N3Conn.Close()
		if err != nil {
			logger.GnbLogger.Error("could not close GTP-U UDP connection", zap.Error(err))
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
