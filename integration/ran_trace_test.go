// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/integration/suites"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

const (
	jaegerQueryAddress = "http://10.3.0.20:16686"
	ranTracePLMNHex    = "00f110"
)

func TestIntegrationRANTrace(t *testing.T) {
	suites.Require(t, suites.RANTrace)

	if DetectIPFamily() != IPv4Only {
		t.Skip("RAN trace integration runs in IPv4 mode only")
	}

	t.Setenv("ELLA_CORE_TELEMETRY", "true")

	ctx := context.Background()
	env := setupTesterEnv(ctx, t, "compose-ran-trace.yaml")

	baseline := fixture.New(t, ctx, env.Client)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	for _, scenario := range []string{"gnb/ran_trace", "s1enb/ran_trace"} {
		sc, ok := scenarios.Get(scenario)
		if !ok {
			t.Fatalf("scenario %q not registered", scenario)
		}

		fx := fixture.New(t, ctx, env.Client)
		fx.Apply(sc.Fixture(scenarios.Env{}))

		tr := registerScenarioTest(scenario)

		t.Run(scenario, func(t *testing.T) {
			defer finishScenarioTest(t, tr)

			env.RunScenario(ctx, t, scenario, tr)
		})
	}

	t.Run("spans share the RAN trace ID", func(t *testing.T) {
		for _, span := range []string{"ngap/receive", "s1ap/receive"} {
			if err := awaitRANTraceSpan(ctx, span, time.Minute); err != nil {
				t.Fatal(err)
			}
		}
	})

	if globalReporter.FailureCount() > 0 {
		writeFailureReports(t, "ran-trace")
	}
}

type jaegerTraces struct {
	Data []struct {
		TraceID string `json:"traceID"`
		Spans   []struct {
			OperationName string `json:"operationName"`
		} `json:"spans"`
	} `json:"data"`
}

func awaitRANTraceSpan(ctx context.Context, operation string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	var lastErr error

	for time.Now().Before(deadline) {
		found, err := ranTraceHasSpan(ctx, operation)
		if err == nil && found {
			return nil
		}

		lastErr = err

		time.Sleep(2 * time.Second)
	}

	if lastErr != nil {
		return fmt.Errorf("no RAN-derived trace with a %s span: %w", operation, lastErr)
	}

	return fmt.Errorf("no RAN-derived trace with a %s span within %s", operation, timeout)
}

func ranTraceHasSpan(ctx context.Context, operation string) (bool, error) {
	url := fmt.Sprintf("%s/api/traces?service=ella-core&operation=%s&lookback=1h&limit=500", jaegerQueryAddress, operation)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("jaeger query returned %s", resp.Status)
	}

	var traces jaegerTraces

	if err := json.NewDecoder(resp.Body).Decode(&traces); err != nil {
		return false, err
	}

	for _, tr := range traces.Data {
		id := fmt.Sprintf("%032s", tr.TraceID)
		if !strings.HasPrefix(id, strings.Repeat("0", 16)+ranTracePLMNHex) {
			continue
		}

		for _, span := range tr.Spans {
			if span.OperationName == operation {
				return true, nil
			}
		}
	}

	return false, nil
}
