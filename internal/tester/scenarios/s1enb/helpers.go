// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const s1enbName = "Ella-Core-Tester-S1eNB"

// startENB starts a single S1AP eNB against the core in env, using the default
// PLMN/TAC and the configured RAN bind addresses.
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

	return s1enb.Start(&s1enb.StartOpts{
		ENBID:            uint32(enbID),
		MCC:              scenarios.DefaultMCC,
		MNC:              scenarios.DefaultMNC,
		TAC:              scenarios.DefaultTAC,
		Name:             s1enbName,
		CoreS1MMEAddress: s1mme,
		ENBAddress:       g.N2Address,
		ENBN3Address:     g.N3Address,
	})
}
