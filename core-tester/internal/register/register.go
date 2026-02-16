package register

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/core-tester/internal/gnb"
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils/procedure"
	"github.com/ellanetworks/core/core-tester/internal/ue"
	"github.com/ellanetworks/core/core-tester/internal/ue/sidf"
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
	PolicyName        string
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
	gNodeB, err := gnb.Start(
		GNBID,
		cfg.MCC,
		cfg.MNC,
		cfg.SST,
		cfg.SD,
		cfg.DNN,
		cfg.TAC,
		"Ella-Core-Tester",
		cfg.EllaCoreN2Address,
		cfg.GnbN2Address,
		cfg.GnbN3Address,
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

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID: RANUENGAPID,
		UE:          newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration procedure failed: %v", err)
	}

	logger.Logger.Info(
		"Completed Initial Registration Procedure",
		zap.String("IMSI", newUE.UeSecurity.Supi),
		zap.Int64("RAN UE NGAP ID", RANUENGAPID),
	)

	pduSession := gNodeB.GetPDUSession(RANUENGAPID)

	uePduSession := newUE.GetPDUSession()
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

	select {}
}
