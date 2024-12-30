package smf

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp"
	"github.com/ellanetworks/core/internal/smf/pfcp/message"
	"github.com/ellanetworks/core/internal/smf/pfcp/udp"
	"github.com/ellanetworks/core/internal/smf/pfcp/upf"
)

const (
	SMF_PFCP_PORT = 8805
	UPF_PFCP_PORT = 8806
)

func Start(dbInstance *db.Database) error {
	if dbInstance == nil {
		return fmt.Errorf("dbInstance is nil")
	}
	smfContext := context.SMF_Self()
	smfContext.Name = "SMF"

	smfContext.PFCPPort = SMF_PFCP_PORT
	smfContext.UpfPfcpPort = UPF_PFCP_PORT

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", "0.0.0.0", SMF_PFCP_PORT))
	if err != nil {
		logger.SmfLog.Warnf("PFCP Parse Addr Fail: %v", err)
	}

	smfContext.CPNodeID.NodeIdType = 0
	smfContext.CPNodeID.NodeIdValue = addr.IP.To4()

	smfContext.SupportedPDUSessionType = context.IPV4

	smfContext.SnssaiInfos = make([]context.SnssaiSmfInfo, 0)
	smfContext.UserPlaneInformation = &context.UserPlaneInformation{
		UPNodes:              make(map[string]*context.UPNode),
		UPF:                  nil,
		AccessNetwork:        make(map[string]*context.UPNode),
		DefaultUserPlanePath: make(map[string][]*context.UPNode),
	}

	smfContext.PodIp = os.Getenv("POD_IP")
	smfContext.DbInstance = dbInstance
	StartPfcpServer()
	metrics.RegisterSmfMetrics()
	return nil
}

func StartPfcpServer() {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		os.Exit(0)
	}()
	udp.Run(pfcp.Dispatch)
	userPlaneInformation := context.GetUserPlaneInformation()
	if userPlaneInformation.UPF != nil {
		err := message.SendPfcpAssociationSetupRequest(userPlaneInformation.UPF.NodeID, userPlaneInformation.UPF.Port)
		if err != nil {
			logger.SmfLog.Warnf("Failed to send PFCP Association Setup Request to UPF: %+v", err)
			return
		}
	}
	go upf.InitPfcpHeartbeatRequest(userPlaneInformation)
	go upf.ProbeInactiveUpfs(userPlaneInformation)
}
