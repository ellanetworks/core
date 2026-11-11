// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/probe"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const roamerAttachedMarker = "ROAMER_ATTACHED"

const (
	roamingModificationTimeout = 90 * time.Second
	roamingNetworkDetachWait   = 6 * time.Minute
)

type roamingParams struct {
	IMSI               string
	Key                string
	OPc                string
	MCC                string
	MNC                string
	SQN                string
	APN                string
	ENBIndex           int
	Ping               bool
	PingDestination    string
	Detach             bool
	ExpectReject       int
	ExpectResyncs      int
	ExpectAPN          string
	AwaitUEAMBRDLMbps  uint64
	AwaitUEAMBRULMbps  uint64
	AwaitNetworkDetach bool
	ExpectDetachCause  int
	Combined           bool
	DeferESMInfo       bool
	PDNType            string
	ExpectPDNType      string
	ExpectESMCause     int
	ReattachWithGUTI   bool
	SwitchOff          bool
	Hold               time.Duration
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "s1enb/roaming/attach",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &roamingParams{}
			fs.StringVar(&p.IMSI, "imsi", "", "IMSI of the roamer")
			fs.StringVar(&p.Key, "key", "", "K of the roamer (hex)")
			fs.StringVar(&p.OPc, "opc", "", "OPc of the roamer (hex)")
			fs.StringVar(&p.MCC, "mcc", scenarios.DefaultMCC, "MCC the eNB broadcasts (the visited PLMN)")
			fs.StringVar(&p.MNC, "mnc", scenarios.DefaultMNC, "MNC the eNB broadcasts (the visited PLMN)")
			fs.StringVar(&p.SQN, "sqn", "", "SQN the USIM holds (hex, 6 octets); when set the UE checks AUTN freshness and resynchronises")
			fs.StringVar(&p.APN, "apn", "", "APN requested in the attach (empty: the default APN)")
			fs.IntVar(&p.ENBIndex, "enb-index", 0, "offset added to the eNB ID, so several testers can attach through different eNBs")
			fs.BoolVar(&p.Ping, "ping", true, "ping through the default bearer after the attach")
			fs.StringVar(&p.PingDestination, "ping-destination", scenarios.DefaultPingDestination, "address pinged through the default bearer")
			fs.BoolVar(&p.Detach, "detach", true, "detach at the end; when false the UE stays registered and the eNB goes away")
			fs.IntVar(&p.ExpectReject, "expect-reject", 0, "expect an Attach Reject before authentication with this EMM cause")
			fs.IntVar(&p.ExpectResyncs, "expect-resyncs", -1, "number of authentication resynchronisations the attach must take (-1: any)")
			fs.StringVar(&p.ExpectAPN, "expect-apn", "", "APN the default bearer must be activated for")
			fs.Uint64Var(&p.AwaitUEAMBRDLMbps, "await-ue-ambr-dl", 0, "after the marker, await a UE Context Modification with this downlink UE-AMBR (Mbps)")
			fs.Uint64Var(&p.AwaitUEAMBRULMbps, "await-ue-ambr-ul", 0, "after the marker, await a UE Context Modification with this uplink UE-AMBR (Mbps)")
			fs.BoolVar(&p.AwaitNetworkDetach, "await-network-detach", false, "after the marker, await a network-initiated detach")
			fs.IntVar(&p.ExpectDetachCause, "expect-detach-cause", 0, "EMM cause the network-initiated detach must carry (0: any)")
			fs.BoolVar(&p.Combined, "combined", false, "request a combined EPS/IMSI attach and expect it answered EPS only with EMM cause #18")
			fs.BoolVar(&p.DeferESMInfo, "defer-esm-info", false, "set the ESM information transfer flag and send the APN in the ESM Information Response")
			fs.StringVar(&p.PDNType, "pdn-type", "ipv4", "PDN type requested in the attach: ipv4, ipv6 or ipv4v6")
			fs.StringVar(&p.ExpectPDNType, "expect-pdn-type", "", "PDN type the default bearer must be activated with (empty: any)")
			fs.IntVar(&p.ExpectESMCause, "expect-esm-cause", -1, "ESM cause the default bearer activation must carry (0: none, -1: any)")
			fs.BoolVar(&p.ReattachWithGUTI, "reattach-with-guti", false, "after the attach, re-attach on a new S1 connection with an integrity-protected GUTI Attach Request that skips authentication")
			fs.BoolVar(&p.SwitchOff, "switch-off", false, "detach with a switch-off Detach Request identified by the GUTI")
			fs.DurationVar(&p.Hold, "hold", 0, "stay attached this long after the marker before detaching")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runRoamingAttach(ctx, env, params.(*roamingParams))
		},
	})
}

func roamerKeys(p *roamingParams) (k, opc [16]byte, err error) {
	for _, f := range []struct {
		name string
		hex  string
		out  *[16]byte
	}{{"key", p.Key, &k}, {"opc", p.OPc, &opc}} {
		b, err := hex.DecodeString(f.hex)
		if err != nil || len(b) != 16 {
			return k, opc, fmt.Errorf("--%s must be 16 octets of hex", f.name)
		}

		copy(f.out[:], b)
	}

	return k, opc, nil
}

func runRoamingAttach(ctx context.Context, env scenarios.Env, p *roamingParams) error {
	if p.IMSI == "" {
		return fmt.Errorf("--imsi is required")
	}

	k, opc, err := roamerKeys(p)
	if err != nil {
		return err
	}

	s1mme, err := s1mmeAddress(env.FirstCore())
	if err != nil {
		return err
	}

	enbID, err := strconv.ParseUint(scenarios.DefaultGNBID, 16, 32)
	if err != nil {
		return fmt.Errorf("parse eNB ID %q: %w", scenarios.DefaultGNBID, err)
	}

	g := env.FirstGNB()

	e, err := s1enb.Start(&s1enb.StartOpts{
		ENBID:            uint32(enbID) + uint32(p.ENBIndex), // #nosec G115 -- a small tester index
		MCC:              p.MCC,
		MNC:              p.MNC,
		TAC:              scenarios.DefaultTAC,
		Name:             fmt.Sprintf("%s-roaming-%d", s1enbName, p.ENBIndex),
		CoreS1MMEAddress: s1mme,
		ENBAddress:       g.N2Address,
		ENBN3Address:     g.N3Address,
		EnableDatapath:   p.Ping,
	})
	if err != nil {
		return fmt.Errorf("start eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(p.IMSI, k, opc)

	if err := configureRoamer(ue, p); err != nil {
		return err
	}

	if p.ExpectReject != 0 {
		cause, err := e.AttachExpectReject(ue, attachTimeout)
		if err != nil {
			return err
		}

		if int(cause) != p.ExpectReject {
			return fmt.Errorf("attach reject cause = %d, want %d", cause, p.ExpectReject)
		}

		logger.Logger.Info("roamer rejected", zap.String("imsi", p.IMSI), zap.Uint8("emm_cause", cause))

		return nil
	}

	res, err := attachRoamer(ctx, e, ue, p)
	if err != nil {
		return err
	}

	fmt.Println(roamerAttachedMarker)

	_ = os.Stdout.Sync()

	if p.Hold > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(p.Hold):
		}
	}

	if p.AwaitUEAMBRDLMbps != 0 || p.AwaitUEAMBRULMbps != 0 {
		if err := awaitRoamerUEAMBR(e, res, p); err != nil {
			return err
		}
	}

	if p.AwaitNetworkDetach {
		req, err := e.AwaitNetworkDetach(ue, res.ENBUES1APID, roamingNetworkDetachWait)
		if err != nil {
			return err
		}

		if p.ExpectDetachCause != 0 && (req.Cause == nil || int(*req.Cause) != p.ExpectDetachCause) {
			return fmt.Errorf("network detach cause = %v, want %d", req.Cause, p.ExpectDetachCause)
		}

		logger.Logger.Info("roamer detached by the network", zap.String("imsi", p.IMSI))

		return nil
	}

	if !p.Detach {
		return nil
	}

	if p.SwitchOff {
		ue.DetachWithGUTI(res.GUTI)

		return e.SwitchOff(ue, res.MMEUES1APID, res.ENBUES1APID, releaseTimeout)
	}

	return e.Detach(ue, res.MMEUES1APID, res.ENBUES1APID, releaseTimeout)
}

var roamerPDNTypes = map[string]eps.PDNType{
	"ipv4":   eps.PDNTypeIPv4,
	"ipv6":   eps.PDNTypeIPv6,
	"ipv4v6": eps.PDNTypeIPv4v6,
}

func configureRoamer(ue *s1enb.UE, p *roamingParams) error {
	if p.APN != "" {
		ue.RequestAPN(p.APN)
	}

	if p.SQN != "" {
		b, err := hex.DecodeString(p.SQN)
		if err != nil || len(b) != 6 {
			return fmt.Errorf("--sqn must be 6 octets of hex")
		}

		ue.TrackSequenceNumber([6]byte(b))
	}

	pdnType, ok := roamerPDNTypes[p.PDNType]
	if !ok {
		return fmt.Errorf("--pdn-type %q is not ipv4, ipv6 or ipv4v6", p.PDNType)
	}

	if _, ok := roamerPDNTypes[p.ExpectPDNType]; p.ExpectPDNType != "" && !ok {
		return fmt.Errorf("--expect-pdn-type %q is not ipv4, ipv6 or ipv4v6", p.ExpectPDNType)
	}

	ue.RequestPDNType(uint8(pdnType))

	if p.Combined {
		ue.RequestCombinedAttach()
	}

	if p.DeferESMInfo {
		ue.DeferESMInformation()
	}

	return nil
}

func attachRoamer(ctx context.Context, e *s1enb.ENB, ue *s1enb.UE, p *roamingParams) (*s1enb.AttachResult, error) {
	res, err := e.Attach(ue, attachTimeout)
	if err != nil {
		return nil, fmt.Errorf("attach: %w", err)
	}

	logger.Logger.Info("roamer attached",
		zap.String("imsi", p.IMSI), zap.String("apn", res.APN), zap.String("ue_ipv4", res.UEIPv4),
		zap.Stringer("pdn_type", res.PDNType), zap.Any("esm_cause", res.ESMCause), zap.Any("emm_cause", res.EMMCause),
		zap.Uint64("ue_ambr_dl", res.UEAmbrDownlinkBps), zap.Uint64("ue_ambr_ul", res.UEAmbrUplinkBps),
		zap.Uint64("apn_ambr_dl", res.SessAmbrDownlinkBps), zap.Uint64("apn_ambr_ul", res.SessAmbrUplinkBps),
		zap.Int("resynchronisations", ue.Resynchronisations()))

	if p.ExpectResyncs >= 0 && ue.Resynchronisations() != p.ExpectResyncs {
		return nil, fmt.Errorf("the attach took %d resynchronisations, want %d", ue.Resynchronisations(), p.ExpectResyncs)
	}

	if err := checkRoamerAttach(res, p); err != nil {
		return nil, err
	}

	if p.Ping {
		if err := roamerPing(ctx, e, res, p.PingDestination); err != nil {
			return nil, err
		}
	}

	if !p.ReattachWithGUTI {
		return res, nil
	}

	again, err := e.ReattachWithGUTI(ue, res.GUTI, res.ENBUES1APID, attachTimeout)
	if err != nil {
		return nil, fmt.Errorf("integrity-protected GUTI re-attach: %w", err)
	}

	if again.GUTI == nil {
		again.GUTI = res.GUTI
	}

	logger.Logger.Info("roamer re-attached with its GUTI without authentication",
		zap.String("imsi", p.IMSI), zap.String("apn", again.APN), zap.String("ue_ipv4", again.UEIPv4))

	if err := checkRoamerAttach(again, p); err != nil {
		return nil, fmt.Errorf("re-attach: %w", err)
	}

	if p.Ping {
		if err := roamerPing(ctx, e, again, p.PingDestination); err != nil {
			return nil, fmt.Errorf("re-attach: %w", err)
		}
	}

	return again, nil
}

func checkRoamerAttach(res *s1enb.AttachResult, p *roamingParams) error {
	if p.ExpectAPN != "" && res.APN != p.ExpectAPN {
		return fmt.Errorf("default bearer APN = %q, want %q", res.APN, p.ExpectAPN)
	}

	if p.Combined {
		if res.AttachResultValue != eps.AttachResultEPS {
			return fmt.Errorf("combined attach result = %d, want EPS only (%d)", res.AttachResultValue, eps.AttachResultEPS)
		}

		if res.EMMCause == nil || *res.EMMCause != eps.EMMCauseCSDomainNotAvailable {
			return fmt.Errorf("combined attach EMM cause = %v, want %s", res.EMMCause, eps.EMMCauseCSDomainNotAvailable)
		}
	}

	if want, ok := roamerPDNTypes[p.ExpectPDNType]; ok && res.PDNType != want {
		return fmt.Errorf("default bearer PDN type = %s, want %s", res.PDNType, want)
	}

	switch {
	case p.ExpectESMCause == 0 && res.ESMCause != nil:
		return fmt.Errorf("default bearer activated with %s, want no ESM cause", *res.ESMCause)
	case p.ExpectESMCause > 0 && (res.ESMCause == nil || int(*res.ESMCause) != p.ExpectESMCause):
		return fmt.Errorf("default bearer ESM cause = %v, want #%d", res.ESMCause, p.ExpectESMCause)
	}

	return nil
}

func awaitRoamerUEAMBR(e *s1enb.ENB, res *s1enb.AttachResult, p *roamingParams) error {
	req, err := e.AcceptUEContextModification(res.ENBUES1APID, roamingModificationTimeout)
	if err != nil {
		return err
	}

	ambr := req.UEAggregateMaximumBitRate
	if ambr == nil {
		return fmt.Errorf("UE Context Modification Request without a UE-AMBR")
	}

	if uint64(ambr.DL) != p.AwaitUEAMBRDLMbps*mbpsToBps || uint64(ambr.UL) != p.AwaitUEAMBRULMbps*mbpsToBps {
		return fmt.Errorf("UE-AMBR = %d/%d bit/s (DL/UL), want %d/%d Mbps", ambr.DL, ambr.UL, p.AwaitUEAMBRDLMbps, p.AwaitUEAMBRULMbps)
	}

	logger.Logger.Info("roamer UE-AMBR modified", zap.Int64("dl", int64(ambr.DL)), zap.Int64("ul", int64(ambr.UL)))

	return nil
}

func roamerPing(ctx context.Context, e *s1enb.ENB, res *s1enb.AttachResult, destination string) error {
	if res.UEIPv4 == "" {
		return fmt.Errorf("attach assigned no IPv4 address")
	}

	const iface = "roamer0"

	if err := e.AddTunnel(&s1enb.TunnelOpts{
		UEIPv4:           res.UEIPv4 + "/16",
		UpfAddress:       res.UpfAddress,
		ULTEID:           res.ULTEID,
		DLTEID:           res.DLTEID,
		TunInterfaceName: iface,
	}); err != nil {
		return fmt.Errorf("add GTP tunnel: %w", err)
	}

	defer e.CloseTunnel(res.DLTEID)

	awaitDownlinkReady()

	if err := probe.Run(ctx, probe.ICMP, iface, destination, scenarios.DefaultProbePort, false); err != nil {
		return fmt.Errorf("ping %s via %s failed: %w", destination, iface, err)
	}

	logger.Logger.Info("roamer data path verified", zap.String("destination", destination))

	return nil
}
