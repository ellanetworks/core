// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package apimatrix

import (
	"context"
	"testing"
)

// runOperatorMatrix exercises the operator resource. Its fields are updated
// through separate endpoints, so each is covered as a subtest.
func runOperatorMatrix(ctx context.Context, t *testing.T, env *Env) {
	c := env.Client

	t.Run("code", func(t *testing.T) { runOperatorCodeMatrix(ctx, t, c) })
	t.Run("id", func(t *testing.T) { runOperatorIDMatrix(ctx, t, c) })
	t.Run("tracking", func(t *testing.T) { runOperatorTrackingMatrix(ctx, t, c) })
	t.Run("nas_security", func(t *testing.T) { runOperatorNASSecurityMatrix(ctx, t, c) })
	t.Run("spn", func(t *testing.T) { runOperatorSPNMatrix(ctx, t, c) })
	t.Run("sms", func(t *testing.T) { runOperatorSMSMatrix(ctx, t, c) })
}
