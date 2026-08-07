// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package interworking holds the scenarios that move one subscriber's session
// between 4G and 5G and assert it survives the move: same IP address, same
// anchored session, traffic flowing on the target access's tunnel
// (TS 23.502 §4.11.2).
//
// Both scenarios drive a single-registration UE, which is on one radio at a
// time: the source RAN is closed before the target starts, so they also share
// the one N3/S1-U address the standard one-core one-tester topology provides.
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

const (
	// interworkingIMSI is the one subscriber both accesses serve; the whole point
	// is that it is the same subscriber and the same session on either radio.
	interworkingIMSI = "001017271246700"

	// movedPDUSessionID is the PDU session identity the UE allocates. It is what
	// correlates the two accesses (TS 23.501 §5.17.2.1): on 4G it travels in the
	// PCO, on 5G it is the session identity itself.
	movedPDUSessionID = 1

	gnbTunIface   = "iwkgnbtun"
	enbTunIface   = "iwkenbtun"
	s1enbNodeName = "Ella-Core-Tester-IWK-S1eNB"

	// The move re-points the UPF's downlink; give it a moment to take before
	// probing, as the connectivity scenarios do.
	datapathSettle = 500 * time.Millisecond

	attachTimeout = 15 * time.Second

	s1MMEPort = "36412"
)

// fixture provisions the single subscriber both accesses share. Asserting usage
// for it makes the runner check that bytes actually moved, so a move that
// preserves the control-plane bookkeeping but not the user plane fails.
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

	if _, err := gNodeB.WaitForMessage(gnb.Successful, ngap.ProcNGSetup, 200*time.Millisecond); err != nil {
		gNodeB.Close()

		return nil, fmt.Errorf("await NG Setup Response: %w", err)
	}

	return gNodeB, nil
}

func startENB(env scenarios.Env) (*s1enb.ENB, error) {
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
		ENBN3Address:     g.N3Address,
		EnableDatapath:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("start eNB: %w", err)
	}

	return e, nil
}

// s1mmeAddress is the core's S1-MME endpoint, which shares the N2 address and
// differs only in port.
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
