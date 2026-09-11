// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"encoding/binary"
	"fmt"
	"strconv"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/scenarios/common"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"github.com/spf13/pflag"
)

const pagingIMSI = "001017271246621"

type pagingParams struct {
	EllaAPIAddress string
	EllaAPIToken   string
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "s1enb/paging",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &pagingParams{}
			fs.StringVar(&p.EllaAPIAddress, "ella-api-address", "", "Ella Core API address")
			fs.StringVar(&p.EllaAPIToken, "ella-api-token", "", "Ella Core API token")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runS1ENBPaging(ctx, env, params.(*pagingParams))
		},
		Fixture: func(_ scenarios.Env) scenarios.FixtureSpec {
			return scenarios.FixtureSpec{
				Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(pagingIMSI, "")},
			}
		},
	})
}

func runS1ENBPaging(ctx context.Context, env scenarios.Env, p *pagingParams) error {
	if p.EllaAPIAddress == "" || p.EllaAPIToken == "" {
		return fmt.Errorf("--ella-api-address and --ella-api-token are required")
	}

	cl, err := client.New(&client.Config{BaseURL: p.EllaAPIAddress})
	if err != nil {
		return fmt.Errorf("create Ella client: %w", err)
	}

	cl.SetToken(p.EllaAPIToken)

	k, opc, err := defaultKeyAndOPc()
	if err != nil {
		return err
	}

	e, err := startENB(env)
	if err != nil {
		return fmt.Errorf("start S1 eNB: %w", err)
	}

	defer func() { _ = e.Close() }()

	ue := e.NewUE(pagingIMSI, k, opc)
	ue.RequestPDNType(env.PDUSessionType())

	res, err := e.Attach(ue, attachTimeout)
	if err != nil {
		return fmt.Errorf("attach: %w", err)
	}

	if res.GUTI == nil {
		return fmt.Errorf("attach completed without a GUTI")
	}

	if err := e.ReleaseContext(res.MMEUES1APID, res.ENBUES1APID, s1enb.CauseUserInactivity, releaseTimeout); err != nil {
		return fmt.Errorf("release to ECM-IDLE: %w", err)
	}

	errCh := make(chan error, 1)

	go func() {
		_, err := common.GetLocation(ctx, cl, "imsi-"+pagingIMSI, "ecid")
		errCh <- err
	}()

	paging, err := e.WaitForPaging(15 * time.Second)
	if err != nil {
		return fmt.Errorf("await Paging: %w", err)
	}

	if err := assertPaging(paging, res.GUTI.GUTI); err != nil {
		return err
	}

	if _, err := e.ServiceRequestAnsweringPage(ue, res.GUTI, releaseTimeout, nil); err != nil {
		return fmt.Errorf("service request answering the page: %w", err)
	}

	if err := <-errCh; err != nil {
		return fmt.Errorf("location request that triggered the page: %w", err)
	}

	return nil
}

func assertPaging(p *s1ap.Paging, guti *eps.GUTI) error {
	if p.STMSI == nil {
		return fmt.Errorf("paging carried no S-TMSI, so the UE cannot recognise itself")
	}

	wantMTMSI := binary.BigEndian.Uint32(guti.TMSI[:])
	if uint32(p.STMSI.MTMSI) != wantMTMSI {
		return fmt.Errorf("paging M-TMSI = %#08x, want the assigned %#08x", uint32(p.STMSI.MTMSI), wantMTMSI)
	}

	if uint8(p.STMSI.MMEC) != guti.MMECode {
		return fmt.Errorf("paging MME code = %#02x, want the assigned %#02x", uint8(p.STMSI.MMEC), guti.MMECode)
	}

	if p.UEIdentityIndexValue == nil {
		return fmt.Errorf("paging carried no UE Identity Index Value, so the eNB cannot compute the paging occasion")
	}

	imsi, err := strconv.ParseUint(pagingIMSI, 10, 64)
	if err != nil {
		return fmt.Errorf("parse IMSI: %w", err)
	}

	if want := uint16(imsi % 1024); *p.UEIdentityIndexValue != want {
		return fmt.Errorf("UE Identity Index Value = %d, want IMSI mod 1024 = %d", *p.UEIdentityIndexValue, want)
	}

	if p.CNDomain == nil || *p.CNDomain != s1ap.CNDomainPS {
		return fmt.Errorf("paging CN Domain = %v, want ps", p.CNDomain)
	}

	if len(p.TAIList) != 1 {
		return fmt.Errorf("paging TAI List: got %d entries, want 1", len(p.TAIList))
	}

	tac, err := strconv.ParseUint(scenarios.DefaultTAC, 16, 16)
	if err != nil {
		return fmt.Errorf("parse TAC: %w", err)
	}

	tai := p.TAIList[0]
	if uint16(tai.TAC) != uint16(tac) {
		return fmt.Errorf("paging TAI TAC = %d, want %d", uint16(tai.TAC), uint16(tac))
	}

	plmn, err := nas.ParsePLMN([3]byte(tai.PLMNIdentity))
	if err != nil {
		return fmt.Errorf("decode paging TAI PLMN: %w", err)
	}

	if plmn.MCC != scenarios.DefaultMCC || plmn.MNC != scenarios.DefaultMNC {
		return fmt.Errorf("paging TAI PLMN = %s/%s, want %s/%s", plmn.MCC, plmn.MNC, scenarios.DefaultMCC, scenarios.DefaultMNC)
	}

	return nil
}
