// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sms

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/usim"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

// otaParams are the flags of the OTA card scenarios: the card's ADF RFM application and key set.
type otaParams struct {
	TAR       string
	KVN       uint8
	Algorithm string
	KIc, KID  string
	CCLength  int
	Entries   int
	Duration  time.Duration
}

func bindOTAFlags(fs *pflag.FlagSet) any {
	p := &otaParams{}
	fs.StringVar(&p.TAR, "ota-tar", "b00001", "TAR of the card's ADF RFM application, in hexadecimal")
	fs.Uint8Var(&p.KVN, "ota-kvn", 1, "version of the card's key set")
	fs.StringVar(&p.Algorithm, "ota-algorithm", "3des", "algorithm of the key set: 3des or aes")
	fs.StringVar(&p.KIc, "ota-kic", "", "ciphering key of the key set, in hexadecimal")
	fs.StringVar(&p.KID, "ota-kid", "", "integrity key of the key set, in hexadecimal")
	fs.IntVar(&p.CCLength, "ota-cc-length", 0, "length of the AES key set's checksums, 4 or 8; 0 is 8")
	fs.IntVar(&p.Entries, "ota-oplmn-entries", 0, "entries of EF.OPLMNwAcT; 0 is the fewest, 8")
	fs.DurationVar(&p.Duration, "duration", 0, "how long the UE stays registered; 0 is until interrupted")

	return p
}

func (p *otaParams) card(imsi string) (*usim.Card, error) {
	tar, err := hex.DecodeString(p.TAR)
	if err != nil || len(tar) != 3 {
		return nil, fmt.Errorf("--ota-tar %q is not 3 octets in hexadecimal", p.TAR)
	}

	kic, err := hex.DecodeString(p.KIc)
	if err != nil {
		return nil, fmt.Errorf("--ota-kic: %w", err)
	}

	kid, err := hex.DecodeString(p.KID)
	if err != nil {
		return nil, fmt.Errorf("--ota-kid: %w", err)
	}

	k := usim.Keyset{Version: p.KVN, KIc: kic, KID: kid, CCLength: p.CCLength}

	switch p.Algorithm {
	case "3des":
		k.Algorithm = usim.TripleDES
	case "aes":
		k.Algorithm = usim.AES
	default:
		return nil, fmt.Errorf("--ota-algorithm %q is not 3des or aes", p.Algorithm)
	}

	return usim.New(usim.Config{
		TAR: [3]byte(tar), Keysets: []usim.Keyset{k}, IMSI: imsi, OPLMNwAcTEntries: p.Entries,
		OnDownload: logDownload,
	})
}

// logDownload logs what the card did with a (U)SIM data download.
func logDownload(d usim.Download) {
	fields := []zap.Field{
		zap.String("tar", hex.EncodeToString(d.TAR[:])), zap.Uint64("counter", d.Counter), zap.Bool("por", d.PoR),
		zap.String("status", fmt.Sprintf("%02x", byte(d.Status))), zap.Int("executed", d.Executed),
		zap.String("status_word", fmt.Sprintf("%04x", d.StatusWord)),
	}

	if d.Err != nil {
		logger.Logger.Warn("sms/ota_card: (U)SIM data download discarded", append(fields, zap.Error(d.Err))...)
		return
	}

	logger.Logger.Info("sms/ota_card: (U)SIM data download", fields...)
}

func init() {
	for _, rat := range []string{"4g", "5g"} {
		subs := pair(rat, 1200)[:1]

		scenarios.Register(scenarios.Scenario{
			Name:      fmt.Sprintf("sms/%s_ota_card", rat),
			BindFlags: bindOTAFlags,
			Run: func(ctx context.Context, env scenarios.Env, raw any) error {
				p, ok := raw.(*otaParams)
				if !ok {
					return fmt.Errorf("unexpected params %T", raw)
				}

				return runOTACard(ctx, env, rat, p, subs[0])
			},
			Fixture: func(scenarios.Env) scenarios.FixtureSpec {
				return scenarios.FixtureSpec{Subscribers: subs}
			},
		})
	}
}

// runOTACard registers a UE of a USIM that runs SIM OTA, and keeps it registered, answering its pages, until ctx ends
// or the duration elapses, for an OTA platform to send it commands through the SMSC.
func runOTACard(ctx context.Context, env scenarios.Env, rat string, p *otaParams, sub scenarios.SubscriberSpec) error {
	card, err := p.card(sub.IMSI)
	if err != nil {
		return err
	}

	net, err := startNetwork(env, rat)
	if err != nil {
		return err
	}

	defer net.Close()

	ph, err := net.Attach(sub.IMSI, sub.MSISDN)
	if err != nil {
		return fmt.Errorf("%s attach %s: %w", net.Name(), sub.IMSI, err)
	}

	defer ph.Close()

	ph.Stack().SetUICC(card)

	if p.Duration > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, p.Duration)
		defer cancel()
	}

	logger.Logger.Info("sms/ota_card: registered with the card", zap.String("imsi", sub.IMSI), zap.String("msisdn", sub.MSISDN),
		zap.String("network", net.Name()))

	// An idle UE is paged for its MT short messages.
	for ctx.Err() == nil {
		if err := net.AwaitPage(time.Second); err != nil {
			continue
		}

		if err := ph.AnswerPage(); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("answer the page: %w", err)
		}
	}

	return nil
}
