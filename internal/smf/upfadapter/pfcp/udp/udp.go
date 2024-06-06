package udp

import (
	"fmt"
	"net"
	"time"

	"upf-adapter/config"
	"upf-adapter/logger"

	"github.com/omec-project/pfcp"
	"github.com/omec-project/pfcp/pfcpType"
	"github.com/omec-project/pfcp/pfcpUdp"
)

var Server *pfcpUdp.PfcpServer

var (
	ServerStartTime time.Time
	CPNodeID        *pfcpType.NodeID
)

func init() {
	CPNodeID = &pfcpType.NodeID{NodeIdType: uint8(0), NodeIdValue: []byte(config.UpfAdapterIp)}
}

func SendPfcp(msg pfcp.Message, addr *net.UDPAddr, eventData interface{}) error {
	err := Server.WriteTo(msg, addr, eventData)
	if err != nil {
		logger.PfcpLog.Errorf("failed to send pfcp message, error [%v] ", err)

		return err
	}

	return nil
}

func Run(Dispatch func(*pfcpUdp.Message)) {
	Server = pfcpUdp.NewPfcpServer(CPNodeID.ResolveNodeIdToIp().String())

	err := Server.Listen()
	if err != nil {
		logger.AppLog.Errorf("Failed to listen: %v", err)
	}
	logger.AppLog.Debugf("Listen on %s", Server.Conn.LocalAddr().String())

	go func(p *pfcpUdp.PfcpServer) {
		for {
			var pfcpMessage pfcp.Message
			remoteAddr, eventData, err := p.ReadFrom(&pfcpMessage)
			if err != nil {
				if err.Error() == "receive resend pfcp request" {
					fmt.Println(err)
				} else {
					logger.AppLog.Errorf("read pfcp error: %v", err)
				}

				continue
			}

			msg := pfcpUdp.NewMessage(remoteAddr, &pfcpMessage, eventData)
			go Dispatch(&msg)
		}
	}(Server)

	config.UpfServerStartTime = time.Now()
}
