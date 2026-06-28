// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const sessionModIMSI = "001017271246651"

type sessionModParams struct {
	EllaAPIAddress string
	EllaAPIToken   string
}

// sessionModConfig is the policy change a scenario applies. A zero field means
// "unchanged" and is sent to the API as the default value.
type sessionModConfig struct {
	ambrUplinkMbps   uint64
	ambrDownlinkMbps uint64
	qci              uint8
	arp              uint8
}

func bindSessionModFlags(fs *pflag.FlagSet) any {
	p := &sessionModParams{}
	fs.StringVar(&p.EllaAPIAddress, "ella-api-address", "", "Ella Core API address")
	fs.StringVar(&p.EllaAPIToken, "ella-api-token", "", "Ella Core API token")

	return p
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/session-modification",
		BindFlags: bindSessionModFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runSessionModification(ctx, env, params.(*sessionModParams),
				sessionModConfig{ambrUplinkMbps: 200, ambrDownlinkMbps: 200, qci: 8, arp: 14})
		},
		Fixture: sessionModFixture,
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/session-modification-ambr-only",
		BindFlags: bindSessionModFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runSessionModification(ctx, env, params.(*sessionModParams),
				sessionModConfig{ambrUplinkMbps: 300, ambrDownlinkMbps: 400})
		},
		Fixture: sessionModFixture,
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/session-modification-qos-only",
		BindFlags: bindSessionModFlags,
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runSessionModification(ctx, env, params.(*sessionModParams),
				sessionModConfig{qci: 8, arp: 14})
		},
		Fixture: sessionModFixture,
	})
}

func sessionModFixture(_ scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(sessionModIMSI, "")},
	}
}

// A mid-session policy change applies in place (TS 24.301 §6.4.2): a QCI/ARP
// change via an S1AP E-RAB Modify Request (TS 36.413 §8.2.2) with the new EPS QoS
// piggybacked in the NAS-PDU; a Session-AMBR-only change via a standalone Modify
// EPS Bearer Context Request carrying the new APN-AMBR.
func runSessionModification(ctx context.Context, env scenarios.Env, p *sessionModParams, cfg sessionModConfig) error {
	if p.EllaAPIAddress == "" || p.EllaAPIToken == "" {
		return fmt.Errorf("--ella-api-address and --ella-api-token are required")
	}

	cl, err := client.New(&client.Config{BaseURL: p.EllaAPIAddress})
	if err != nil {
		return fmt.Errorf("failed to create Ella client: %w", err)
	}

	cl.SetToken(p.EllaAPIToken)

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(sessionModIMSI, k, opc)

	attach, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	// The bearer must come up with the default QoS before it is modified.
	if err := assertAttach(attach, expectedAttach{
		QCI: 9, ARP: 15,
		SessAmbrUplinkBps: 100 * mbpsToBps, SessAmbrDownlinkBps: 100 * mbpsToBps,
		RequireGUTI: true,
	}); err != nil {
		return fmt.Errorf("initial QoS: %w", err)
	}

	// Settle into EMM-REGISTERED so the change reconciles against an established
	// bearer and does not race the attach.
	time.Sleep(2 * time.Second)

	want := defaultSessionModPolicy()
	applySessionModConfig(&want, cfg)

	if err := cl.UpdatePolicy(ctx, scenarios.DefaultPolicyName, &want); err != nil {
		return fmt.Errorf("update policy: %w", err)
	}

	defer func() {
		restore := defaultSessionModPolicy()
		_ = cl.UpdatePolicy(context.Background(), scenarios.DefaultPolicyName, &restore)
	}()

	logger.GnbLogger.Info("policy updated; awaiting session modification",
		zap.Uint8("qci", cfg.qci), zap.Uint8("arp", cfg.arp),
		zap.Uint64("ambr-ul-mbps", cfg.ambrUplinkMbps), zap.Uint64("ambr-dl-mbps", cfg.ambrDownlinkMbps))

	qosChanged := cfg.qci != 0 || cfg.arp != 0
	ambrChanged := cfg.ambrUplinkMbps != 0 || cfg.ambrDownlinkMbps != 0

	if qosChanged {
		if err := assertQoSModification(e, ue, attach.ENBUES1APID, cfg, ambrChanged); err != nil {
			return err
		}
	} else {
		if err := assertAMBROnlyModification(e, ue, attach.ENBUES1APID, cfg); err != nil {
			return err
		}
	}

	logger.GnbLogger.Info("session modification applied in place")

	return e.Detach(ue, attach.MMEUES1APID, attach.ENBUES1APID, 10*time.Second)
}

// assertQoSModification validates the E-RAB Modify path: the eNB receives the new
// E-RAB QoS, and the piggybacked NAS carries the new EPS QoS (and APN-AMBR when the
// Session-AMBR also changed).
func assertQoSModification(e *s1enb.ENB, ue *s1enb.UE, enbUEID int64, cfg sessionModConfig, ambrChanged bool) error {
	erabReq, nasReq, err := e.ModifyBearerViaERABModify(ue, enbUEID, 20*time.Second)
	if err != nil {
		return fmt.Errorf("await E-RAB Modify: %w", err)
	}

	item := erabReq.ERABToBeModified[0]
	if uint8(item.QoS.QCI) != cfg.qci || item.QoS.ARP.PriorityLevel != cfg.arp {
		return fmt.Errorf("E-RAB QoS = QCI %d ARP %d, want %d/%d", item.QoS.QCI, item.QoS.ARP.PriorityLevel, cfg.qci, cfg.arp)
	}

	if len(nasReq.NewEPSQoS) == 0 || nasReq.NewEPSQoS[0] != cfg.qci {
		return fmt.Errorf("NAS New-EPS-QoS = % x, want QCI %d", nasReq.NewEPSQoS, cfg.qci)
	}

	if ambrChanged {
		if err := assertModifyAPNAMBR(nasReq, cfg); err != nil {
			return err
		}
	}

	return nil
}

func assertAMBROnlyModification(e *s1enb.ENB, ue *s1enb.UE, enbUEID int64, cfg sessionModConfig) error {
	nasReq, err := e.ModifyBearer(ue, enbUEID, 20*time.Second)
	if err != nil {
		return fmt.Errorf("await Modify EPS Bearer Context Request: %w", err)
	}

	return assertModifyAPNAMBR(nasReq, cfg)
}

func assertModifyAPNAMBR(nasReq *eps.ModifyEPSBearerContextRequest, cfg sessionModConfig) error {
	ambr, err := eps.ParseAPNAMBR(nasReq.APNAMBR)
	if err != nil {
		return fmt.Errorf("modification missing APN-AMBR: %w", err)
	}

	wantDL := cfg.ambrDownlinkMbps * mbpsToBps
	wantUL := cfg.ambrUplinkMbps * mbpsToBps

	if dl, ul := ambr.BitsPerSecond(); dl != wantDL || ul != wantUL {
		return fmt.Errorf("APN-AMBR = %d/%d bps, want %d/%d", dl, ul, wantDL, wantUL)
	}

	return nil
}

func defaultSessionModPolicy() client.UpdatePolicyOptions {
	return client.UpdatePolicyOptions{
		ProfileName:         scenarios.DefaultProfileName,
		SliceName:           scenarios.DefaultSliceName,
		DataNetworkName:     scenarios.DefaultDNN,
		SessionAmbrUplink:   scenarios.DefaultPolicySessionAmbrUplink,
		SessionAmbrDownlink: scenarios.DefaultPolicySessionAmbrDownlink,
		Var5qi:              9,
		Arp:                 15,
	}
}

func applySessionModConfig(opts *client.UpdatePolicyOptions, cfg sessionModConfig) {
	if cfg.ambrUplinkMbps != 0 {
		opts.SessionAmbrUplink = fmt.Sprintf("%d Mbps", cfg.ambrUplinkMbps)
	}

	if cfg.ambrDownlinkMbps != 0 {
		opts.SessionAmbrDownlink = fmt.Sprintf("%d Mbps", cfg.ambrDownlinkMbps)
	}

	if cfg.qci != 0 {
		opts.Var5qi = int32(cfg.qci)
	}

	if cfg.arp != 0 {
		opts.Arp = int32(cfg.arp)
	}
}
