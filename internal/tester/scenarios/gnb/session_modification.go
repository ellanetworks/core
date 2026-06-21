// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/testutil/validate"
	"github.com/free5gc/nas"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const sessionModificationIMSI = "001017271246546"

type sessionModificationParams struct {
	EllaAPIAddress string
	EllaAPIToken   string
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "gnb/session-modification",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &sessionModificationParams{}
			fs.StringVar(&p.EllaAPIAddress, "ella-api-address", "", "Ella Core API address (e.g. http://10.3.0.2:5002)")
			fs.StringVar(&p.EllaAPIToken, "ella-api-token", "", "Ella Core API token")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			p := params.(*sessionModificationParams)

			return runSessionModification(ctx, env, p, sessionModificationConfig{
				ambrUplink:   "200 Mbps",
				ambrDownlink: "200 Mbps",
				var5qi:       8,
				arp:          14,
			})
		},
		Fixture: fixtureSessionModification,
	})

	scenarios.Register(scenarios.Scenario{
		Name: "gnb/session-modification-ambr-only",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &sessionModificationParams{}
			fs.StringVar(&p.EllaAPIAddress, "ella-api-address", "", "Ella Core API address")
			fs.StringVar(&p.EllaAPIToken, "ella-api-token", "", "Ella Core API token")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			p := params.(*sessionModificationParams)

			return runSessionModification(ctx, env, p, sessionModificationConfig{
				ambrUplink:   "300 Mbps",
				ambrDownlink: "400 Mbps",
			})
		},
		Fixture: fixtureSessionModification,
	})

	scenarios.Register(scenarios.Scenario{
		Name: "gnb/session-modification-qos-only",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &sessionModificationParams{}
			fs.StringVar(&p.EllaAPIAddress, "ella-api-address", "", "Ella Core API address")
			fs.StringVar(&p.EllaAPIToken, "ella-api-token", "", "Ella Core API token")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			p := params.(*sessionModificationParams)

			return runSessionModification(ctx, env, p, sessionModificationConfig{
				var5qi: 8,
				arp:    14,
			})
		},
		Fixture: fixtureSessionModification,
	})
}

func fixtureSessionModification(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

type sessionModificationConfig struct {
	ambrUplink   string
	ambrDownlink string
	var5qi       int32
	arp          int32
}

func runSessionModification(ctx context.Context, env scenarios.Env, p *sessionModificationParams, cfg sessionModificationConfig) error {
	if p.EllaAPIAddress == "" {
		return fmt.Errorf("--ella-api-address is required for session modification scenario")
	}

	if p.EllaAPIToken == "" {
		return fmt.Errorf("--ella-api-token is required for session modification scenario")
	}

	// Build Ella API client.
	cl, err := client.New(&client.Config{BaseURL: p.EllaAPIAddress})
	if err != nil {
		return fmt.Errorf("failed to create Ella client: %v", err)
	}

	cl.SetToken(p.EllaAPIToken)

	// Start gNB.
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	// Create UE and register.
	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	sub := subscriber{
		IMSI:           sessionModificationIMSI,
		Key:            scenarios.DefaultKey,
		OPc:            scenarios.DefaultOPC,
		SequenceNumber: scenarios.DefaultSequenceNumber,
	}

	newUE, err := newDefaultUE(gNodeB, sub.IMSI[5:], sub.Key, sub.OPc, sub.SequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration failed: %v", err)
	}

	logger.Logger.Info(
		"PDU session established, proceeding to modify policy",
		zap.String("IMSI", sub.IMSI),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
	)

	// Validate initial QoS.
	gnbPDUSession := gNodeB.GetPDUSession(ranUENGAPID, int64(scenarios.DefaultPDUSessionID))
	if gnbPDUSession == nil {
		return fmt.Errorf("PDU session not stored on gNB after initial registration")
	}

	err = validate.PDUSessionInformation(gnbPDUSession, &validate.ExpectedPDUSessionInformation{
		FiveQi: 9,
		PriArp: 15,
		QFI:    1,
	})
	if err != nil {
		return fmt.Errorf("initial QoS validation failed: %v", err)
	}

	// --- Phase 2: Modify the policy via Ella API ---
	// Build update options from config, filling in defaults for unchanged fields.
	apiAmbrUplink := cfg.ambrUplink
	if apiAmbrUplink == "" {
		apiAmbrUplink = scenarios.DefaultPolicySessionAmbrUplink
	}

	apiAmbrDownlink := cfg.ambrDownlink
	if apiAmbrDownlink == "" {
		apiAmbrDownlink = scenarios.DefaultPolicySessionAmbrDownlink
	}

	apiVar5qi := cfg.var5qi
	if apiVar5qi == 0 {
		apiVar5qi = 9
	}

	apiArp := cfg.arp
	if apiArp == 0 {
		apiArp = 15
	}

	updateOpts := &client.UpdatePolicyOptions{
		ProfileName:         scenarios.DefaultProfileName,
		SliceName:           scenarios.DefaultSliceName,
		DataNetworkName:     scenarios.DefaultDNN,
		SessionAmbrUplink:   apiAmbrUplink,
		SessionAmbrDownlink: apiAmbrDownlink,
		Var5qi:              apiVar5qi,
		Arp:                 apiArp,
	}

	err = cl.UpdatePolicy(ctx, scenarios.DefaultPolicyName, updateOpts)
	if err != nil {
		return fmt.Errorf("failed to update policy: %v", err)
	}

	// Restore original policy on exit so subsequent scenarios see default values.
	defer func() {
		_ = cl.UpdatePolicy(ctx, scenarios.DefaultPolicyName, &client.UpdatePolicyOptions{
			ProfileName:         scenarios.DefaultProfileName,
			SliceName:           scenarios.DefaultSliceName,
			DataNetworkName:     scenarios.DefaultDNN,
			SessionAmbrUplink:   scenarios.DefaultPolicySessionAmbrUplink,
			SessionAmbrDownlink: scenarios.DefaultPolicySessionAmbrDownlink,
			Var5qi:              9,
			Arp:                 15,
		})
	}()

	logger.Logger.Info("Policy updated, waiting for session modification signalling")

	// --- Phase 3: Wait for N2 (NGAP) Modify Request to arrive at gNB ---
	_, err = gNodeB.WaitForMessage(
		ngapType.NGAPPDUPresentInitiatingMessage,
		ngapType.InitiatingMessagePresentPDUSessionResourceModifyRequest,
		15*time.Second,
	)
	if err != nil {
		return fmt.Errorf("gNB did not receive PDUSessionResourceModifyRequest: %v", err)
	}

	logger.Logger.Info("gNB received PDUSessionResourceModifyRequest")

	// --- Phase 4: Wait for N1 (NAS) PDU Session Modification Command ---
	modCmd, err := newUE.WaitForNASGSMMessage(nas.MsgTypePDUSessionModificationCommand, 15*time.Second)
	if err != nil {
		return fmt.Errorf("UE did not receive PDU Session Modification Command: %v", err)
	}

	logger.Logger.Info("UE received PDU Session Modification Command")

	// --- Phase 5: Validate the NAS message ---
	if cfg.ambrUplink != "" || cfg.ambrDownlink != "" {
		err = validate.PDUSessionModificationCommand(modCmd, &validate.ExpectedPDUSessionModificationCommand{
			AmbrUplinkKbps:   parseKbps(cfg.ambrUplink),
			AmbrDownlinkKbps: parseKbps(cfg.ambrDownlink),
		})
		if err != nil {
			return fmt.Errorf("PDU Session Modification Command AMBR validation failed: %v", err)
		}
	}

	// --- Phase 6: Validate updated QoS on gNB ---
	updatedSession := gNodeB.GetPDUSession(ranUENGAPID, int64(scenarios.DefaultPDUSessionID))
	if updatedSession == nil {
		return fmt.Errorf("PDU session not found on gNB after modification")
	}

	if cfg.var5qi != 0 || cfg.arp != 0 {
		err = validate.PDUSessionInformation(updatedSession, &validate.ExpectedPDUSessionInformation{
			FiveQi: int64(cfg.var5qi),
			PriArp: int64(cfg.arp),
			QFI:    1,
		})
		if err != nil {
			return fmt.Errorf("post-modification QoS validation failed: %v", err)
		}

		logger.Logger.Info("Session modification validated successfully",
			zap.Int64("New 5QI", updatedSession.FiveQi),
			zap.Int64("New ARP", updatedSession.PriArp),
		)
	}

	// --- Cleanup: Deregister UE ---
	pduSessionIDs := [16]bool{}
	pduSessionIDs[scenarios.DefaultPDUSessionID] = true

	err = procedure.UEContextRelease(&procedure.UEContextReleaseOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID:   ranUENGAPID,
		GnodeB:        gNodeB,
		UE:            newUE,
		PDUSessionIDs: pduSessionIDs,
	})
	if err != nil {
		return fmt.Errorf("UE context release failed: %v", err)
	}

	logger.Logger.Info("Session modification scenario completed successfully")

	return nil
}

// parseKbps converts a human-readable bitrate string (e.g. "300 Mbps") to
// kilobits per second, matching the kbps convention used by the validator.
func parseKbps(s string) uint64 {
	if s == "" {
		return 0
	}

	re := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(bps|Kbps|Mbps|Gbps|Tbps)`)

	matches := re.FindStringSubmatch(s)
	if len(matches) < 3 {
		return 0
	}

	value, _ := strconv.ParseFloat(matches[1], 64)
	unit := matches[2]

	switch unit {
	case "bps":
		return uint64(value / 1000)
	case "Kbps":
		return uint64(value)
	case "Mbps":
		return uint64(value * 1000)
	case "Gbps":
		return uint64(value * 1000000)
	case "Tbps":
		return uint64(value * 1000000000)
	default:
		return 0
	}
}
