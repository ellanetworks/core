package message_test

import (
	"net"
	"testing"
	"time"

	"github.com/omec-project/pfcp/pfcpType"
	"github.com/yeastengine/ella/internal/smf/context"
	smf_pfcp "github.com/yeastengine/ella/internal/smf/pfcp"
	"github.com/yeastengine/ella/internal/smf/pfcp/message"
	"github.com/yeastengine/ella/internal/smf/pfcp/udp"
)

func TestSendHeartbeatResponse(t *testing.T) {
	context.SMF_Self().CPNodeID = pfcpType.NodeID{
		NodeIdType:  pfcpType.NodeIdTypeIpv4Address,
		NodeIdValue: net.ParseIP("127.0.0.2").To4(),
	}
	udp.Run(smf_pfcp.Dispatch)

	udp.ServerStartTime = time.Now()
	var seq uint32 = 1
	addr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 7001,
	}
	message.SendHeartbeatResponse(addr, seq)
}
