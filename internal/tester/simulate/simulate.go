package simulate

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils/core"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils/procedure"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	gnbID              = "000008"
	pduSessionID       = 1
	gtpInterfacePrefix = "ellatester"
	startRANUENGAPID   = int64(1)

	defaultIPPool  = "10.45.0.0/16"
	defaultDNS     = "8.8.8.8"
	defaultMTU     = int32(1500)
	defaultBitrate = "100 Mbps"
	defaultVar5qi  = int32(9)
	defaultArp     = int32(15)
)

// SimulateConfig holds all parameters required to run the simulation.
type SimulateConfig struct {
	// NumSubscribers is the number of UEs to simulate.
	NumSubscribers int
	// MCC is the Mobile Country Code for the operator and gNB.
	MCC string
	// MNC is the Mobile Network Code for the operator and gNB.
	MNC string
	// SST is the Slice/Service Type.
	SST int32
	// SD is the Slice Differentiator.
	SD string
	// TAC is the Tracking Area Code.
	TAC string
	// DNN is the Data Network Name (APN).
	DNN string
	// GnbN2Address is the local IP address used for the gNB N2 (SCTP) interface.
	GnbN2Address string
	// GnbN3Address is the local IP address used for the gNB N3 (GTP-U/UDP) interface.
	GnbN3Address string
	// EllaCoreN2Address is the Ella Core SCTP/N2 address (host:port).
	EllaCoreN2Address string
	// StartIMSI is the first IMSI to use; subsequent UEs increment from this value.
	StartIMSI string
	// Key is the shared subscriber authentication key (hex string).
	Key string
	// OPC is the operator variant of the algorithm configuration field (hex string).
	OPC string
	// SQN is the sequence number used for AKA authentication (hex string).
	SQN string
	// PingDestination is the IP address each UE pings to generate traffic.
	PingDestination string
	// PingInterval controls how often each UE sends a ping probe.
	PingInterval time.Duration
}

// ueState holds the runtime state for a single registered UE, needed for traffic and cleanup.
type ueState struct {
	ranUENGAPID int64
	dlTeid      uint32
	ue          *ue.UE
	tunName     string
	ueIP        string
}

// Simulate provisions N subscribers in Ella Core, connects a gNB, registers all UEs with
// individual GTP tunnels, runs periodic ping traffic on each UE, and performs a clean
// shutdown on SIGINT.
func Simulate(ctx context.Context, cfg SimulateConfig, cl *client.Client) error {
	subs, err := buildSubscriberConfigs(cfg.NumSubscribers, cfg.StartIMSI, cfg.Key, cfg.OPC, cfg.SQN)
	if err != nil {
		return fmt.Errorf("could not build subscriber configs: %v", err)
	}

	ellaCoreEnv := core.NewEllaCoreEnv(cl, core.EllaCoreConfig{
		Operator: core.OperatorConfig{
			ID: core.OperatorID{
				MCC: cfg.MCC,
				MNC: cfg.MNC,
			},
			Tracking: core.OperatorTracking{
				SupportedTACs: []string{cfg.TAC},
			},
		},
		Profiles: []core.ProfileConfig{
			{
				Name:           defaultProfileName,
				UeAmbrUplink:   defaultBitrate,
				UeAmbrDownlink: defaultBitrate,
			},
		},
		Slices: []core.SliceConfig{
			{
				Name: defaultSliceName,
				SST:  cfg.SST,
				SD:   cfg.SD,
			},
		},
		DataNetworks: []core.DataNetworkConfig{
			{
				Name:   cfg.DNN,
				IPPool: defaultIPPool,
				DNS:    defaultDNS,
				Mtu:    defaultMTU,
			},
		},
		Policies: []core.PolicyConfig{
			{
				Name:                defaultPolicyName,
				ProfileName:         defaultProfileName,
				SliceName:           defaultSliceName,
				SessionAmbrUplink:   defaultBitrate,
				SessionAmbrDownlink: defaultBitrate,
				Var5qi:              defaultVar5qi,
				Arp:                 defaultArp,
				DataNetworkName:     cfg.DNN,
			},
		},
		Subscribers: subs,
	})

	if err = ellaCoreEnv.Create(ctx); err != nil {
		return fmt.Errorf("could not create EllaCore environment: %v", err)
	}

	defer func() {
		if err := ellaCoreEnv.Delete(ctx); err != nil {
			logger.Logger.Error("could not delete EllaCore environment", zap.Error(err))
		} else {
			logger.Logger.Info("deleted EllaCore environment")
		}
	}()

	logger.Logger.Info(
		"provisioned EllaCore environment",
		zap.Int("subscribers", cfg.NumSubscribers),
		zap.String("DNN", cfg.DNN),
	)

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:         gnbID,
		MCC:           cfg.MCC,
		MNC:           cfg.MNC,
		SST:           cfg.SST,
		SD:            cfg.SD,
		DNN:           cfg.DNN,
		TAC:           cfg.TAC,
		Name:          "Ella-Core-Tester",
		CoreN2Address: cfg.EllaCoreN2Address,
		GnbN2Address:  cfg.GnbN2Address,
		GnbN3Address:  cfg.GnbN3Address,
	})
	if err != nil {
		return fmt.Errorf("error starting gNB: %v", err)
	}

	defer func() {
		gNodeB.Close()
		logger.Logger.Info("closed gNodeB")
	}()

	_, err = gNodeB.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentNGSetupResponse,
		200*time.Millisecond,
	)
	if err != nil {
		return fmt.Errorf("did not receive NGSetupResponse: %v", err)
	}

	logger.Logger.Info("gNB connected to Ella Core")

	// Pre-allocate the state slice so each goroutine can write to its own index safely.
	ueStates := make([]ueState, cfg.NumSubscribers)

	eg := errgroup.Group{}

	for i := range cfg.NumSubscribers {
		eg.Go(func() error {
			ranUENGAPID := startRANUENGAPID + int64(i)
			tunName := fmt.Sprintf("%s%d", gtpInterfacePrefix, i)

			state, err := registerUE(cfg, gNodeB, subs[i], ranUENGAPID, tunName)
			if err != nil {
				return fmt.Errorf("failed to register UE %s: %v", subs[i].Imsi, err)
			}

			ueStates[i] = state

			return nil
		})

		// Basic stagger of the registrations to prevent timeouts
		// caused by many simultaneous registrations.
		if i%20 == 0 {
			time.Sleep(2 * time.Second)
		}
	}

	if err = eg.Wait(); err != nil {
		return fmt.Errorf("UE registration phase failed: %v", err)
	}

	logger.Logger.Info("all UEs registered", zap.Int("count", cfg.NumSubscribers))

	trafficCtx, cancelTraffic := context.WithCancel(ctx)
	defer cancelTraffic()

	for i, state := range ueStates {
		go runTraffic(trafficCtx, cfg, state, fmt.Sprintf("%s%d", gtpInterfacePrefix, i))
	}

	sctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	<-sctx.Done()
	logger.Logger.Info("received interrupt signal, shutting down")

	cancelTraffic()

	cleanupEg := errgroup.Group{}

	for _, state := range ueStates {
		cleanupEg.Go(func() error {
			if err := procedure.Deregistration(&procedure.DeregistrationOpts{
				UE:          state.ue,
				AMFUENGAPID: gNodeB.GetAMFUENGAPID(state.ranUENGAPID),
				RANUENGAPID: state.ranUENGAPID,
			}); err != nil {
				logger.Logger.Error(
					"deregistration failed",
					zap.String("IMSI", state.ue.UeSecurity.Supi),
					zap.Int64("RAN UE NGAP ID", state.ranUENGAPID),
					zap.Error(err),
				)
			} else {
				logger.Logger.Info(
					"deregistered UE",
					zap.String("IMSI", state.ue.UeSecurity.Supi),
					zap.Int64("RAN UE NGAP ID", state.ranUENGAPID),
				)
			}

			if err := gNodeB.CloseTunnel(state.dlTeid); err != nil {
				logger.Logger.Error(
					"could not close GTP tunnel",
					zap.String("interface", state.tunName),
					zap.Uint32("DL TEID", state.dlTeid),
					zap.Error(err),
				)
			} else {
				logger.Logger.Info("closed GTP tunnel", zap.String("interface", state.tunName))
			}

			return nil
		})
	}

	_ = cleanupEg.Wait()

	return nil
}

// registerUE creates and registers a single UE, establishes its PDU session and GTP tunnel,
// and returns the ueState needed for traffic generation and cleanup.
func registerUE(
	cfg SimulateConfig,
	gNodeB *gnb.GnodeB,
	sub core.SubscriberConfig,
	ranUENGAPID int64,
	tunName string,
) (ueState, error) {
	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   pduSessionID,
		PDUSessionType: nasMessage.PDUSessionTypeIPv4,
		Msin:           sub.Imsi[5:],
		K:              sub.Key,
		OpC:            sub.OPc,
		Amf:            "80000000000000000000000000000000",
		Sqn:            sub.SequenceNumber,
		Mcc:            cfg.MCC,
		Mnc:            cfg.MNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.NullScheme,
			PublicKeyID:      "0",
		},
		RoutingIndicator: "0000",
		DNN:              cfg.DNN,
		Sst:              cfg.SST,
		Sd:               cfg.SD,
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
		return ueState{}, fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID: ranUENGAPID,
		UE:          newUE,
	})
	if err != nil {
		return ueState{}, fmt.Errorf("initial registration procedure failed: %v", err)
	}

	uePDUSession, err := newUE.WaitForPDUSession(pduSessionID, 5*time.Second)
	if err != nil {
		return ueState{}, fmt.Errorf("timeout waiting for PDU session: %v", err)
	}

	gnbPDUSession, err := gNodeB.WaitForPDUSession(ranUENGAPID, int64(pduSessionID), 5*time.Second)
	if err != nil {
		return ueState{}, fmt.Errorf("could not get gNB PDU session for RAN UE NGAP ID %d: %v", ranUENGAPID, err)
	}

	ueIP := newUE.GetPDUSession(pduSessionID).UEIP + "/16"

	_, err = gNodeB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP,
		UpfIP:            gnbPDUSession.UpfAddress,
		TunInterfaceName: tunName,
		ULteid:           gnbPDUSession.ULTeid,
		DLteid:           gnbPDUSession.DLTeid,
		MTU:              uePDUSession.MTU,
		QFI:              newUE.GetPDUSession(pduSessionID).QFI,
	})
	if err != nil {
		return ueState{}, fmt.Errorf("could not create GTP tunnel (name: %s, DL TEID: %d): %v", tunName, gnbPDUSession.DLTeid, err)
	}

	logger.Logger.Info(
		"registered UE and established GTP tunnel",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
		zap.String("interface", tunName),
		zap.String("UE IP", ueIP),
	)

	rawUEIP := newUE.GetPDUSession(pduSessionID).UEIP

	return ueState{
		ranUENGAPID: ranUENGAPID,
		dlTeid:      gnbPDUSession.DLTeid,
		ue:          newUE,
		tunName:     tunName,
		ueIP:        rawUEIP,
	}, nil
}

// runPingProbe sends a ping probe via the TUN interface.
func runPingProbe(ctx context.Context, cfg SimulateConfig, state ueState, tunName string) {
	// #nosec G404 -- non-cryptographic: simulated traffic volume (1–100 packets)
	count := rand.IntN(100) + 1
	// #nosec G404 -- non-cryptographic: simulated payload size (0–1000 bytes)
	size := rand.IntN(1001)

	// #nosec G204 -- ping is a fixed executable; tunName is internally derived and PingDestination is test-supplied config
	cmd := exec.CommandContext(
		ctx,
		"ping",
		"-I", tunName,
		"-c", strconv.Itoa(count),
		"-s", strconv.Itoa(size),
		"-W", "2",
		cfg.PingDestination,
	)

	err := cmd.Run()
	if err != nil {
		logger.Logger.Debug(
			"ping failed",
			zap.String("IMSI", state.ue.UeSecurity.Supi),
			zap.String("interface", tunName),
			zap.String("destination", cfg.PingDestination),
			zap.Int("count", count),
			zap.Int("size", size),
			zap.Error(err),
		)
	} else {
		logger.Logger.Debug(
			"ping successful",
			zap.String("IMSI", state.ue.UeSecurity.Supi),
			zap.String("interface", tunName),
			zap.String("destination", cfg.PingDestination),
			zap.Int("count", count),
			zap.Int("size", size),
		)
	}
}

// runTCPProbe sends a TCP probe with SO_BINDTODEVICE.
func runTCPProbe(ctx context.Context, cfg SimulateConfig, state ueState) {
	// #nosec G404 -- non-cryptographic: simulated destination port
	port := rand.IntN(65534) + 1
	// #nosec G404 -- non-cryptographic: simulated payload size (0–1000 bytes)
	payloadSize := rand.IntN(1001)

	dialer := &net.Dialer{
		LocalAddr: &net.TCPAddr{
			IP:   net.ParseIP(state.ueIP),
			Port: 0,
		},
		Timeout: 2 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				_ = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, state.tunName)
			})
		},
	}

	conn, err := dialer.DialContext(ctx, "tcp", cfg.PingDestination+":"+strconv.Itoa(port))
	if err != nil {
		logger.Logger.Debug(
			"tcp probe failed",
			zap.String("IMSI", state.ue.UeSecurity.Supi),
			zap.String("interface", state.tunName),
			zap.String("destination", cfg.PingDestination),
			zap.Int("port", port),
			zap.Error(err),
		)

		return
	}

	defer func() { _ = conn.Close() }()

	// Generate random payload
	payload := make([]byte, payloadSize)
	for i := range payload {
		payload[i] = byte(rand.IntN(256)) // #nosec G404 -- non-cryptographic: simulated payload byte
	}

	_, err = conn.Write(payload)
	if err != nil {
		logger.Logger.Debug(
			"tcp write failed",
			zap.String("IMSI", state.ue.UeSecurity.Supi),
			zap.String("interface", state.tunName),
			zap.String("destination", cfg.PingDestination),
			zap.Int("port", port),
			zap.Int("payload_size", payloadSize),
			zap.Error(err),
		)
	} else {
		logger.Logger.Debug(
			"tcp probe successful",
			zap.String("IMSI", state.ue.UeSecurity.Supi),
			zap.String("interface", state.tunName),
			zap.String("destination", cfg.PingDestination),
			zap.Int("port", port),
			zap.Int("payload_size", payloadSize),
		)
	}
}

// runUDPProbe sends a UDP probe with SO_BINDTODEVICE.
func runUDPProbe(ctx context.Context, cfg SimulateConfig, state ueState) {
	// #nosec G404 -- non-cryptographic: simulated destination port
	port := rand.IntN(65534) + 1
	// #nosec G404 -- non-cryptographic: simulated payload size (0–1000 bytes)
	payloadSize := rand.IntN(1001)

	dialer := &net.Dialer{
		LocalAddr: &net.UDPAddr{
			IP:   net.ParseIP(state.ueIP),
			Port: 0,
		},
		Timeout: 2 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				_ = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, state.tunName)
			})
		},
	}

	conn, err := dialer.DialContext(ctx, "udp", cfg.PingDestination+":"+strconv.Itoa(port))
	if err != nil {
		logger.Logger.Debug(
			"udp probe failed",
			zap.String("IMSI", state.ue.UeSecurity.Supi),
			zap.String("interface", state.tunName),
			zap.String("destination", cfg.PingDestination),
			zap.Int("port", port),
			zap.Error(err),
		)

		return
	}

	defer func() { _ = conn.Close() }()

	// Generate random payload
	payload := make([]byte, payloadSize)
	for i := range payload {
		payload[i] = byte(rand.IntN(256)) // #nosec G404 -- non-cryptographic: simulated payload byte
	}

	_, err = conn.Write(payload)
	if err != nil {
		logger.Logger.Debug(
			"udp write failed",
			zap.String("IMSI", state.ue.UeSecurity.Supi),
			zap.String("interface", state.tunName),
			zap.String("destination", cfg.PingDestination),
			zap.Int("port", port),
			zap.Int("payload_size", payloadSize),
			zap.Error(err),
		)
	} else {
		logger.Logger.Debug(
			"udp probe successful",
			zap.String("IMSI", state.ue.UeSecurity.Supi),
			zap.String("interface", state.tunName),
			zap.String("destination", cfg.PingDestination),
			zap.Int("port", port),
			zap.Int("payload_size", payloadSize),
		)
	}
}

// runTraffic sends periodic traffic probes via the UE's network, rotating between ping (ICMP),
// TCP connect probes, and UDP probes. Each action uses a random destination port and payload size.
// Ping uses the GTP tunnel (TUN interface), while TCP and UDP use SO_BINDTODEVICE to bind to the UE IP.
// All failures are logged at Debug level and do not terminate the loop.
func runTraffic(ctx context.Context, cfg SimulateConfig, state ueState, tunName string) {
	logger.Logger.Info(
		"starting traffic loop",
		zap.String("IMSI", state.ue.UeSecurity.Supi),
		zap.String("interface", tunName),
		zap.String("UE IP", state.ueIP),
		zap.String("destination", cfg.PingDestination),
		zap.Duration("interval", cfg.PingInterval),
	)

	for {
		select {
		case <-ctx.Done():
			logger.Logger.Info(
				"stopping traffic loop",
				zap.String("IMSI", state.ue.UeSecurity.Supi),
				zap.String("interface", tunName),
			)

			return
		case <-time.After(cfg.PingInterval):
			// Randomly select: 0=ping, 1=tcp, 2=udp
			action := rand.IntN(3) // #nosec G404 -- non-cryptographic: probe type selection

			switch action {
			case 0:
				runPingProbe(ctx, cfg, state, tunName)
			case 1:
				runTCPProbe(ctx, cfg, state)
			case 2:
				runUDPProbe(ctx, cfg, state)
			}
		}
	}
}
