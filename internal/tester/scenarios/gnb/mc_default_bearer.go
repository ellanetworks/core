// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/validate"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
)

const mcDefaultBearerIMSI = "001017271246902"

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/mc_default_bearer",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runMCDefaultBearer(ctx, env)
		},
		Fixture: func(env scenarios.Env) scenarios.FixtureSpec {
			return scenarios.MCFixture(env, mcDefaultBearerIMSI)
		},
	})
}

func runMCDefaultBearer(_ context.Context, env scenarios.Env) error {
	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	sub := subscriber{IMSI: mcDefaultBearerIMSI, Key: scenarios.DefaultKey, SequenceNumber: scenarios.DefaultSequenceNumber, OPc: scenarios.DefaultOPC, ProfileName: scenarios.MCProfileName}

	newUE, err := newSubscriberUE(gNodeB, sub, scenarios.MCDNN, env.PDUSessionType())
	if err != nil {
		return err
	}

	gNodeB.AddUE(scenarios.DefaultRANUENGAPID, newUE)

	registration, err := gNodeB.Register(newUE, scenarios.DefaultRANUENGAPID, scenarios.DefaultPDUSessionID, registrationTimeout)
	if err != nil {
		return fmt.Errorf("registration on %s failed: %v", scenarios.MCDNN, err)
	}

	err = validate.PDUSessionEstablishmentAccept(registration.Session.Accept, &validate.ExpectedPDUSessionEstablishmentAccept{
		PDUSessionID:               scenarios.DefaultPDUSessionID,
		PDUSessionType:             fgs.PDUSessionType(env.PDUSessionType()),
		UeIPSubnet:                 netip.MustParsePrefix(scenarios.MCUEIPv4Pool),
		Dnn:                        scenarios.MCDNN,
		Sst:                        scenarios.DefaultSST,
		Sd:                         scenarios.DefaultSD,
		MaximumBitRateUplinkMbps:   100,
		MaximumBitRateDownlinkMbps: 100,
		Qfi:                        1,
		FiveQI:                     scenarios.MCPolicy5QI,
	})
	if err != nil {
		return err
	}

	s := registration.Session
	if s.FiveQI != scenarios.MCPolicy5QI || s.ARP != scenarios.MCPolicyARP || s.PreemptCap != ngap.PreemptionMayTrigger || s.PreemptVuln != ngap.PreemptionNotPreemptable {
		return fmt.Errorf("default QoS flow 5QI %d, ARP %d, pre-emption capability %v, vulnerability %v; want 5QI %d, ARP %d, may trigger, not pre-emptable",
			s.FiveQI, s.ARP, s.PreemptCap, s.PreemptVuln, scenarios.MCPolicy5QI, scenarios.MCPolicyARP)
	}

	acc, err := fgs.ParsePDUSessionEstablishmentAccept(s.Accept)
	if err != nil {
		return fmt.Errorf("parse PDU Session Establishment Accept: %v", err)
	}

	var got []netip.Addr
	if acc.ExtendedPCO != nil {
		got = acc.ExtendedPCO.PCSCFAddresses()
	}

	if err := scenarios.CheckPCSCFAddresses(got, scenarios.ExpectedPCSCFAddresses(env)); err != nil {
		return err
	}

	return gNodeB.Deregister(newUE, scenarios.DefaultRANUENGAPID, releaseTimeout)
}
