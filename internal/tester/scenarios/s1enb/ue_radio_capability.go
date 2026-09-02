// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"bytes"
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

const ueRadioCapabilityIMSI = "001017271246624"

var ueRadioCapabilityBlob = []byte{0x04, 0x4c, 0x7a, 0x01, 0x08, 0x98, 0xbe, 0x01, 0xba, 0x04, 0x11, 0x04}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/ue_radio_capability",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBUERadioCapability,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(ueRadioCapabilityIMSI, "")},
			}
		},
	})
}

// The eNB reports the UE Radio Capability after the attach Initial Context Setup
// Request, so the MME cannot echo it there; it must store it and replay it in the
// next Initial Context Setup Request, which is what spares the eNB a second RRC
// capability enquiry (TS 36.413 §9.1.7.5, TS 23.401 §5.11.2).
func runS1ENBUERadioCapability(_ context.Context, env scenarios.Env, _ any) error {
	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	e.UERadioCapability = ueRadioCapabilityBlob

	ue := e.NewUE(ueRadioCapabilityIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	res, err := e.Attach(ue, attachTimeout)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if len(res.UERadioCapability) != 0 {
		return fmt.Errorf("attach Initial Context Setup Request carried a UE Radio Capability the eNB had not reported yet")
	}

	if res.GUTI == nil {
		return fmt.Errorf("attach completed without a GUTI, cannot service-request")
	}

	if err := e.ReleaseContext(res.MMEUES1APID, res.ENBUES1APID, s1enb.CauseUserInactivity, releaseTimeout); err != nil {
		return fmt.Errorf("release to ECM-IDLE: %w", err)
	}

	sr, err := e.ServiceRequest(ue, res.GUTI, releaseTimeout)
	if err != nil {
		return fmt.Errorf("service request: %w", err)
	}

	if len(sr.UERadioCapability) == 0 {
		return fmt.Errorf("initial Context Setup Request after the service request carried no UE Radio Capability; the MME did not retain what the eNB reported")
	}

	if !bytes.Equal(sr.UERadioCapability, ueRadioCapabilityBlob) {
		return fmt.Errorf("replayed UE Radio Capability = %x, want the %x the eNB reported", sr.UERadioCapability, ueRadioCapabilityBlob)
	}

	return nil
}
