// Package common holds tester scenario helpers shared across packages.
package common

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/ue"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

const pduSessionType = nasMessage.PDUSessionTypeIPv4

// RegisterAndPingOpts: Key/OpC/SQN/PDUSessionID default to
// scenarios.Default* when zero; IMSI and TunInterfaceName are required.
type RegisterAndPingOpts struct {
	GNB              *gnb.GnodeB
	RANUENGAPID      int64
	PDUSessionID     uint8
	IMSI             string
	Key              string
	OpC              string
	SQN              string
	TunInterfaceName string
}

// RegisterAndPing drives one UE: Initial Registration, PDU Session
// Establishment, GTP tunnel, and a short ping. Safe to call
// concurrently for distinct UEs on the same gNB.
func RegisterAndPing(ctx context.Context, opts *RegisterAndPingOpts) error {
	if opts.GNB == nil {
		return fmt.Errorf("GNB is required")
	}

	if len(opts.IMSI) < 5 {
		return fmt.Errorf("IMSI %q is too short", opts.IMSI)
	}

	if opts.TunInterfaceName == "" {
		return fmt.Errorf("TunInterfaceName is required")
	}

	key := opts.Key
	if key == "" {
		key = scenarios.DefaultKey
	}

	opc := opts.OpC
	if opc == "" {
		opc = scenarios.DefaultOPC
	}

	sqn := opts.SQN
	if sqn == "" {
		sqn = scenarios.DefaultSequenceNumber
	}

	pduID := opts.PDUSessionID
	if pduID == 0 {
		pduID = scenarios.DefaultPDUSessionID
	}

	newUE, err := ue.NewUE(&ue.UEOpts{
		GnodeB:         opts.GNB,
		PDUSessionID:   pduID,
		PDUSessionType: pduSessionType,
		Msin:           opts.IMSI[5:],
		K:              key,
		OpC:            opc,
		Amf:            scenarios.DefaultAMF,
		Sqn:            sqn,
		Mcc:            scenarios.DefaultMCC,
		Mnc:            scenarios.DefaultMNC,
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{
			ProtectionScheme: sidf.NullScheme,
			PublicKeyID:      "0",
		},
		RoutingIndicator: scenarios.DefaultRoutingIndicator,
		DNN:              scenarios.DefaultDNN,
		Sst:              scenarios.DefaultSST,
		Sd:               scenarios.DefaultSD,
		IMEISV:           scenarios.DefaultIMEISV,
		UeSecurityCapability: testutil.GetUESecurityCapability(&testutil.UeSecurityCapability{
			Integrity: testutil.IntegrityAlgorithms{
				Nia2: true,
			},
			Ciphering: testutil.CipheringAlgorithms{
				Nea0: true,
				Nea2: true,
			},
		}),
	})
	if err != nil {
		return fmt.Errorf("create UE: %w", err)
	}

	opts.GNB.AddUE(opts.RANUENGAPID, newUE)

	if _, err := procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  opts.RANUENGAPID,
		PDUSessionID: pduID,
		UE:           newUE,
	}); err != nil {
		return fmt.Errorf("initial registration: %w", err)
	}

	if _, err := newUE.WaitForPDUSession(pduID, 5*time.Second); err != nil {
		return fmt.Errorf("wait UE PDU session: %w", err)
	}

	uePduSession := newUE.GetPDUSession(pduID)
	ueIP := uePduSession.UEIP + "/16"

	gnbPDUSession, err := opts.GNB.WaitForPDUSession(opts.RANUENGAPID, int64(pduID), 5*time.Second)
	if err != nil {
		return fmt.Errorf("wait gNB PDU session: %w", err)
	}

	if _, err := opts.GNB.AddTunnel(&gnb.NewTunnelOpts{
		UEIP:             ueIP,
		UpfIP:            gnbPDUSession.UpfAddress,
		TunInterfaceName: opts.TunInterfaceName,
		ULteid:           gnbPDUSession.ULTeid,
		DLteid:           gnbPDUSession.DLTeid,
		MTU:              uePduSession.MTU,
		QFI:              uePduSession.QFI,
	}); err != nil {
		return fmt.Errorf("create GTP tunnel %q: %w", opts.TunInterfaceName, err)
	}

	// #nosec G204 -- ping is fixed; tunInterfaceName is internally derived; PingDestination is a test constant
	cmd := exec.CommandContext(ctx, "ping",
		"-I", opts.TunInterfaceName,
		scenarios.DefaultPingDestination,
		"-c", "3",
		"-W", "1",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ping via %s: %w\noutput:\n%s", opts.TunInterfaceName, err, string(out))
	}

	logger.Logger.Debug(
		"ping ok",
		zap.String("interface", opts.TunInterfaceName),
		zap.String("imsi", opts.IMSI),
	)

	return nil
}
