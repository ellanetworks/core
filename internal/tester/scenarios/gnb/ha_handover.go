// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/spf13/pflag"
)

const (
	HAHandoverMarker  = "HA_HANDOVER_DONE"
	haN2HandoverIMSI  = "001017271246660"
	haXnHandoverIMSI  = "001017271246661"
	haHandoverMaxHold = 3 * time.Minute
)

type handoverConnectivityOpts struct {
	imsi       string
	connectAll bool
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name:      "ha/n2_handover_5g",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return n2HandoverConnectivity(ctx, env, handoverConnectivityOpts{imsi: haN2HandoverIMSI, connectAll: true})
		},
		Fixture: haHandoverFixture(haN2HandoverIMSI),
	})

	scenarios.Register(scenarios.Scenario{
		Name:      "ha/xn_handover_5g",
		BindFlags: func(fs *pflag.FlagSet) any { return struct{}{} },
		Run: func(ctx context.Context, env scenarios.Env, _ any) error {
			return xnHandoverConnectivity(ctx, env, handoverConnectivityOpts{imsi: haXnHandoverIMSI, connectAll: true})
		},
		Fixture: haHandoverFixture(haXnHandoverIMSI),
	})
}

func haHandoverFixture(imsi string) func(scenarios.Env) scenarios.FixtureSpec {
	return func(_ scenarios.Env) scenarios.FixtureSpec {
		return scenarios.FixtureSpec{
			Subscribers:         []scenarios.SubscriberSpec{scenarios.DefaultSubscriberWith(imsi, "")},
			AssertUsageForIMSIs: []string{imsi},
		}
	}
}

func (o handoverConnectivityOpts) finish(ctx context.Context, env scenarios.Env, sourceAMF, targetAMF, ueIP string, targetRANUENGAPID int64) error {
	if !o.connectAll {
		return nil
	}

	if want := env.FirstCore(); sourceAMF != want {
		return fmt.Errorf("the UE registered through the AMF at %q, want the first core %q", sourceAMF, want)
	}

	if targetAMF != sourceAMF {
		return fmt.Errorf("the target gNB reached the UE's AMF at %q, want the AMF at %q that holds the UE context", targetAMF, sourceAMF)
	}

	fmt.Printf("%s imsi=%s ue_ip=%s ran_ue_ngap_id=%d amf=%s\n", HAHandoverMarker, o.imsi, ueIP, targetRANUENGAPID, sourceAMF)

	_ = os.Stdout.Sync()

	select {
	case <-ctx.Done():
	case <-time.After(haHandoverMaxHold):
	}

	return nil
}
