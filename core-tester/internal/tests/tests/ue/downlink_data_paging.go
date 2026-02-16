package ue

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"time"

	"github.com/ellanetworks/core/core-tester/internal/gnb"
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/ellanetworks/core/core-tester/internal/tests/engine"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/core"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/procedure"
	"github.com/ellanetworks/core/core-tester/internal/ue"
	"github.com/ellanetworks/core/core-tester/internal/ue/sidf"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"github.com/google/gopacket/layers"
	"github.com/tiagomdiogo/ScaGo/packet"
	"github.com/tiagomdiogo/ScaGo/supersocket"
	scago_utils "github.com/tiagomdiogo/ScaGo/utils"
	"go.uber.org/zap"
)

type DownlinkDataPaging struct{}

func (DownlinkDataPaging) Meta() engine.Meta {
	return engine.Meta{
		ID:      "ue/paging/downlink_data",
		Summary: "Downlink data triggered UE Paging procedure",
		Timeout: 10 * time.Second,
	}
}

func (t DownlinkDataPaging) Run(ctx context.Context, env engine.Env) error {
	ellaCoreEnv := core.NewEllaCoreEnv(env.EllaCoreClient, getDefaultEllaCoreConfig())

	err := ellaCoreEnv.Create(ctx)
	if err != nil {
		return fmt.Errorf("could not create EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Created EllaCore environment")

	gNodeB, err := gnb.Start(
		GNBID,
		DefaultMCC,
		DefaultMNC,
		DefaultSST,
		DefaultSD,
		DefaultDNN,
		DefaultTAC,
		"Ella-Core-Tester",
		env.Config.EllaCore.N2Address,
		env.Config.Gnb.N2Address,
		env.Config.Gnb.N3Address,
	)
	if err != nil {
		return fmt.Errorf("error starting gNB: %v", err)
	}

	defer gNodeB.Close()

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   PDUSessionID,
		PDUSessionType: PDUSessionType,
		Msin:           DefaultIMSI[5:],
		K:              DefaultKey,
		OpC:            DefaultOPC,
		Amf:            "80000000000000000000000000000000",
		Sqn:            DefaultSequenceNumber,
		Mcc:            DefaultMCC,
		Mnc:            DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.NullScheme,
			PublicKeyID:      "0",
		},
		RoutingIndicator: "0000",
		DNN:              DefaultDNN,
		Sst:              DefaultSST,
		Sd:               DefaultSD,
		IMEISV:           "3569380356438091",
		UeSecurityCapability: utils.GetUESecurityCapability(&utils.UeSecurityCapability{
			Integrity: utils.IntegrityAlgorithms{
				Nia2: true,
			},
			Ciphering: utils.CipheringAlgorithms{
				Nea0: true,
				Nea2: true,
			},
		}),
	})
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(RANUENGAPID, newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID: RANUENGAPID,
		UE:          newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	logger.Logger.Debug(
		"Completed Initial Registration Procedure",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.Int64("RAN UE NGAP ID", RANUENGAPID),
		zap.Int64("AMF UE NGAP ID", gNodeB.GetAMFUENGAPID(RANUENGAPID)),
	)

	uePduSession := newUE.GetPDUSession()
	ueIP := uePduSession.UEIP + "/16"

	gnbPDUSession := gNodeB.GetPDUSession(RANUENGAPID)

	tunInterfaceName := GTPInterfaceNamePrefix + "paging0"

	_, err = gNodeB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP,
		UpfIP:            gnbPDUSession.UpfAddress,
		TunInterfaceName: tunInterfaceName,
		ULteid:           gnbPDUSession.ULTeid,
		DLteid:           gnbPDUSession.DLTeid,
		MTU:              uePduSession.MTU,
		QFI:              uePduSession.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create GTP tunnel (name: %s, DL TEID: %d): %v", tunInterfaceName, gnbPDUSession.DLTeid, err)
	}

	logger.GnbLogger.Debug(
		"Created GTP Tunnel for PDU Session",
		zap.String("Interface", tunInterfaceName),
		zap.String("UE IP", ueIP),
		zap.String("UPF IP", gnbPDUSession.UpfAddress),
		zap.Uint32("UL TEID", gnbPDUSession.ULTeid),
		zap.Uint32("DL TEID", gnbPDUSession.DLTeid),
	)

	cmd := exec.CommandContext(context.TODO(), "ping", "-I", tunInterfaceName, env.Config.PingDestination, "-c", "3", "-W", "1")

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("could not ping destination %s: %v", env.Config.PingDestination, err)
	}

	logger.Logger.Debug(
		"Ping successful",
		zap.String("interface", tunInterfaceName),
		zap.String("destination", env.Config.PingDestination),
	)

	pduSessionStatus := [16]bool{}
	pduSessionStatus[PDUSessionID] = true

	err = procedure.UEContextRelease(&procedure.UEContextReleaseOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(RANUENGAPID),
		RANUENGAPID:   RANUENGAPID,
		GnodeB:        gNodeB,
		UE:            newUE,
		PDUSessionIDs: pduSessionStatus,
	})
	if err != nil {
		return fmt.Errorf("UEContextReleaseProcedure failed: %v", err)
	}

	err = gNodeB.CloseTunnel(gnbPDUSession.DLTeid)
	if err != nil {
		return fmt.Errorf("could not close GTP tunnel (name: %s, DL TEID: %d): %v", tunInterfaceName, gnbPDUSession.DLTeid, err)
	}

	source_ip, err := net.ResolveIPAddr("ip", env.Config.Gnb.N3Address)
	if err != nil {
		return fmt.Errorf("could not parse N3 IP address: %v", err)
	}

	n3_interface, err := scago_utils.GetInterfaceByIP(source_ip.IP)
	if err != nil {
		return fmt.Errorf("could not find N3 interface from IP address: %v", err)
	}

	n3_mac := scago_utils.MacByInt(n3_interface.Name)
	eth := packet.EthernetLayer()

	if err = eth.SetSrcMAC(n3_mac); err != nil {
		return fmt.Errorf("could not set source MAC: %v", err)
	}

	if err = eth.SetDstMAC("a8:b8:e0:03:df:5c"); err != nil {
		return fmt.Errorf("could not set destination MAC: %v", err)
	}

	eth.SetEthernetType(layers.EthernetTypeIPv4)

	ip := packet.IPv4Layer()

	if err = ip.SetDstIP(newUE.GetPDUSession().UEIP); err != nil {
		return fmt.Errorf("could not set destination IP: %v", err)
	}

	if err = ip.SetSrcIP(env.Config.Gnb.N3Address); err != nil {
		return fmt.Errorf("could not set source IP: %v", err)
	}

	ip.SetProtocol(layers.IPProtocolICMPv4)

	icmp := packet.ICMPv4Layer()

	icmp.SetTypeCode(layers.ICMPv4TypeEchoRequest)
	icmp.SetID(1)

	packet, err := packet.CraftPacket(eth.Layer(), ip.Layer(), icmp.Layer())
	if err != nil {
		return fmt.Errorf("could not craft downlink packet: %v", err)
	}

	supersocket.Send(packet, n3_interface.Name)

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentInitiatingMessage, ngapType.InitiatingMessagePresentPaging, 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive expected Paging message: %v", err)
	}

	err = NWOriginatedServiceRequest(&NWOriginatedServiceRequestOpts{
		PDUSessionStatus: pduSessionStatus,
		RANUENGAPID:      RANUENGAPID,
		UE:               newUE,
	})
	if err != nil {
		return fmt.Errorf("service request procedure failed: %v", err)
	}

	gnbPDUSession = gNodeB.GetPDUSession(RANUENGAPID)

	_, err = gNodeB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP,
		UpfIP:            gnbPDUSession.UpfAddress,
		TunInterfaceName: tunInterfaceName,
		ULteid:           gnbPDUSession.ULTeid,
		DLteid:           gnbPDUSession.DLTeid,
		MTU:              uePduSession.MTU,
		QFI:              uePduSession.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create GTP tunnel (name: %s, DL TEID: %d): %v", tunInterfaceName, gnbPDUSession.DLTeid, err)
	}

	cmd = exec.CommandContext(context.TODO(), "ping", "-I", tunInterfaceName, env.Config.PingDestination, "-c", "3", "-W", "1")

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("could not ping destination %s: %v", env.Config.PingDestination, err)
	}

	logger.Logger.Debug(
		"Ping successful",
		zap.String("interface", tunInterfaceName),
		zap.String("destination", env.Config.PingDestination),
	)

	// Cleanup
	err = procedure.Deregistration(&procedure.DeregistrationOpts{
		UE:          newUE,
		AMFUENGAPID: gNodeB.GetAMFUENGAPID(RANUENGAPID),
		RANUENGAPID: RANUENGAPID,
	})
	if err != nil {
		return fmt.Errorf("DeregistrationProcedure failed: %v", err)
	}

	err = ellaCoreEnv.Delete(ctx)
	if err != nil {
		return fmt.Errorf("could not delete EllaCore environment: %v", err)
	}

	logger.Logger.Debug("Deleted EllaCore environment")

	return nil
}

type NWOriginatedServiceRequestOpts struct {
	PDUSessionStatus [16]bool
	RANUENGAPID      int64
	UE               *ue.UE
}

func NWOriginatedServiceRequest(opts *NWOriginatedServiceRequestOpts) error {
	guti := opts.UE.Get5gGuti()

	err := opts.UE.SendServiceRequest(opts.RANUENGAPID, opts.PDUSessionStatus, nasMessage.ServiceTypeMobileTerminatedServices)
	if err != nil {
		return fmt.Errorf("could not send Service Request NAS message: %v", err)
	}

	msg, err := opts.UE.WaitForNASGMMMessage(nas.MsgTypeConfigurationUpdateCommand, 1*time.Second)
	if err != nil {
		return fmt.Errorf("did not receive expected Configuration Update Command: %v", err)
	}

	if msg.ConfigurationUpdateCommand.GUTI5G == nil {
		return fmt.Errorf("missing GUTI in Configuration Update Command")
	}

	if msg.ConfigurationUpdateCommand.GUTI5G == guti {
		return fmt.Errorf("GUTI was not changed by Configuration Update Command")
	}

	if guti == opts.UE.Get5gGuti() {
		return fmt.Errorf("UE did not process GUTI change")
	}

	return nil
}
