// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const (
	s1SetupResponseIMSI = "001017271246615"
	homeENBIMSI         = "001017271246617"
)

const homeENBID = 0x5ee0000

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/s1_setup_response",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1SetupResponse,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(s1SetupResponseIMSI, "")},
			}
		},
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/s1_setup_home_enb",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1SetupHomeENB,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(homeENBIMSI, "")},
			}
		},
	})
}

func runS1SetupHomeENB(_ context.Context, env scenarios.Env, _ any) error {
	s1mme, err := s1mmeAddress(env.FirstCore())
	if err != nil {
		return err
	}

	g := env.FirstGNB()

	e, err := s1enb.Start(&s1enb.StartOpts{
		ENBID:            homeENBID,
		ENBIDKind:        s1ap.ENBIDHome,
		MCC:              scenarios.DefaultMCC,
		MNC:              scenarios.DefaultMNC,
		TAC:              scenarios.DefaultTAC,
		Name:             "Ella-Core-Tester-HeNB",
		CoreS1MMEAddress: s1mme,
		ENBAddress:       g.N2Address,
		ENBN3Address:     g.N3Address,
	})
	if err != nil {
		return fmt.Errorf("start home eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	resp, err := e.S1SetupResponse()
	if err != nil {
		return err
	}

	if _, err := validateS1SetupResponse(resp, scenarios.DefaultMCC, scenarios.DefaultMNC); err != nil {
		return err
	}

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	ue := e.NewUE(homeENBIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	res, err := e.Attach(ue, attachTimeout)
	if err != nil {
		return fmt.Errorf("attach over home eNB: %w", err)
	}

	return assertAttach(res, familyExpect(env, scenarios.DefaultDNN, scenarios.DefaultUEIPv4Pool))
}

func runS1SetupResponse(_ context.Context, env scenarios.Env, _ any) error {
	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	resp, err := e.S1SetupResponse()
	if err != nil {
		return err
	}

	gummei, err := validateS1SetupResponse(resp, scenarios.DefaultMCC, scenarios.DefaultMNC)
	if err != nil {
		return err
	}

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	ue := e.NewUE(s1SetupResponseIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	res, err := e.Attach(ue, attachTimeout)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if res.GUTI == nil || res.GUTI.GUTI == nil {
		return fmt.Errorf("attach: MME assigned no GUTI")
	}

	guti := res.GUTI.GUTI

	if guti.MMEGroupID != gummei.groupID {
		return fmt.Errorf("assigned GUTI names MME group %#04x, but S1 Setup Response served group %#04x", guti.MMEGroupID, gummei.groupID)
	}

	if guti.MMECode != gummei.code {
		return fmt.Errorf("assigned GUTI names MME code %#02x, but S1 Setup Response served code %#02x", guti.MMECode, gummei.code)
	}

	return nil
}

type servedMME struct {
	groupID uint16
	code    uint8
}

func validateS1SetupResponse(resp *s1ap.S1SetupResponse, expMCC, expMNC string) (servedMME, error) {
	var out servedMME

	if resp.MMEName == nil {
		return out, fmt.Errorf("MME Name missing")
	}

	if *resp.MMEName != "ella" {
		return out, fmt.Errorf("MME Name: got %q, want %q", *resp.MMEName, "ella")
	}

	if resp.RelativeMMECapacity == nil {
		return out, fmt.Errorf("relative MME Capacity missing")
	}

	if len(resp.ServedGUMMEIs) != 1 {
		return out, fmt.Errorf("served GUMMEIs: got %d items, want 1", len(resp.ServedGUMMEIs))
	}

	item := resp.ServedGUMMEIs[0]

	if len(item.ServedPLMNs) != 1 {
		return out, fmt.Errorf("served PLMNs: got %d entries, want 1", len(item.ServedPLMNs))
	}

	plmn, err := nas.ParsePLMN([3]byte(item.ServedPLMNs[0]))
	if err != nil {
		return out, fmt.Errorf("decode served PLMN: %w", err)
	}

	if plmn.MCC != expMCC || plmn.MNC != expMNC {
		return out, fmt.Errorf("served PLMN: got %s/%s, want %s/%s", plmn.MCC, plmn.MNC, expMCC, expMNC)
	}

	if len(item.ServedGroupIDs) != 1 {
		return out, fmt.Errorf("served Group IDs: got %d entries, want 1", len(item.ServedGroupIDs))
	}

	if len(item.ServedMMECs) != 1 {
		return out, fmt.Errorf("served MME Codes: got %d entries, want 1", len(item.ServedMMECs))
	}

	g := item.ServedGroupIDs[0]
	out.groupID = uint16(g[0])<<8 | uint16(g[1])
	out.code = uint8(item.ServedMMECs[0])

	return out, nil
}
