// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sms

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/smsue"
	"github.com/ellanetworks/core/nas/sms"
	"github.com/spf13/pflag"
)

const (
	registrationTimeout = 15 * time.Second
	releaseTimeout      = 10 * time.Second
	reportTimeout       = 30 * time.Second
	deliveryTimeout     = 45 * time.Second
	absentTimeout       = 60 * time.Second
	pagingTimeout       = 20 * time.Second
)

type params struct {
	SMSCAPI       string
	ServiceCentre string
}

type phone interface {
	Stack() *smsue.Stack
	Number() string
	Connect() error
	Idle() error
	AnswerPage() error
	Close()
}

type network interface {
	Name() string
	Attach(imsi, msisdn string) (phone, error)
	AwaitPage(timeout time.Duration) error
	Close()
}

type ratScenario struct {
	suffix string
	imsis  func(rat string) []scenarios.SubscriberSpec
	run    func(ctx context.Context, env scenarios.Env, p *params, net network, subs []scenarios.SubscriberSpec) error
}

func bindFlags(fs *pflag.FlagSet) any {
	p := &params{}
	fs.StringVar(&p.SMSCAPI, "smsc-api-address", "", "SMSC API address (e.g. http://10.3.0.5:5010)")
	fs.StringVar(&p.ServiceCentre, "service-centre", "+15550000000", "the SMSC's service centre address")

	return p
}

func register(name string, subs []scenarios.SubscriberSpec, run func(ctx context.Context, env scenarios.Env, p *params) error) {
	scenarios.Register(scenarios.Scenario{
		Name:      name,
		BindFlags: bindFlags,
		Run: func(ctx context.Context, env scenarios.Env, raw any) error {
			p, ok := raw.(*params)
			if !ok {
				return fmt.Errorf("unexpected params %T", raw)
			}

			return run(ctx, env, p)
		},
		Fixture: func(scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{Subscribers: subs}
		},
	})
}

func registerPerRAT(s ratScenario) {
	for _, rat := range []string{"4g", "5g"} {
		subs := s.imsis(rat)

		register(fmt.Sprintf("sms/%s_%s", rat, s.suffix), subs, func(ctx context.Context, env scenarios.Env, p *params) error {
			net, err := startNetwork(env, rat)
			if err != nil {
				return err
			}

			defer net.Close()

			return s.run(ctx, env, p, net, subs)
		})
	}
}

func startNetwork(env scenarios.Env, rat string) (network, error) {
	if rat == "4g" {
		return startLTE(env)
	}

	return startNR(env)
}

func subscriber(imsi, msisdn string) scenarios.SubscriberSpec {
	s := scenarios.DefaultSubscriberWith(imsi, "")
	s.MSISDN = msisdn

	return s
}

func pair(rat string, base int) []scenarios.SubscriberSpec {
	prefix := "7"
	if rat == "5g" {
		prefix = "8"
	}

	return []scenarios.SubscriberSpec{
		subscriber(fmt.Sprintf("00101%s9%08d", prefix, base), fmt.Sprintf("+1555%s%06d", prefix, base)),
		subscriber(fmt.Sprintf("00101%s9%08d", prefix, base+1), fmt.Sprintf("+1555%s%06d", prefix, base+1)),
	}
}

func attachAll(net network, subs []scenarios.SubscriberSpec) ([]phone, error) {
	phones := make([]phone, 0, len(subs))

	for _, s := range subs {
		p, err := net.Attach(s.IMSI, s.MSISDN)
		if err != nil {
			for _, q := range phones {
				q.Close()
			}

			return nil, fmt.Errorf("%s attach %s: %w", net.Name(), s.IMSI, err)
		}

		phones = append(phones, p)
	}

	return phones, nil
}

func closeAll(phones []phone) {
	for _, p := range phones {
		p.Close()
	}
}

func submit(ctx context.Context, p *params, from phone, to string, text string) error {
	ctx, cancel := context.WithTimeout(ctx, reportTimeout)
	defer cancel()

	report, err := from.Stack().Submit(ctx, p.ServiceCentre, to, text)
	if err != nil {
		return fmt.Errorf("%s submitting to %s: %w", from.Number(), to, err)
	}

	if rpErr, ok := report.(*sms.RPError); ok {
		return fmt.Errorf("%s submitting to %s: RP-ERROR %s", from.Number(), to, rpErr.Cause)
	}

	return nil
}

func expectText(ctx context.Context, to phone, from, text string) error {
	ctx, cancel := context.WithTimeout(ctx, deliveryTimeout)
	defer cancel()

	m, err := to.Stack().Receive(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", to.Number(), err)
	}

	if m.Text != text {
		return fmt.Errorf("%s received %q, want %q", to.Number(), m.Text, text)
	}

	if from != "" && strings.TrimPrefix(m.From, "+") != strings.TrimPrefix(from, "+") {
		return fmt.Errorf("%s received a message from %s, want %s", to.Number(), m.From, from)
	}

	return nil
}
