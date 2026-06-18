// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package s1enb holds core-tester scenarios that drive the 4G MME over S1-MME
// (S1AP), via the internal/tester/s1enb simulator. It is the EPS/MME counterpart
// to the gnb (5GC/NGAP) scenarios; it is distinct from the enb package, which is
// an ng-eNB (LTE radio attached to the 5G core) and tests the AMF.
package s1enb

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

// s1MMEPort is the standard S1-MME SCTP port (TS 36.412). The core listens for
// S1AP here, on the same address as the AMF's N2.
const s1MMEPort = "36412"

// s1enbIMSI is dedicated to this scenario so it does not race the ng-eNB and gNB
// scenarios that reuse the default IMSI.
const s1enbIMSI = "001017271246600"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "s1enb/registration_success",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runS1ENBRegistrationSuccess,
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(s1enbIMSI, "")},
			}
		},
	})
}

// runS1ENBRegistrationSuccess attaches a UE over S1AP and verifies the MME
// completes the EPS attach and assigns a GUTI (TS 24.301 §5.5.1.2).
func runS1ENBRegistrationSuccess(_ context.Context, env scenarios.Env, _ any) error {
	s1mme, err := s1mmeAddress(env.FirstCore())
	if err != nil {
		return err
	}

	g := env.FirstGNB()

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	enbID, err := strconv.ParseUint(scenarios.DefaultGNBID, 16, 32)
	if err != nil {
		return fmt.Errorf("parse eNB ID %q: %w", scenarios.DefaultGNBID, err)
	}

	e, err := s1enb.Start(&s1enb.StartOpts{
		ENBID:            uint32(enbID),
		MCC:              scenarios.DefaultMCC,
		MNC:              scenarios.DefaultMNC,
		TAC:              scenarios.DefaultTAC,
		Name:             "Ella-Core-Tester-S1eNB",
		CoreS1MMEAddress: s1mme,
		ENBAddress:       g.N2Address,
		ENBN3Address:     g.N3Address,
	})
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(s1enbIMSI, k, opc)

	res, err := e.Attach(ue, 15*time.Second)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if res.GUTI == nil {
		return fmt.Errorf("attach completed without a GUTI")
	}

	return nil
}

// s1mmeAddress derives the S1-MME endpoint from the core's N2 address: the same
// host, on the S1-MME port.
func s1mmeAddress(coreN2 string) (string, error) {
	host, _, err := net.SplitHostPort(coreN2)
	if err != nil {
		return "", fmt.Errorf("parse core N2 address %q: %w", coreN2, err)
	}

	return net.JoinHostPort(host, s1MMEPort), nil
}

func defaultKeyAndOPc() (k, opc [16]byte, err error) {
	kb, err := hex.DecodeString(scenarios.DefaultKey)
	if err != nil || len(kb) != 16 {
		return k, opc, fmt.Errorf("invalid default key")
	}

	ob, err := hex.DecodeString(scenarios.DefaultOPC)
	if err != nil || len(ob) != 16 {
		return k, opc, fmt.Errorf("invalid default OPc")
	}

	copy(k[:], kb)
	copy(opc[:], ob)

	return k, opc, nil
}
