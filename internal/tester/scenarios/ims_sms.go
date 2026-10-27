// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package scenarios

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/smsue"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/sms"
	"github.com/ellanetworks/ims/sip"
	"github.com/ellanetworks/ims/testue"
)

const (
	IPSMGWApplicationServer = "ip-sm-gw"
	IPSMGWServiceProfile    = "ip-sm-gw"
	IPSMGWServerName        = "sip:ipsmgw." + IMSHomeDomain
	imsSMSRegistrationDelay = 3 * time.Second
)

func IPSMGWFilterCriteria() []client.IMSInitialFilterCriterion {
	register, message, originating := "REGISTER", "MESSAGE", "ORIGINATING_REGISTERED"
	header, content := "^Content-Type$", `^application/vnd\.3gpp\.sms$`

	return []client.IMSInitialFilterCriterion{
		{
			Priority:                0,
			ApplicationServerName:   IPSMGWApplicationServer,
			Trigger:                 &client.IMSTrigger{ConditionsMatch: "all", Groups: []client.IMSConditionGroup{{Conditions: []client.IMSCondition{{Method: &register}}}}},
			DefaultHandling:         new("SESSION_CONTINUED"),
			IncludeRegisterRequest:  true,
			IncludeRegisterResponse: true,
		},
		{
			Priority:              1,
			ApplicationServerName: IPSMGWApplicationServer,
			Trigger: &client.IMSTrigger{ConditionsMatch: "all", Groups: []client.IMSConditionGroup{{Conditions: []client.IMSCondition{
				{Method: &message},
				{SessionCase: &originating},
				{SIPHeader: &client.IMSHeaderMatch{Header: header, Content: &content}},
			}}}},
			DefaultHandling: new("SESSION_CONTINUED"),
		},
	}
}

func ProvisionIPSMGWService(ctx context.Context, env Env, profile string) (func(), error) {
	cl, err := coreClient(env)
	if err != nil {
		return nil, err
	}

	if _, err := cl.CreateIMSApplicationServer(ctx, &client.CreateIMSApplicationServerOptions{
		Name:       IPSMGWApplicationServer,
		ServerName: IPSMGWServerName,
	}); err != nil {
		return nil, fmt.Errorf("create the IP-SM-GW application server: %w", err)
	}

	unassign, err := provisionServiceProfile(ctx, cl, profile, MMTelSet, IPSMGWServiceProfile, IPSMGWFilterCriteria())
	if err != nil {
		_ = cl.DeleteIMSApplicationServer(context.Background(), IPSMGWApplicationServer)

		return nil, err
	}

	return func() {
		unassign()

		_ = cl.DeleteIMSApplicationServer(context.Background(), IPSMGWApplicationServer)
	}, nil
}

func RequireIMSSMS(ctx context.Context, sender, receiver IMSEndpoint, transport sip.Transport, serviceCentre string) error {
	ctx, cancel := context.WithTimeout(ctx, imsTestUETimeout)
	defer cancel()

	a, err := newIMSUE(sender.Subscriber, sender.PCSCF, sender.Local, transport, 0)
	if err != nil {
		return err
	}

	defer a.close()

	b, err := newIMSUE(receiver.Subscriber, receiver.PCSCF, receiver.Local, transport, 0)
	if err != nil {
		return err
	}

	defer b.close()

	if err := a.register(ctx, sender.Subscriber, transport); err != nil {
		return err
	}

	if err := b.register(ctx, receiver.Subscriber, transport); err != nil {
		return err
	}

	time.Sleep(imsSMSRegistrationDelay)

	sc := strings.TrimPrefix(serviceCentre, "+")
	text := "sms over ip " + string(transport)

	submit, err := smsue.EncodeSubmit(1, receiver.Subscriber.MSISDN, text)
	if err != nil {
		return err
	}

	data, err := (&sms.RPData{Direction: nas.DirectionUplink, Reference: 7, Destination: sms.E164Address(sc), UserData: submit}).MarshalBinary()
	if err != nil {
		return err
	}

	res, err := a.SMS().Send(ctx, "tel:+"+sc, data)
	if err != nil {
		return fmt.Errorf("MESSAGE of the short message: %w", err)
	}

	if res.StatusCode != 202 {
		return fmt.Errorf("MESSAGE of the short message answered %d, want 202", res.StatusCode)
	}

	report, rp, err := receiveSMS(ctx, a.UE)
	if err != nil {
		return fmt.Errorf("submit report: %w", err)
	}

	if ack, ok := rp.(*sms.RPAck); !ok || ack.Reference != 7 {
		return fmt.Errorf("submit report %+v, want an RP-ACK of reference 7", rp)
	}

	if got := report.Header.Get("In-Reply-To"); got != res.Header.CallID() {
		return fmt.Errorf("submit report In-Reply-To %q, want %q", got, res.Header.CallID())
	}

	req, rp, err := receiveSMS(ctx, b.UE)
	if err != nil {
		return fmt.Errorf("short message to %s: %w", receiver.Subscriber.MSISDN, err)
	}

	deliver, ok := rp.(*sms.RPData)
	if !ok {
		return fmt.Errorf("short message %+v, want an RP-DATA", rp)
	}

	from, got, err := smsue.DecodeDeliver(deliver.UserData)
	if err != nil {
		return fmt.Errorf("SMS-DELIVER %x: %w", deliver.UserData, err)
	}

	if got != text || strings.TrimPrefix(from, "+") != strings.TrimPrefix(sender.Subscriber.MSISDN, "+") {
		return fmt.Errorf("SMS-DELIVER from %q of %q, want from %s of %q", from, got, sender.Subscriber.MSISDN, text)
	}

	ack, err := (&sms.RPAck{Direction: nas.DirectionUplink, Reference: deliver.Reference, UserData: []byte{0x00, 0x00}}).MarshalBinary()
	if err != nil {
		return err
	}

	res, err = b.SMS().Send(ctx, "tel:+"+sc, ack, sip.Field{Name: "In-Reply-To", Value: req.Header.CallID()})
	if err != nil {
		return fmt.Errorf("delivery report: %w", err)
	}

	if res.StatusCode != 202 {
		return fmt.Errorf("delivery report answered %d, want 202", res.StatusCode)
	}

	if err := a.deregister(ctx, transport); err != nil {
		return err
	}

	return b.deregister(ctx, transport)
}

func receiveSMS(ctx context.Context, u *testue.UE) (*sip.Request, sms.RPMessage, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case e := <-u.Events():
			if e.Request == nil || e.Request.Method != "MESSAGE" {
				continue
			}

			rp, err := sms.ParseRP(e.Request.Content().Body, nas.DirectionDownlink)
			if err != nil {
				return nil, nil, fmt.Errorf("RP message %x: %w", e.Request.Content().Body, err)
			}

			return e.Request, rp, nil
		}
	}
}
