// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const statusPoll = 200 * time.Millisecond

func coreClient(env scenarios.Env) (*client.Client, error) {
	cl, err := client.New(&client.Config{BaseURL: env.APIAddress})
	if err != nil {
		return nil, fmt.Errorf("create core client: %w", err)
	}

	cl.SetToken(env.APIToken)

	return cl, nil
}

func assertRegisteredOn(ctx context.Context, env scenarios.Env, want string) error {
	cl, err := coreClient(env)
	if err != nil {
		return err
	}

	deadline := time.Now().Add(sessionSettle)

	var last []string

	for {
		sub, err := cl.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: interworkingIMSI})
		if err == nil {
			last = sub.Status.RadioAccessTypes

			if sub.Status.Registered && slices.Equal(last, []string{want}) {
				return nil
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("the subscriber is not registered on %s alone (radio access types %v)", want, last)
		}

		time.Sleep(statusPoll)
	}
}
