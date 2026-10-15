// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const (
	haIMSMarker         = "PHASE1_DONE"
	haIMSNodeTimeout    = 60 * time.Second
	haIMSAttachAttempts = 3
)

var (
	haIMSRegistered = scenarios.IMSSubscriber("001017400000101", "+15557400101")
	haIMSFresh      = scenarios.IMSSubscriber("001017400000102", "+15557400102")
	haIMSMoving     = scenarios.IMSSubscriber("001017400000103", "+15557400103")
	haIMSCaller     = scenarios.IMSSubscriber("001017400000104", "+15557400104")
	haIMSCallee     = scenarios.IMSSubscriber("001017400000105", "+15557400105")
)

func init() {
	registerIMS("ha_ims/hss_node_loss", haIMSRegistered, runHSSNodeLoss, haIMSFresh)
	registerIMS("ha_ims/serving_node_loss", haIMSMoving, runServingNodeLoss)
	registerIMS("ha_ims/call_across_nodes", haIMSCaller, runCallAcrossNodes, haIMSCallee)
}

func haIMSCores(env scenarios.Env) ([]string, error) {
	if len(env.CoreN2Addresses) < 2 {
		return nil, fmt.Errorf("HA IMS scenarios need at least 2 cores, got %d", len(env.CoreN2Addresses))
	}

	return env.CoreN2Addresses, nil
}

func runHSSNodeLoss(ctx context.Context, env scenarios.Env) error {
	cores, err := haIMSCores(env)
	if err != nil {
		return err
	}

	e, err := startENBOn(env, cores[0], 0, true)
	if err != nil {
		return fmt.Errorf("start the eNB of the serving node: %w", err)
	}

	defer func() { _ = e.Close() }()

	watch, err := startENBOn(env, cores[1], 1, false)
	if err != nil {
		return fmt.Errorf("start the eNB of the lost node: %w", err)
	}

	defer func() { _ = watch.Close() }()

	registered, _, cleanup, err := attachIMSCallUEs(env, e, haIMSRegistered)
	if err != nil {
		return err
	}

	defer cleanup()

	release := func() {}

	defer func() { release() }()

	fresh := func(context.Context) (scenarios.IMSEndpoint, error) {
		var err error

		for range haIMSAttachAttempts {
			var (
				attached     []scenarios.IMSEndpoint
				cleanupFresh func()
			)

			attached, _, cleanupFresh, err = attachIMSCallUEsFrom(env, e, 1, haIMSFresh)
			if err == nil {
				release = cleanupFresh
				return attached[0], nil
			}
		}

		return scenarios.IMSEndpoint{}, fmt.Errorf("attach after the node loss: %w", err)
	}

	return scenarios.RequireIMSRegistrationsAcrossNodeLoss(ctx, registered[0], scenarios.IMSTransports[0],
		func(ctx context.Context) error { return awaitNodeLoss(ctx, watch) }, fresh)
}

func runServingNodeLoss(ctx context.Context, env scenarios.Env) error {
	cores, err := haIMSCores(env)
	if err != nil {
		return err
	}

	first, err := startENBOn(env, cores[0], 0, true)
	if err != nil {
		return fmt.Errorf("start the eNB of the serving node: %w", err)
	}

	before, _, cleanup, err := attachIMSCallUEs(env, first, haIMSMoving)
	if err != nil {
		_ = first.Close()
		return err
	}

	closeFirst := sync.OnceFunc(func() {
		cleanup()

		_ = first.Close()
	})

	defer closeFirst()

	var (
		second  *s1enb.ENB
		release = func() {}
	)

	defer func() {
		release()

		if second != nil {
			_ = second.Close()
		}
	}()

	move := func(ctx context.Context) (scenarios.IMSEndpoint, error) {
		lost := awaitNodeLoss(ctx, first)

		closeFirst()

		if lost != nil {
			return scenarios.IMSEndpoint{}, lost
		}

		second, err = startENBOn(env, cores[1], 1, true)
		if err != nil {
			return scenarios.IMSEndpoint{}, fmt.Errorf("start the eNB of the surviving node: %w", err)
		}

		after, _, cleanupAfter, err := attachIMSCallUEs(env, second, haIMSMoving)
		if err != nil {
			return scenarios.IMSEndpoint{}, fmt.Errorf("attach on the surviving node: %w", err)
		}

		release = cleanupAfter

		return after[0], nil
	}

	return scenarios.RequireIMSRegistrationAfterMove(ctx, before[0], scenarios.IMSTransports[0], move)
}

func runCallAcrossNodes(ctx context.Context, env scenarios.Env) error {
	cores, err := haIMSCores(env)
	if err != nil {
		return err
	}

	first, err := startENBOn(env, cores[0], 0, true)
	if err != nil {
		return fmt.Errorf("start the eNB of the serving node: %w", err)
	}

	endpoints, ues, cleanup, err := attachIMSCallUEs(env, first, haIMSCaller, haIMSCallee)
	if err != nil {
		_ = first.Close()
		return err
	}

	closeFirst := sync.OnceFunc(func() {
		cleanup()

		_ = first.Close()
	})

	defer closeFirst()

	if err := haIMSCall(ctx, first, endpoints, ues); err != nil {
		return fmt.Errorf("call on the serving node: %w", err)
	}

	lost := awaitNodeLoss(ctx, first)

	closeFirst()

	if lost != nil {
		return lost
	}

	second, err := startENBOn(env, cores[1], 1, true)
	if err != nil {
		return fmt.Errorf("start the eNB of the surviving node: %w", err)
	}

	defer func() { _ = second.Close() }()

	for range haIMSAttachAttempts {
		var cleanupAfter func()

		endpoints, ues, cleanupAfter, err = attachIMSCallUEs(env, second, haIMSCaller, haIMSCallee)
		if err == nil {
			defer cleanupAfter()
			break
		}
	}

	if err != nil {
		return fmt.Errorf("attach on the surviving node: %w", err)
	}

	if err := haIMSCall(ctx, second, endpoints, ues); err != nil {
		return fmt.Errorf("call after the node loss: %w", err)
	}

	return nil
}

func haIMSCall(ctx context.Context, e *s1enb.ENB, endpoints []scenarios.IMSEndpoint, ues []imsCallUE) error {
	bearers := expectVoiceBearers(e, ues)

	if err := scenarios.RequireIMSCallWithMedia(ctx, endpoints[0], endpoints[1], scenarios.IMSTransports[0], voiceMedia(e, ues, bearers)); err != nil {
		return errors.Join(err, bearers.wait())
	}

	return bearers.wait()
}

func awaitNodeLoss(ctx context.Context, e *s1enb.ENB) error {
	fmt.Println(haIMSMarker)

	_ = os.Stdout.Sync()

	ctx, cancel := context.WithTimeout(ctx, haIMSNodeTimeout)
	defer cancel()

	peer, err := e.WaitForActivePeerChange(ctx)
	if err != nil {
		return fmt.Errorf("the node was not lost: %w", err)
	}

	if peer != "" {
		return fmt.Errorf("the eNB moved to %s instead of losing its MME", peer)
	}

	return nil
}
