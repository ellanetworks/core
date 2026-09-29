// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sms

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/sms"
)

const injectedFrom = "+15559990000"

func init() {
	registerPerRAT(ratScenario{suffix: "mo_mt", imsis: func(rat string) []scenarios.SubscriberSpec { return pair(rat, 100) }, run: runMOMT})
	registerPerRAT(ratScenario{suffix: "idle_recipient", imsis: func(rat string) []scenarios.SubscriberSpec { return pair(rat, 200) }, run: runIdleRecipient})
	registerPerRAT(ratScenario{suffix: "absent_then_alert", imsis: func(rat string) []scenarios.SubscriberSpec { return pair(rat, 300) }, run: runAbsentThenAlert})
	registerPerRAT(ratScenario{suffix: "injected", imsis: func(rat string) []scenarios.SubscriberSpec { return pair(rat, 400)[:1] }, run: runInjected})
	registerPerRAT(ratScenario{suffix: "memory_full", imsis: func(rat string) []scenarios.SubscriberSpec { return pair(rat, 500) }, run: runMemoryFull})
	registerPerRAT(ratScenario{suffix: "switched_off", imsis: func(rat string) []scenarios.SubscriberSpec { return pair(rat, 900)[:1] }, run: runSwitchedOff})
	registerPerRAT(ratScenario{suffix: "withdrawal", imsis: func(rat string) []scenarios.SubscriberSpec { return pair(rat, 700)[:1] }, run: runWithdrawal})
	registerPerRAT(ratScenario{suffix: "back_to_back", imsis: func(rat string) []scenarios.SubscriberSpec { return pair(rat, 1000)[:1] }, run: runBackToBack})

	lteNoMSISDN := subscriber("001017900000600", "")
	register("sms/4g_not_granted", []scenarios.SubscriberSpec{lteNoMSISDN}, func(ctx context.Context, env scenarios.Env, p *params) error {
		return runLTENotGranted(env, lteNoMSISDN.IMSI)
	})

	nrNoMSISDN := subscriber("001018900000600", "")
	register("sms/5g_not_granted", []scenarios.SubscriberSpec{nrNoMSISDN}, func(ctx context.Context, env scenarios.Env, p *params) error {
		return runNRNotGranted(env, nrNoMSISDN.IMSI)
	})

	crossRAT := []scenarios.SubscriberSpec{pair("4g", 800)[0], pair("5g", 800)[0]}
	register("sms/cross_rat", crossRAT, func(ctx context.Context, env scenarios.Env, p *params) error {
		return runCrossRAT(ctx, env, p, crossRAT)
	})
}

func runMOMT(ctx context.Context, _ scenarios.Env, p *params, net network, subs []scenarios.SubscriberSpec) error {
	phones, err := attachAll(net, subs)
	if err != nil {
		return err
	}

	defer closeAll(phones)

	a, b := phones[0], phones[1]

	if err := submit(ctx, p, a, b.Number(), "Hello from A"); err != nil {
		return err
	}

	if err := expectText(ctx, b, a.Number(), "Hello from A"); err != nil {
		return err
	}

	if err := submit(ctx, p, b, a.Number(), "Hi A, ça va? €"); err != nil {
		return err
	}

	return expectText(ctx, a, b.Number(), "Hi A, ça va? €")
}

func runIdleRecipient(ctx context.Context, _ scenarios.Env, p *params, net network, subs []scenarios.SubscriberSpec) error {
	phones, err := attachAll(net, subs)
	if err != nil {
		return err
	}

	defer closeAll(phones)

	a, b := phones[0], phones[1]

	if err := b.Idle(); err != nil {
		return fmt.Errorf("release the recipient: %w", err)
	}

	if err := submit(ctx, p, a, b.Number(), "Wake up"); err != nil {
		return err
	}

	if err := net.AwaitPage(pagingTimeout); err != nil {
		return fmt.Errorf("the idle recipient was not paged: %w", err)
	}

	if err := b.AnswerPage(); err != nil {
		return fmt.Errorf("answer the page: %w", err)
	}

	return expectText(ctx, b, a.Number(), "Wake up")
}

func runAbsentThenAlert(ctx context.Context, _ scenarios.Env, p *params, net network, subs []scenarios.SubscriberSpec) error {
	smsc, err := newSMSC(p)
	if err != nil {
		return err
	}

	phones, err := attachAll(net, subs)
	if err != nil {
		return err
	}

	defer closeAll(phones)

	a, b := phones[0], phones[1]

	if err := b.Idle(); err != nil {
		return fmt.Errorf("release the recipient: %w", err)
	}

	if err := submit(ctx, p, a, b.Number(), "While you were away"); err != nil {
		return err
	}

	if err := net.AwaitPage(pagingTimeout); err != nil {
		return fmt.Errorf("the idle recipient was not paged: %w", err)
	}

	fetch := func(ctx context.Context) (smscMessage, error) { return smsc.latestTo(ctx, b.Number()) }

	if err := smsc.waitFor(ctx, absentTimeout, "an absent-user attempt with no paging response", fetch, func(m smscMessage) bool {
		return m.absentWith(net.NodeType(), "no_paging_response_msc")
	}); err != nil {
		return err
	}

	if err := b.Connect(); err != nil {
		return fmt.Errorf("reconnect the recipient: %w", err)
	}

	if err := expectText(ctx, b, a.Number(), "While you were away"); err != nil {
		return fmt.Errorf("no redelivery after the alert: %w", err)
	}

	return smsc.waitFor(ctx, reportTimeout, "the message to be delivered", fetch, func(m smscMessage) bool { return m.deliveredVia(net.NodeType()) })
}

func runInjected(ctx context.Context, _ scenarios.Env, p *params, net network, subs []scenarios.SubscriberSpec) error {
	smsc, err := newSMSC(p)
	if err != nil {
		return err
	}

	phones, err := attachAll(net, subs)
	if err != nil {
		return err
	}

	defer closeAll(phones)

	b := phones[0]

	id, err := smsc.submit(ctx, b.Number(), "Injected by the SMSC")
	if err != nil {
		return err
	}

	if err := expectText(ctx, b, injectedFrom, "Injected by the SMSC"); err != nil {
		return err
	}

	fetch := func(ctx context.Context) (smscMessage, error) { return smsc.message(ctx, id) }

	return smsc.waitFor(ctx, reportTimeout, "the message to be delivered", fetch, func(m smscMessage) bool { return m.deliveredVia(net.NodeType()) })
}

func runMemoryFull(ctx context.Context, _ scenarios.Env, p *params, net network, subs []scenarios.SubscriberSpec) error {
	smsc, err := newSMSC(p)
	if err != nil {
		return err
	}

	phones, err := attachAll(net, subs)
	if err != nil {
		return err
	}

	defer closeAll(phones)

	a, b := phones[0], phones[1]

	b.Stack().SetMemoryFull(true)

	if err := submit(ctx, p, a, b.Number(), "Make some room"); err != nil {
		return err
	}

	rejectCtx, cancel := context.WithTimeout(ctx, deliveryTimeout)
	defer cancel()

	if err := b.Stack().AwaitRejection(rejectCtx); err != nil {
		return err
	}

	smmaCtx, cancelSMMA := context.WithTimeout(ctx, reportTimeout)
	defer cancelSMMA()

	report, err := b.Stack().MemoryAvailable(smmaCtx)
	if err != nil {
		return fmt.Errorf("memory available: %w", err)
	}

	if _, ok := report.(*sms.RPAck); !ok {
		return fmt.Errorf("memory available answered with %T", report)
	}

	if err := expectText(ctx, b, a.Number(), "Make some room"); err != nil {
		return err
	}

	fetch := func(ctx context.Context) (smscMessage, error) { return smsc.latestTo(ctx, b.Number()) }

	return smsc.waitFor(ctx, reportTimeout, "a memory-full attempt followed by the delivery", fetch, func(m smscMessage) bool {
		return m.failedWith("memory_capacity_exceeded", "memory_capacity_exceeded") && m.deliveredVia(net.NodeType())
	})
}

func runSwitchedOff(ctx context.Context, _ scenarios.Env, p *params, net network, subs []scenarios.SubscriberSpec) error {
	smsc, err := newSMSC(p)
	if err != nil {
		return err
	}

	phones, err := attachAll(net, subs)
	if err != nil {
		return err
	}

	if err := phones[0].SwitchOff(); err != nil {
		closeAll(phones)
		return fmt.Errorf("switch off: %w", err)
	}

	closeAll(phones)

	id, err := smsc.submit(ctx, subs[0].MSISDN, "Welcome back")
	if err != nil {
		return err
	}

	fetch := func(ctx context.Context) (smscMessage, error) { return smsc.message(ctx, id) }

	if err := smsc.waitFor(ctx, reportTimeout, "an IMSI-detached absent-user attempt while the phone is off", fetch, func(m smscMessage) bool {
		return m.absentWith(net.NodeType(), "imsi_detached")
	}); err != nil {
		return err
	}

	b, err := net.Attach(subs[0].IMSI, subs[0].MSISDN)
	if err != nil {
		return fmt.Errorf("attach after switching on: %w", err)
	}

	defer b.Close()

	if err := expectText(ctx, b, injectedFrom, "Welcome back"); err != nil {
		return fmt.Errorf("no delivery after switching on: %w", err)
	}

	return smsc.waitFor(ctx, reportTimeout, "the message to be delivered", fetch, func(m smscMessage) bool { return m.deliveredVia(net.NodeType()) })
}

func runBackToBack(ctx context.Context, _ scenarios.Env, p *params, net network, subs []scenarios.SubscriberSpec) error {
	smsc, err := newSMSC(p)
	if err != nil {
		return err
	}

	phones, err := attachAll(net, subs)
	if err != nil {
		return err
	}

	defer closeAll(phones)

	b := phones[0]
	want := map[string]bool{"First of two": true, "Second of two": true}
	ids := make([]int64, 0, len(want))

	for text := range want {
		id, err := smsc.submit(ctx, b.Number(), text)
		if err != nil {
			return err
		}

		ids = append(ids, id)
	}

	for range ids {
		rctx, cancel := context.WithTimeout(ctx, deliveryTimeout)
		m, err := b.Stack().Receive(rctx)

		cancel()

		if err != nil {
			return fmt.Errorf("%s received %d of %d messages: %w", b.Number(), len(ids)-len(want), len(ids), err)
		}

		if !want[m.Text] {
			return fmt.Errorf("%s received %q, want one of %v", b.Number(), m.Text, want)
		}

		delete(want, m.Text)
	}

	for _, id := range ids {
		fetch := func(ctx context.Context) (smscMessage, error) { return smsc.message(ctx, id) }

		if err := smsc.waitFor(ctx, reportTimeout, "both messages to be delivered", fetch, func(m smscMessage) bool { return m.deliveredVia(net.NodeType()) }); err != nil {
			return err
		}
	}

	return nil
}

func runLTENotGranted(env scenarios.Env, imsi string) error {
	l, err := startLTE(env)
	if err != nil {
		return err
	}

	defer l.Close()

	res, _, err := l.attach(imsi, true)
	if err != nil {
		return err
	}

	if res.SMSOnly || res.AttachResultValue != eps.AttachResultEPS {
		return fmt.Errorf("a subscriber without an MSISDN was attached for SMS (result %s, SMS only %t)", res.AttachResultValue, res.SMSOnly)
	}

	if res.EMMCause == nil || *res.EMMCause != eps.EMMCauseCSDomainNotAvailable {
		return fmt.Errorf("EMM cause = %v, want #18", res.EMMCause)
	}

	return nil
}

func runNRNotGranted(env scenarios.Env, imsi string) error {
	n, err := startNR(env)
	if err != nil {
		return err
	}

	defer n.Close()

	p, err := n.register(imsi, "", true)
	if err != nil {
		return err
	}

	if p.ue.SMSAllowed() {
		return fmt.Errorf("a subscriber without an MSISDN was granted SMS over NAS")
	}

	return nil
}

func withSMSDisabled(ctx context.Context, env scenarios.Env, during func() error) error {
	cl, err := client.New(&client.Config{BaseURL: env.APIAddress, APIToken: env.APIToken})
	if err != nil {
		return err
	}

	op, err := cl.GetOperator(ctx)
	if err != nil {
		return fmt.Errorf("get operator: %w", err)
	}

	original := op.SMS

	if err := cl.UpdateOperatorSMS(ctx, &client.UpdateOperatorSMSOptions{}); err != nil {
		return fmt.Errorf("disable SMS: %w", err)
	}

	restore := func() error {
		return cl.UpdateOperatorSMS(ctx, &client.UpdateOperatorSMSOptions{SMSCAddress: original.SMSCAddress, SMSCPort: original.SMSCPort, SMSNumber: original.SMSNumber})
	}

	if err := during(); err != nil {
		_ = restore()
		return err
	}

	if err := restore(); err != nil {
		return fmt.Errorf("re-enable SMS: %w", err)
	}

	return nil
}

func eventually(timeout time.Duration, what string, cond func() bool) error {
	deadline := time.Now().Add(timeout)

	for !cond() {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for %s", what)
		}

		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

func runWithdrawal(ctx context.Context, env scenarios.Env, cfg *params, net network, subs []scenarios.SubscriberSpec) error {
	smsc, err := newSMSC(cfg)
	if err != nil {
		return err
	}

	phones, err := attachAll(net, subs)
	if err != nil {
		return err
	}

	defer closeAll(phones)

	p, ok := phones[0].(withdrawalPhone)
	if !ok {
		return fmt.Errorf("%T cannot run the withdrawal scenario", phones[0])
	}

	if err := withSMSDisabled(ctx, env, func() error {
		if err := p.AwaitWithdrawal(deliveryTimeout); err != nil {
			return err
		}

		if err := expectNoReport(ctx, cfg, p); err != nil {
			return err
		}

		granted, err := p.UpdateRegistration()
		if err != nil {
			return fmt.Errorf("registration update while SMS is disabled: %w", err)
		}

		if granted {
			return fmt.Errorf("a registration update while SMS is disabled was granted SMS")
		}

		return nil
	}); err != nil {
		return err
	}

	if err := p.AwaitAvailable(deliveryTimeout); err != nil {
		return err
	}

	id, err := smsc.submit(ctx, p.Number(), "Before registering")
	if err != nil {
		return err
	}

	fetch := func(ctx context.Context) (smscMessage, error) { return smsc.message(ctx, id) }

	if err := smsc.waitFor(ctx, reportTimeout, "an IMSI-detached absent-user attempt before the UE registers for SMS", fetch, func(m smscMessage) bool {
		return m.absentWith(net.NodeType(), "imsi_detached")
	}); err != nil {
		return err
	}

	if err := p.RegainSMS(); err != nil {
		return fmt.Errorf("register for SMS again: %w", err)
	}

	if err := expectText(ctx, p, injectedFrom, "Before registering"); err != nil {
		return fmt.Errorf("no delivery after registering for SMS again: %w", err)
	}

	if err := smsc.waitFor(ctx, reportTimeout, "the waiting message to be delivered", fetch, func(m smscMessage) bool { return m.deliveredVia(net.NodeType()) }); err != nil {
		return err
	}

	if err := submit(ctx, cfg, p, p.Number(), "Back again"); err != nil {
		return err
	}

	return expectText(ctx, p, p.Number(), "Back again")
}

func expectNoReport(ctx context.Context, cfg *params, from phone) error {
	ctx, cancel := context.WithTimeout(ctx, noReportTimeout)
	defer cancel()

	report, err := from.Stack().Submit(ctx, cfg.ServiceCentre, from.Number(), "Not allowed")
	if err == nil {
		return fmt.Errorf("an SMS submitted after the withdrawal was answered with %T", report)
	}

	return nil
}

func runCrossRAT(ctx context.Context, env scenarios.Env, p *params, subs []scenarios.SubscriberSpec) error {
	l, err := startLTE(env)
	if err != nil {
		return err
	}

	defer l.Close()

	n, err := startNR(env)
	if err != nil {
		return err
	}

	defer n.Close()

	a, err := l.Attach(subs[0].IMSI, subs[0].MSISDN)
	if err != nil {
		return fmt.Errorf("4G attach: %w", err)
	}

	defer a.Close()

	b, err := n.Attach(subs[1].IMSI, subs[1].MSISDN)
	if err != nil {
		return fmt.Errorf("5G registration: %w", err)
	}

	if err := b.Idle(); err != nil {
		return err
	}

	if err := submit(ctx, p, a, b.Number(), "From LTE"); err != nil {
		return err
	}

	if err := n.AwaitPage(pagingTimeout); err != nil {
		return fmt.Errorf("the 5G recipient was not paged: %w", err)
	}

	if err := b.AnswerPage(); err != nil {
		return err
	}

	if err := expectText(ctx, b, a.Number(), "From LTE"); err != nil {
		return err
	}

	if err := submit(ctx, p, b, a.Number(), "From NR"); err != nil {
		return err
	}

	return expectText(ctx, a, b.Number(), "From NR")
}
