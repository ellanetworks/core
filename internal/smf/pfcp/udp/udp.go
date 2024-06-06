package udp

import (
	"net"
	"time"

	"github.com/omec-project/pfcp"
	"github.com/omec-project/pfcp/pfcpUdp"
	"github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/logger"
)

const MaxPfcpUdpDataSize = 1024

var Server *pfcpUdp.PfcpServer

var ServerStartTime time.Time

func Run(Dispatch func(*pfcpUdp.Message)) {
	CPNodeID := context.SMF_Self().CPNodeID
	Server = pfcpUdp.NewPfcpServer(CPNodeID.ResolveNodeIdToIp().String())

	err := Server.Listen()
	if err != nil {
		logger.PfcpLog.Errorf("Failed to listen: %v", err)
	}
	logger.PfcpLog.Infof("Listen on %s", Server.Conn.LocalAddr().String())

	go func(p *pfcpUdp.PfcpServer) {
		for {
			var pfcpMessage pfcp.Message
			remoteAddr, eventData, err := p.ReadFrom(&pfcpMessage)
			if err != nil {
				if err.Error() == "Receive resend PFCP request" {
					logger.PfcpLog.Infoln(err)
				} else {
					logger.PfcpLog.Warnf("Read PFCP error: %v", err)
				}

				continue
			}

			msg := pfcpUdp.NewMessage(remoteAddr, &pfcpMessage, eventData)
			go Dispatch(&msg)
		}
	}(Server)

	ServerStartTime = time.Now()
}

func SendPfcp(msg pfcp.Message, addr *net.UDPAddr, eventData interface{}) error {
	err := Server.WriteTo(msg, addr, eventData)
	if err != nil {
		logger.PfcpLog.Errorf("Failed to send PFCP message: %v", err)
		return err
	}

	return nil
}
