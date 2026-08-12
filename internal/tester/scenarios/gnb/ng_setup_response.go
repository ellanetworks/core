// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/ngap"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
)

const numRadios = 24

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "gnb/ngap/setup_response",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run:       runNGSetupResponse,
	})
}

func runNGSetupResponse(_ context.Context, env scenarios.Env, _ any) error {
	eg := errgroup.Group{}

	for i := range numRadios {
		idx := i

		eg.Go(func() error { return ngSetupOneRadio(env, idx) })
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("NGSetup: %w", err)
	}

	return nil
}

func ngSetupOneRadio(env scenarios.Env, index int) error {
	g := env.FirstGNB()

	node, err := gnb.Start(&gnb.StartOpts{
		GnbID:           fmt.Sprintf("%06x", index+1),
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             scenarios.DefaultDNN,
		TAC:             scenarios.DefaultTAC,
		Name:            fmt.Sprintf("Ella-Core-Tester-%d", index),
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    g.N2Address,
	})
	if err != nil {
		return fmt.Errorf("start gNB %d: %w", index, err)
	}

	defer node.Close()

	frame, err := node.WaitForMessage(
		gnb.Successful,
		ngap.ProcNGSetup,
		500*time.Millisecond,
	)
	if err != nil {
		return fmt.Errorf("wait NGSetupResponse: %w", err)
	}

	if err := testutil.ValidateSCTP(frame.Info, 60, 0); err != nil {
		return fmt.Errorf("SCTP validation: %w", err)
	}

	resp, err := ngap.ParseNGSetupResponse(frame.Value)
	if err != nil {
		return fmt.Errorf("parse NGSetupResponse: %w", err)
	}

	return validateNGSetupResponse(resp, scenarios.DefaultMCC, scenarios.DefaultMNC, scenarios.DefaultSST, scenarios.DefaultSD)
}

func validateNGSetupResponse(resp *ngap.NGSetupResponse, expMCC, expMNC string, expSST int, expSD string) error {
	if resp.AMFName != "amf" {
		return fmt.Errorf("AMF Name: got %q, want %q", resp.AMFName, "amf")
	}

	if len(resp.ServedGUAMIList) == 0 {
		return fmt.Errorf("GUAMI List missing")
	}

	if resp.RelativeAMFCapacity == nil {
		return fmt.Errorf("relative AMF Capacity missing")
	}

	if len(resp.PLMNSupportList) != 1 {
		return fmt.Errorf("PLMN Support List: got %d entries, want 1", len(resp.PLMNSupportList))
	}

	plmn := resp.PLMNSupportList[0]

	mcc, mnc := plmnIDToString(plmn.PLMNIdentity)
	if mcc != expMCC {
		return fmt.Errorf("MCC: got %q, want %q", mcc, expMCC)
	}

	if mnc != expMNC {
		return fmt.Errorf("MNC: got %q, want %q", mnc, expMNC)
	}

	if len(plmn.SliceSupportList) != 1 {
		return fmt.Errorf("slice support list: got %d entries, want 1", len(plmn.SliceSupportList))
	}

	sst, sd := snssaiToString(plmn.SliceSupportList[0].SNSSAI)
	if int(sst) != expSST {
		return fmt.Errorf("SST: got %d, want %d", sst, expSST)
	}

	if sd != expSD {
		return fmt.Errorf("SD: got %q, want %q", sd, expSD)
	}

	return nil
}

// plmnIDToString decodes the TBCD PLMN identity back to MCC/MNC digit strings
// (TS 23.003 §2.2).
func plmnIDToString(id ngap.PLMNIdentity) (string, string) {
	p, err := nas.ParsePLMN([3]byte(id))
	if err != nil {
		return "", ""
	}

	return p.MCC, p.MNC
}

func snssaiToString(snssai ngap.SNSSAI) (int32, string) {
	sd := ""
	if snssai.SD != nil {
		sd = hex.EncodeToString(snssai.SD[:])
	}

	return int32(snssai.SST), sd
}
