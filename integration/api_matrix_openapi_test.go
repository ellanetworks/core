// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runOpenAPISpecMatrix exercises the OpenAPI spec endpoint, which serves the
// specification as raw YAML.
func runOpenAPISpecMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	spec, err := c.GetOpenAPISpec(ctx)
	if err != nil {
		t.Fatalf("get openapi spec: %v", err)
	}

	if len(spec) == 0 {
		t.Fatalf("openapi spec: got empty body")
	}

	if !bytes.Contains(spec, []byte("openapi")) {
		t.Fatalf("openapi spec: missing \"openapi\" key")
	}
}
