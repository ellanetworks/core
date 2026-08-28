// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/ngap"
)

// Timeouts for the lifecycle procedures. s1enb scenarios spell theirs the same
// way, so a 4G and a 5G scenario read alike.
const (
	registrationTimeout = 15 * time.Second
	releaseTimeout      = 10 * time.Second
)

const (
	interworkingIMSI = "001017271246700"

	movedPDUSessionID = 1

	gnbTunIface   = "iwkgnbtun"
	enbTunIface   = "iwkenbtun"
	s1enbNodeName = "Ella-Core-Tester-IWK-S1eNB"

	datapathSettle = 500 * time.Millisecond

	attachTimeout = 15 * time.Second
	slaacTimeout  = 5 * time.Second

	ipv4TunPrefix = "/16"
	ipv6TunPrefix = "/64"

	s1MMEPort = "36412"
)

func fixture(_ scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers:         []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(interworkingIMSI, "")},
		AssertUsageForIMSIs: []string{interworkingIMSI},
	}
}

func startGNB(env scenarios.Env) (*gnb.GnodeB, error) {
	g := env.FirstGNB()

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:           scenarios.DefaultGNBID,
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             scenarios.DefaultDNN,
		TAC:             scenarios.DefaultTAC,
		Name:            "Ella-Core-Tester-IWK",
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    g.N2Address,
		GnbN3Address:    g.N3Address,
	})
	if err != nil {
		return nil, fmt.Errorf("start gNB: %w", err)
	}

	if _, err := gNodeB.WaitForMessage(gnb.Successful, ngap.ProcNGSetup, scenarios.NGSetupTimeout); err != nil {
		gNodeB.Close()

		return nil, fmt.Errorf("await NG Setup Response: %w", err)
	}

	return gNodeB, nil
}

func startENBOnSecondaryN3(env scenarios.Env) (*s1enb.ENB, error) {
	g := env.FirstGNB()
	if g.N3Secondary == "" {
		return nil, fmt.Errorf("this scenario needs a secondary N3 address so the eNB and the gNB can both hold one")
	}

	return startENBOn(env, g.N3Secondary)
}

func startENB(env scenarios.Env) (*s1enb.ENB, error) {
	return startENBOn(env, env.FirstGNB().N3Address)
}

func startENBOn(env scenarios.Env, n3Address string) (*s1enb.ENB, error) {
	s1mme, err := s1mmeAddress(env.FirstCore())
	if err != nil {
		return nil, err
	}

	enbID, err := strconv.ParseUint(scenarios.DefaultGNBID, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("parse eNB ID %q: %w", scenarios.DefaultGNBID, err)
	}

	g := env.FirstGNB()

	e, err := s1enb.Start(&s1enb.StartOpts{
		ENBID:            uint32(enbID),
		MCC:              scenarios.DefaultMCC,
		MNC:              scenarios.DefaultMNC,
		TAC:              scenarios.DefaultTAC,
		Name:             s1enbNodeName,
		CoreS1MMEAddress: s1mme,
		ENBAddress:       g.N2Address,
		ENBN3Address:     n3Address,
		EnableDatapath:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("start eNB: %w", err)
	}

	return e, nil
}

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
