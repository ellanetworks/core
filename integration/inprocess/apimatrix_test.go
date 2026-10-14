// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package inprocess_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/integration/apimatrix"
	"github.com/ellanetworks/core/pkg/runtime/runtimetest"
)

func TestAPIMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("boots one core per resource")
	}

	for name, run := range apimatrix.Resources {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			core := runtimetest.Start(t, runtimetest.Options{})

			run(context.Background(), t, &apimatrix.Env{
				Client:        core.Client,
				APIAddress:    core.APIAddress,
				N6GatewayIPv4: "10.6.0.3",
				N6GatewayIPv6: "fd00:6::3",
			})
		})
	}
}
