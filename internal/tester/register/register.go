package register

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils"
	"github.com/ellanetworks/core/internal/tester/tests/tests/utils/procedure"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

const (
	RANUENGAPID  = 1
	GNBID        = "000008"
	PDUSessionID = 1
)

const (
	GTPInterfaceName = "ellatester0"
	GTPUPort         = 2152
)

type RegisterConfig struct {
	IMSI              string
	Key               string
	OPC               string
	SequenceNumber    string
	ProfileName       string
	MCC               string
	MNC               string
	SST               int32
	SD                string
	TAC               string
	DNN               string
	GnbN2Address      string
	GnbN3Address      string
	EllaCoreN2Address string
}

func Register(ctx context.Context, cfg RegisterConfig) error {
	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:         GNBID,
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

	logger.Logger.Info("started gNodeB")

	defer func() {
		gNodeB.Close()
		logger.Logger.Info("closed gNodeB")
	}()

	_, err = gNodeB.WaitForMessage(ngapType.NGAPPDUPresentSuccessfulOutcome, ngapType.SuccessfulOutcomePresentNGSetupResponse, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("did not receive SCTP frame: %v", err)
	}

	logger.Logger.Info("received NGSetupResponse")

	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         gNodeB,
		PDUSessionID:   1,
		PDUSessionType: nasMessage.PDUSessionTypeIPv4,
		Msin:           cfg.IMSI[5:],
		K:              cfg.Key,
		OpC:            cfg.OPC,
		Amf:            "80000000000000000000000000000000",
		Sqn:            cfg.SequenceNumber,
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
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(RANUENGAPID, newUE)
	logger.Logger.Info("added new UE to gNodeB")

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  RANUENGAPID,
		PDUSessionID: PDUSessionID,
		UE:           newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	defer func() {
		err = procedure.Deregistration(&procedure.DeregistrationOpts{
			AMFUENGAPID: gNodeB.GetAMFUENGAPID(RANUENGAPID),
			RANUENGAPID: RANUENGAPID,
			UE:          newUE,
		})
		if err != nil {
			logger.Logger.Error("could not deregister UE", zap.Error(err))
		}

		logger.Logger.Info("deregistered UE")
	}()

	logger.Logger.Info(
		"Completed Initial Registration Procedure",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.Int64("RAN UE NGAP ID", RANUENGAPID),
	)

	pduSession := gNodeB.GetPDUSession(RANUENGAPID, int64(PDUSessionID))

	uePduSession := newUE.GetPDUSession(PDUSessionID)
	ueIP := uePduSession.UEIP + "/16"

	_, err = gNodeB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP,
		UpfIP:            pduSession.UpfAddress,
		TunInterfaceName: GTPInterfaceName,
		ULteid:           pduSession.ULTeid,
		DLteid:           pduSession.DLTeid,
		MTU:              uePduSession.MTU,
		QFI:              uePduSession.QFI,
	})
	if err != nil {
		return fmt.Errorf("could not create GTP tunnel (name: %s, DL TEID: %d): %v", GTPInterfaceName, pduSession.DLTeid, err)
	}

	defer func() {
		err = gNodeB.CloseTunnel(pduSession.DLTeid)
		if err != nil {
			logger.Logger.Error("could not close tunnel", zap.Error(err))
		}

		logger.Logger.Info("closed tunnel")
	}()

	logger.Logger.Info(
		"Created GTP tunnel",
		zap.String("interface", GTPInterfaceName),
		zap.String("UE IP", ueIP),
		zap.String("gNB IP", cfg.GnbN3Address),
		zap.String("UPF IP", pduSession.UpfAddress),
		zap.Uint32("LTEID", pduSession.ULTeid),
		zap.Uint32("RTEID", pduSession.DLTeid),
		zap.Uint16("GTPU Port", GTPUPort),
		zap.Uint16("MTU", uePduSession.MTU),
	)

	sctx, _ := signal.NotifyContext(context.Background(), os.Interrupt)
	<-sctx.Done()
	logger.Logger.Info("received interrupt signal, shutting down")

	return nil
}
