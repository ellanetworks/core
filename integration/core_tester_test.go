package integration_test

import (
	"context"
	"os"
	"testing"

	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

// TestIntegrationTester brings the core-tester compose up once, bootstraps
// Ella Core, and runs one subtest per ported scenario. Each subtest
// provisions its fixture via the fixture package and invokes
// env.RunScenario.
//
// Subtests share the same Ella Core database; resources are provisioned
// idempotently and left in place for post-failure inspection. Compose is
// brought down by the t.Cleanup in setupTesterEnv.
//
// Scenarios NOT included here:
//   - ue/xn_handover_connectivity, gnb/ngap/xn_handover — multi-gNB, out of scope
//   - ue/paging/downlink_data — paging, out of scope
func TestIntegrationTester(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx := context.Background()
	env := setupTesterEnv(ctx, t)

	t.Logf("core-tester compose up in %s mode", DetectIPFamily())

	// Provision the baseline fixture once. All scenarios share
	// scenarios.Default* constants, so resources are identified by fixed
	// names; fixture helpers are idempotent so re-provisioning during
	// subtest setup is safe.
	f := fixture.New(t, ctx, env.Client)
	f.OperatorDefault()
	f.Profile(fixture.DefaultProfileSpec())
	f.Slice(fixture.DefaultSliceSpec())
	f.DataNetwork(fixture.DefaultDataNetworkSpec())
	f.Policy(fixture.DefaultPolicySpec())

	// Tier 5 — protocol-level, no fixture beyond the baseline.
	t.Run("Tier5", func(t *testing.T) {
		for _, name := range []string{
			"gnb/sctp",
			"gnb/ngap/setup_response",
			"gnb/ngap/setup_failure/unknown_plmn",
			"gnb/ngap/reset",
			"enb/ng_setup",
		} {
			name := name

			t.Run(subtestName(name), func(t *testing.T) {
				env.RunScenario(ctx, t, name)
			})
		}
	})

	// Tier 1 — single-subscriber scenarios. They reference
	// scenarios.DefaultIMSI so we provision that subscriber once.
	t.Run("Tier1", func(t *testing.T) {
		fx := fixture.New(t, ctx, env.Client)
		fx.Subscriber(fixture.DefaultSubscriberSpec())

		for _, name := range []string{
			"enb/registration_success",
			"enb/deregistration",
			"enb/connectivity",
			"ue/registration_success",
			"ue/registration_success_profile_a",
			"ue/registration_success_no_sd",
			"ue/registration_success_v4v6",
			"ue/registration/incorrect_guti",
			"ue/registration/periodic/signalling",
			"ue/authentication/wrong_key",
			"ue/registration_reject/unknown_ue",
			"ue/deregistration",
			"ue/context/release",
			"ue/connectivity",
			"ue/service_request/data",
		} {
			name := name

			t.Run(subtestName(name), func(t *testing.T) {
				// Scenarios that need additional subscribers beyond
				// the default (enb/connectivity, ue/connectivity) provision
				// their extras via per-test fixtures below.
				runScenarioWithExtras(ctx, t, env, name, fx)
			})
		}
	})

	t.Run("Tier2", func(t *testing.T) {
		// Multi-resource scenarios provision their extra slices / data
		// networks / policies / profiles by name. Fixture calls are
		// idempotent so we can safely include defaults too.
		fx := fixture.New(t, ctx, env.Client)
		fx.Subscriber(fixture.DefaultSubscriberSpec())

		for _, name := range []string{
			"ue/registration_success_multiple_slices",
			"ue/registration_success_multiple_data_networks",
			"ue/registration_success_multiple_policies",
			"ue/registration_success_multiple_policies_per_profile",
			"ue/connectivity_multi_pdu_session",
			"ue/connectivity_multiple_policies_per_profile",
		} {
			name := name

			t.Run(subtestName(name), func(t *testing.T) {
				provisionTier2(ctx, t, env, name)
				env.RunScenario(ctx, t, name)
			})
		}
	})

	t.Run("Tier3", func(t *testing.T) {
		// Volume scenarios. Each scenario advances IMSIs from a base in
		// its own range; fixture provisions the matching subscribers.
		for _, name := range []string{
			"ue/registration_success_50_sequential",
			"ue/registration_success_150_parallel",
			"enb/multi_ue_registration",
		} {
			name := name

			t.Run(subtestName(name), func(t *testing.T) {
				provisionTier3(ctx, t, env, name)
				env.RunScenario(ctx, t, name)
			})
		}
	})

	t.Run("Tier6", func(t *testing.T) {
		t.Run(subtestName("ue/registration_reject_invalid_home_network_public_key"), func(t *testing.T) {
			fx := fixture.New(t, ctx, env.Client)
			fx.Subscriber(fixture.DefaultSubscriberSpec())
			env.RunScenario(ctx, t, "ue/registration_reject_invalid_home_network_public_key")
		})
	})
}

// subtestName converts a scenario name like "ue/registration/incorrect_guti"
// into a Go-friendly subtest name.
func subtestName(s string) string {
	return s
}

// runScenarioWithExtras is a hook for Tier 1 scenarios that need extra
// fixtures (e.g., enb/connectivity and ue/connectivity provision 5 UEs)
// or scenario-specific args (profile_a requires a coordinated Home
// Network private key).
func runScenarioWithExtras(ctx context.Context, t *testing.T, env *testerEnv, name string, baseFx *fixture.F) {
	t.Helper()

	var extraArgs []string

	switch name {
	case "enb/connectivity", "ue/connectivity":
		const parallelCount = 5

		fx := fixture.New(t, ctx, env.Client)
		fx.SubscriberBatch("001017271246100", parallelCount, fixture.DefaultSubscriberSpec())

	case "ue/registration_success_profile_a":
		// Profile A SUCI protection requires a matching X25519 keypair
		// between Core and UE. Provision the private key on Core and pass
		// the same hex to the scenario so its UE derives the matching
		// public key.
		const profileAPrivKeyHex = "c53c22208b61860b06c62e5406a7b330c2b577aab3cd7cd2d3fa33ef6b3df3f6"

		fx := fixture.New(t, ctx, env.Client)
		fx.HomeNetworkKey(fixture.HomeNetworkKeySpec{
			KeyIdentifier: 4,
			Scheme:        "A",
			PrivateKey:    profileAPrivKeyHex,
		})

		extraArgs = append(extraArgs, "--home-network-private-key", profileAPrivKeyHex)
	}

	env.RunScenario(ctx, t, name, extraArgs...)

	_ = baseFx
}

// provisionTier2 provisions multi-resource fixtures keyed by scenario
// name. The resource names match what the corresponding scenario file
// references (see scenarios/ue/*.go).
func provisionTier2(ctx context.Context, t *testing.T, env *testerEnv, name string) {
	t.Helper()

	fx := fixture.New(t, ctx, env.Client)

	switch name {
	case "ue/registration_success_multiple_slices":
		// Two slices, two subscribers (see scenarios/ue).
		fx.Slice(fixture.SliceSpec{Name: "enterprise-slice", SST: 1, SD: "204060"})
		fx.Profile(fixture.ProfileSpec{
			Name:           "enterprise-profile",
			UeAmbrUplink:   scenarios.DefaultProfileUeAmbrUplink,
			UeAmbrDownlink: scenarios.DefaultProfileUeAmbrDownlink,
		})
		fx.DataNetwork(fixture.DataNetworkSpec{
			Name:   "enterprise",
			IPPool: "10.46.0.0/16",
			DNS:    "8.8.4.4",
			MTU:    1500,
		})
		fx.Policy(fixture.PolicySpec{
			Name:                "enterprise",
			ProfileName:         "enterprise-profile",
			SliceName:           "enterprise-slice",
			DataNetworkName:     "enterprise",
			SessionAmbrUplink:   "100 Mbps",
			SessionAmbrDownlink: "100 Mbps",
			Var5qi:              9,
			Arp:                 15,
		})
		fx.SubscriberBatch("001017271246200", 2, fixture.DefaultSubscriberSpec())

	case "ue/registration_success_multiple_data_networks":
		for _, dn := range []string{"internet2", "internet3", "internet4"} {
			fx.DataNetwork(fixture.DataNetworkSpec{
				Name:   dn,
				IPPool: tier2IPPool(dn),
				DNS:    "8.8.8.8",
				MTU:    1500,
			})
		}

		for _, p := range []string{"profile1", "profile2", "profile3", "profile4"} {
			fx.Profile(fixture.ProfileSpec{
				Name:           p,
				UeAmbrUplink:   scenarios.DefaultProfileUeAmbrUplink,
				UeAmbrDownlink: scenarios.DefaultProfileUeAmbrDownlink,
			})
		}

		for i, pair := range [][2]string{
			{"policy1", "profile1"},
			{"policy2", "profile2"},
			{"policy3", "profile3"},
			{"policy4", "profile4"},
		} {
			fx.Policy(fixture.PolicySpec{
				Name:                pair[0],
				ProfileName:         pair[1],
				SliceName:           scenarios.DefaultSliceName,
				DataNetworkName:     []string{scenarios.DefaultDNN, "internet2", "internet3", "internet4"}[i],
				SessionAmbrUplink:   scenarios.DefaultPolicySessionAmbrUplink,
				SessionAmbrDownlink: scenarios.DefaultPolicySessionAmbrDownlink,
				Var5qi:              9,
				Arp:                 15,
			})
		}

		fx.SubscriberBatch("001017271246300", 4, fixture.DefaultSubscriberSpec())

	case "ue/registration_success_multiple_policies":
		fx.Policy(fixture.PolicySpec{
			Name:                "policy2",
			ProfileName:         scenarios.DefaultProfileName,
			SliceName:           scenarios.DefaultSliceName,
			DataNetworkName:     scenarios.DefaultDNN,
			SessionAmbrUplink:   scenarios.DefaultPolicySessionAmbrUplink,
			SessionAmbrDownlink: scenarios.DefaultPolicySessionAmbrDownlink,
			Var5qi:              9,
			Arp:                 15,
		})
		fx.SubscriberBatch("001017271246400", 2, fixture.DefaultSubscriberSpec())

	case "ue/registration_success_multiple_policies_per_profile":
		fx.DataNetwork(fixture.DataNetworkSpec{
			Name:   "enterprise",
			IPPool: "10.46.0.0/16",
			DNS:    "8.8.4.4",
			MTU:    1500,
		})
		fx.Policy(fixture.PolicySpec{
			Name:                "enterprise",
			ProfileName:         scenarios.DefaultProfileName,
			SliceName:           scenarios.DefaultSliceName,
			DataNetworkName:     "enterprise",
			SessionAmbrUplink:   "30 Mbps",
			SessionAmbrDownlink: "60 Mbps",
			Var5qi:              7,
			Arp:                 15,
		})
		fx.SubscriberBatch("001017271246500", 2, fixture.DefaultSubscriberSpec())

	case "ue/connectivity_multi_pdu_session":
		fx.Slice(fixture.SliceSpec{Name: "enterprise-slice", SST: 1, SD: "204060"})
		fx.DataNetwork(fixture.DataNetworkSpec{
			Name:   "enterprise",
			IPPool: "10.46.0.0/16",
			DNS:    "8.8.4.4",
			MTU:    1500,
		})
		fx.Policy(fixture.PolicySpec{
			Name:                "enterprise",
			ProfileName:         scenarios.DefaultProfileName,
			SliceName:           "enterprise-slice",
			DataNetworkName:     "enterprise",
			SessionAmbrUplink:   "30 Mbps",
			SessionAmbrDownlink: "60 Mbps",
			Var5qi:              7,
			Arp:                 15,
		})
		fx.Subscriber(fixture.SubscriberSpec{
			IMSI:           "001017271246546",
			Key:            scenarios.DefaultKey,
			OPc:            scenarios.DefaultOPC,
			SequenceNumber: scenarios.DefaultSequenceNumber,
			ProfileName:    scenarios.DefaultProfileName,
		})

	case "ue/connectivity_multiple_policies_per_profile":
		fx.DataNetwork(fixture.DataNetworkSpec{
			Name:   "enterprise",
			IPPool: "10.46.0.0/16",
			DNS:    "8.8.4.4",
			MTU:    1500,
		})
		fx.Policy(fixture.PolicySpec{
			Name:                "enterprise",
			ProfileName:         scenarios.DefaultProfileName,
			SliceName:           scenarios.DefaultSliceName,
			DataNetworkName:     "enterprise",
			SessionAmbrUplink:   "30 Mbps",
			SessionAmbrDownlink: "60 Mbps",
			Var5qi:              7,
			Arp:                 15,
		})
		fx.Subscriber(fixture.SubscriberSpec{
			IMSI:           "001017271246546",
			Key:            scenarios.DefaultKey,
			OPc:            scenarios.DefaultOPC,
			SequenceNumber: scenarios.DefaultSequenceNumber,
			ProfileName:    scenarios.DefaultProfileName,
		})
		fx.Subscriber(fixture.SubscriberSpec{
			IMSI:           "001017271246547",
			Key:            scenarios.DefaultKey,
			OPc:            scenarios.DefaultOPC,
			SequenceNumber: scenarios.DefaultSequenceNumber,
			ProfileName:    scenarios.DefaultProfileName,
		})
	}
}

func tier2IPPool(dn string) string {
	switch dn {
	case "internet2":
		return "10.46.0.0/16"
	case "internet3":
		return "10.47.0.0/16"
	case "internet4":
		return "10.48.0.0/16"
	default:
		return scenarios.DefaultUEIPPool
	}
}

// provisionTier3 provisions batched subscribers for volume scenarios.
// Each scenario uses a distinct IMSI range.
func provisionTier3(ctx context.Context, t *testing.T, env *testerEnv, name string) {
	t.Helper()

	fx := fixture.New(t, ctx, env.Client)

	switch name {
	case "ue/registration_success_50_sequential":
		fx.SubscriberBatch("001017271246000", 50, fixture.DefaultSubscriberSpec())
	case "ue/registration_success_150_parallel":
		fx.SubscriberBatch("001017271246000", 150, fixture.DefaultSubscriberSpec())
	case "enb/multi_ue_registration":
		fx.SubscriberBatch("001017271246000", 10, fixture.DefaultSubscriberSpec())
	}
}
