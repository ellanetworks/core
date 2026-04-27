// Package multi holds tester scenarios that exercise multi-node cluster
// behaviour from the RAN side without involving failover.
package multi

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/scenarios/common"
	"github.com/free5gc/ngap/ngapType"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

// clusterTrafficParams binds the per-invocation flags for the
// multi/cluster_traffic scenario.
type clusterTrafficParams struct {
	UECount  int
	IMSIBase string
	GnbID    string
}

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "multi/cluster_traffic",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &clusterTrafficParams{}
			fs.IntVar(&p.UECount, "ue-count", 5, "number of UEs to drive on this gNB")
			fs.StringVar(&p.IMSIBase, "imsi-base", "", "IMSI of the first UE; subsequent UEs increment the last digit (required, 15 digits)")
			fs.StringVar(&p.GnbID, "gnb-id", scenarios.DefaultGNBID, "gNB-ID for this scenario")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runClusterTraffic(ctx, env, params.(*clusterTrafficParams))
		},
		// No Fixture: the integration test driver provisions the full
		// 15-subscriber pool up-front against the cluster's HA client so
		// every scenario invocation sees the subscribers it needs.
	})
}

// runClusterTraffic stands up one gNB on a single core peer and drives
// UECount UEs through registration + connectivity in parallel. Returns
// nil only if every UE pinged successfully; on failure, returns a
// concatenated error listing every failed UE so the test driver can
// surface all of them at once rather than just the first.
func runClusterTraffic(ctx context.Context, env scenarios.Env, p *clusterTrafficParams) error {
	if len(env.CoreN2Addresses) == 0 {
		return fmt.Errorf("multi/cluster_traffic requires --ella-core-n2-address")
	}

	if p.UECount < 1 {
		return fmt.Errorf("--ue-count must be >= 1, got %d", p.UECount)
	}

	if len(p.IMSIBase) != 15 {
		return fmt.Errorf("--imsi-base must be 15 digits, got %q", p.IMSIBase)
	}

	g := env.FirstGNB()
	if g.N2Address == "" || g.N3Address == "" {
		return fmt.Errorf("multi/cluster_traffic requires a --gnb declaration with n2 and n3")
	}

	gNodeB, err := gnb.Start(&gnb.StartOpts{
		GnbID:           p.GnbID,
		MCC:             scenarios.DefaultMCC,
		MNC:             scenarios.DefaultMNC,
		SST:             scenarios.DefaultSST,
		SD:              scenarios.DefaultSD,
		DNN:             scenarios.DefaultDNN,
		TAC:             scenarios.DefaultTAC,
		Name:            fmt.Sprintf("Ella-Core-Tester-Multi-%s", p.GnbID),
		CoreN2Addresses: env.CoreN2Addresses,
		GnbN2Address:    g.N2Address,
		GnbN3Address:    g.N3Address,
	})
	if err != nil {
		return fmt.Errorf("start gNB: %w", err)
	}

	defer gNodeB.Close()

	if _, err := gNodeB.WaitForMessage(
		ngapType.NGAPPDUPresentSuccessfulOutcome,
		ngapType.SuccessfulOutcomePresentNGSetupResponse,
		5*time.Second,
	); err != nil {
		return fmt.Errorf("NG Setup Response: %w", err)
	}

	logger.Logger.Info("multi/cluster_traffic: gNB up, driving UEs",
		zap.String("gnbId", p.GnbID),
		zap.String("peer", gNodeB.ActivePeerAddress()),
		zap.Int("ueCount", p.UECount),
	)

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		failures []string
	)

	for i := 0; i < p.UECount; i++ {
		i := i

		imsi, err := offsetIMSI(p.IMSIBase, i)
		if err != nil {
			return fmt.Errorf("compute IMSI for UE %d: %w", i, err)
		}

		// RAN-UE-NGAP-IDs and tunnel names must be unique per UE on this
		// gNB; collisions would either alias UE state in the gNB or fail
		// when AddTunnel sees a duplicate interface name.
		ranUENGAPID := int64(scenarios.DefaultRANUENGAPID) + int64(i)
		tunName := fmt.Sprintf("multi%s%d", p.GnbID, i)

		wg.Add(1)

		go func() {
			defer wg.Done()

			err := common.RegisterAndPing(ctx, &common.RegisterAndPingOpts{
				GNB:              gNodeB,
				RANUENGAPID:      ranUENGAPID,
				PDUSessionID:     scenarios.DefaultPDUSessionID,
				IMSI:             imsi,
				TunInterfaceName: tunName,
			})
			if err != nil {
				mu.Lock()

				failures = append(failures, fmt.Sprintf("UE %d (IMSI %s): %v", i, imsi, err))
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if len(failures) > 0 {
		return fmt.Errorf("%d/%d UEs failed: %s", len(failures), p.UECount, strings.Join(failures, "; "))
	}

	logger.Logger.Info("multi/cluster_traffic: all UEs pinged successfully",
		zap.String("gnbId", p.GnbID),
		zap.Int("ueCount", p.UECount),
	)

	return nil
}

// offsetIMSI returns base interpreted as a 15-digit decimal integer
// incremented by offset, zero-padded back to 15 digits. Returns an
// error if the result would overflow 15 digits.
func offsetIMSI(base string, offset int) (string, error) {
	n, err := strconv.ParseUint(base, 10, 64)
	if err != nil {
		return "", fmt.Errorf("parse base IMSI %q: %w", base, err)
	}

	out := strconv.FormatUint(n+uint64(offset), 10)

	if len(out) > 15 {
		return "", fmt.Errorf("base %q + offset %d overflows 15 digits", base, offset)
	}

	return strings.Repeat("0", 15-len(out)) + out, nil
}
